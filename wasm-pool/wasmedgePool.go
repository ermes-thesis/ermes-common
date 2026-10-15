package wasm

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"

	"github.com/ermes-thesis/ermes-common/caches"
	"github.com/ermes-thesis/ermes-common/logger"
	"github.com/ermes-thesis/ermes-common/model"
	"github.com/second-state/WasmEdge-go/wasmedge"
)

type WasmedgeExecutorPool struct {
	*WASPBase
	wasi    *wasmedge.Module
	hostFns *wasmedge.Module
	workers chan *wasmedgeExecutor
	cache   *caches.BaseCache[*RefCountedItem[*wasmedge.AST]]
}

func NewWasmedgeExecutorPool(cfg WASPConfig, log logger.Logger) *WasmedgeExecutorPool {
	if err := os.MkdirAll("./sandbox", os.ModePerm); err != nil {
		log.Errorf("Failed to create sandbox directory: %v", err)
	}

	poolBase, err := NewWASPBase(cfg, log)
	if err != nil {
		log.Errorf("Failed to create WASP base: %v", err)
		return nil
	}

	wasmedge.SetLogErrorLevel()

	wasi := wasmedge.NewWasiModule(
		nil,
		nil,
		nil,
		//[]string{".:./sandbox"},
	)

	cache, err := caches.NewBaseCacheWithEviction[*RefCountedItem[*wasmedge.AST]](cfg.CacheSize, 0, func(key string, ast *RefCountedItem[*wasmedge.AST]) {
		err := ast.Release()
		if err != nil {
			log.ErrorfWith("Failed to release cache item: %v", []any{err}, "error", err)
			return
		}
		log.Debugf("evicted key: %s", key)
	})
	if err != nil {
		log.Errorf("Failed to create cache: %v", err)
	}

	pool := &WasmedgeExecutorPool{
		WASPBase: poolBase,
		workers:  make(chan *wasmedgeExecutor, cfg.PoolSize),
		wasi:     wasi,
		cache:    cache,
	}

	for i := 0; i < cfg.PoolSize; i++ {
		pool.workers <- newWasmedgeExecutor(wasi, log)
	}
	return pool
}

func (p *WasmedgeExecutorPool) InvokeFunction(fnName string, fnBinary []byte, entrypoint string, input ...[]byte) model.WasmResponse {
	select {
	case worker := <-p.workers:
		defer func() {
			p.workers <- worker
		}()
		return p.doInvokeFunction(worker, fnName, fnBinary, entrypoint, input...)
	case <-getTimeoutChannel(p.cfg.Timeout):
		return model.WasmResponse{
			HttpCode: 503,
			Response: "no available wasm executors in pool",
		}
	}
}

func (p *WasmedgeExecutorPool) doInvokeFunction(worker *wasmedgeExecutor, fnName string, fnBinary []byte, entrypoint string, input ...[]byte) model.WasmResponse {
	p.log.Info("Starting WASM function execution")

	p.log.Debug("Loading WASM module")

	var ast *wasmedge.AST
	var err error

	if p.cfg.WithCache == CacheHybrid {
		var rcm *RefCountedItem[*wasmedge.AST]
		for {
			if astCached, found := p.cache.Get(fnName); found {
				p.log.Debug("WASM module found in cache")
				rcm = astCached
			} else {
				a, err, _ := p.group.Do(fnName, func() (interface{}, error) {
					p.log.Debug("WASM module not found in cache, compiling new module")

					if p.cfg.RunMode == AOT {
						p.log.Debug("Compile the WASM binary in AOT mode")

						if err != nil {
							return nil, fmt.Errorf("failed to create temporary directory for AOT compilation: %v", err)
						}

						err = worker.compiler.CompileBuffer(fnBinary, "./funcBinary-aot-wasmedge.wasm")
						if err != nil {
							return nil, fmt.Errorf("failed to compile wasm file in AOT mode: %v", err)
						}
						fnBinary, err = os.ReadFile("./funcBinary-aot-wasmedge.wasm")
						if err != nil {
							return nil, fmt.Errorf("failed to read AOT compiled wasm file: %v", err)
						}
						// Remove the AOT compiled file after reading it, as it will be cached as a module in memory
						err = os.Remove("./funcBinary-aot-wasmedge.wasm")
						if err != nil {
							return nil, fmt.Errorf("failed to remove AOT compiled wasm file: %v", err)
						}
					}

					a, err := worker.loader.LoadBuffer(fnBinary)
					if err != nil {
						return nil, fmt.Errorf("failed to load wasm file: %v", err)
					}

					err = worker.validator.Validate(a)
					if err != nil {
						return nil, fmt.Errorf("failed to validate WASM module: %v", err)
					}

					rcm := NewRefCountedItem(a, func(ast *wasmedge.AST) error {
						ast.Release()
						return nil
					})
					p.cache.Set(fnName, rcm)
					return rcm, nil
				})

				if err != nil {
					return buildErrorResponse(p.log.ErrorfErrWith(err.Error(), []any{}, "error", err))
				}

				rcm = a.(*RefCountedItem[*wasmedge.AST])
			}

			if rcm.Acquire() {
				break
			}
		}
		ast = rcm.item
		defer rcm.Release()
	} else {
		p.log.Debug("Cache not enabled, compiling module without caching")

		if p.cfg.RunMode == AOT {
			p.log.Debug("Compile the WASM binary in AOT mode")

			// Create a unique temporary directory by using a randomly generated suffix
			// TODO: be careful with high concurrency

			tempDirName := fmt.Sprintf("./wasm-aot-%s-%d", fnName, rand.Int63())
			err := os.Mkdir(tempDirName, os.ModePerm)

			if err != nil {
				return buildErrorResponse(p.log.ErrorfErrWith("failed to create temporary directory for AOT compilation: %v", []any{err}, "error", err))
			}

			// Put the wasm file inside the temporary directory.
			err = os.WriteFile(tempDirName+"/funcBinary.wasm", fnBinary, 0644)
			if err != nil {
				return buildErrorResponse(p.log.ErrorfErrWith("failed to write wasm file to temporary directory: %v", []any{err}, "error", err))
			}

			err = worker.compiler.Compile(tempDirName+"/funcBinary.wasm", tempDirName+"/funcBinary-aot-wasmedge.wasm")
			if err != nil {
				return buildErrorResponse(p.log.ErrorfErrWith("failed to compile wasm file in AOT mode: %v", []any{err}, "error", err))
			}
			fnBinary, err = os.ReadFile(tempDirName + "/funcBinary-aot-wasmedge.wasm")
			if err != nil {
				return buildErrorResponse(p.log.ErrorfErrWith("failed to read AOT compiled wasm file: %v", []any{err}, "error", err))
			}
			// Remove the AOT compiled file after reading it, as it will be cached as a module in memory
			err = os.Remove(tempDirName + "/funcBinary-aot-wasmedge.wasm")
			if err != nil {
				return buildErrorResponse(p.log.ErrorfErrWith("failed to remove AOT compiled wasm file: %v", []any{err}, "error", err))
			}

			// Remove the temporary directory
			err = os.RemoveAll(tempDirName)
			if err != nil {
				return buildErrorResponse(p.log.ErrorfErrWith("failed to remove temporary directory: %v", []any{err}, "error", err))
			}
		}

		a, err := worker.loader.LoadBuffer(fnBinary)
		if err != nil {
			return buildErrorResponse(p.log.ErrorfErrWith("failed to load wasm file: %v", []any{err}, "error", err))
		}

		err = worker.validator.Validate(a)
		if err != nil {
			return buildErrorResponse(p.log.ErrorfErrWith("failed to validate WASM module: %v", []any{err}, "error", err))
		}

		ast = a
		defer ast.Release()
	}

	p.log.Debug("Instantiating module")
	mod, err := worker.exec.Instantiate(worker.store, ast)
	defer mod.Release()
	if err != nil {
		return buildErrorResponse(p.log.ErrorfErrWith("failed to instantiate module: %v", []any{err}, "error", err))
	}

	p.log.Debug("Getting _initialize, malloc and free funcs")
	initFunc := mod.FindFunction("_initialize")
	mallocFunc := mod.FindFunction("malloc")
	freeFunc := mod.FindFunction("free")
	if initFunc == nil || mallocFunc == nil || freeFunc == nil {
		return buildErrorResponse(p.log.ErrorfErr("module does not export _initialize/malloc/free; ensure your TinyGo build exports them", []any{}))
	}

	p.log.Debug("Calling func _initialize")
	_, err = worker.exec.Invoke(initFunc)
	if err != nil {
		return buildErrorResponse(p.log.ErrorfErrWith("failed to execute _initialize: %v", []any{err}, "error", err))
	}

	var inputData []byte
	var entryArgs []any
	for _, arg := range input {
		entryArgs = append(entryArgs, int32(len(arg)))
		inputData = append(inputData, arg...)
	}

	p.log.Debug("Calling malloc")
	// Allocate memory for the request and the context.
	allocateResult, err := worker.exec.Invoke(mallocFunc, int32(len(inputData)))
	if err != nil {
		return buildErrorResponse(p.log.ErrorfErrWith("failed to execute malloc: %v", []any{err}, "error", err))
	}
	ptr := allocateResult[0].(int32)

	p.log.Debug("Writing memory")
	// Write the input into the memory.
	mem := mod.FindMemory("memory")
	if mem == nil {
		return buildErrorResponse(p.log.ErrorfErrWith("Wasm instance does not export memory object: %v", []any{err}, "error", err))
	}

	err = mem.SetData(inputData, uint(ptr), uint(len(inputData)))
	if err != nil {
		p.log.Error("Failed to writing WASM memory", "error", err)
		return buildErrorResponse(p.log.ErrorfErrWith("Failed to writing WASM memory: %v", []any{err}, "error", err))
	}

	p.log.Debug("Running func")
	// Run the `entry` function, passing the request pointer and the length of the buffers.
	entryArgs = append([]any{ptr}, entryArgs...)
	entrypointFn := mod.FindFunction(entrypoint)
	if entrypointFn == nil {
		return buildErrorResponse(p.log.ErrorfErrWith("entrypoint function not found: %v", []any{err}, "error", err))
	}

	entryResult, err := worker.exec.Invoke(entrypointFn, entryArgs...)
	if err != nil {
		return buildErrorResponse(p.log.ErrorfErrWith("Failed execute entry function", []any{}, "error", err))
	}
	outputPointer := entryResult[0].(int32)

	p.log.Debug("Getting output")
	outData, err := mem.GetData(uint(outputPointer), 8)
	if err != nil {
		return buildErrorResponse(p.log.ErrorfErrWith("Failed to reading WASM memory: %v", []any{err}, "error", err))
	}
	resultPointer := binary.LittleEndian.Uint32(outData[:4])
	resultLength := binary.LittleEndian.Uint32(outData[4:])

	// Read the result of the function.
	outData, err = mem.GetData(uint(resultPointer), uint(resultLength))
	if err != nil {
		return buildErrorResponse(p.log.ErrorfErrWith("Failed to reading WASM memory: %v", []any{err}, "error", err))
	}

	p.log.Debug("Freeing memory")
	// Deallocate the buffers.
	_, err = worker.exec.Invoke(freeFunc, ptr)
	if err != nil {
		return buildErrorResponse(p.log.ErrorfErrWith("Failed to free memory: %v", []any{err}, "error", err))
	}

	p.log.Debug("Unmarshalling response")
	wasmResp := model.WasmResponse{}
	err = json.Unmarshal(outData, &wasmResp)
	if err != nil {
		return buildErrorResponse(p.log.ErrorfErrWith("Failed to unmarshal wasm response: %v", []any{err}, "error", err))
	}

	p.log.Info("Ending WASM function execution")
	return wasmResp
}

func (p *WasmedgeExecutorPool) RegisterHostFunctions(fns map[string]any) error {
	externMod := wasmedge.NewModule("extern")

	for fnName, fn := range fns {
		wasmedgeFn, ok := fn.(*wasmedge.Function)
		if !ok {
			return p.log.ErrorfErr("Function %s is not a function", fnName)
		}
		externMod.AddFunction(fnName, wasmedgeFn)
	}

	p.hostFns = externMod
	for i := 0; i < cap(p.workers); i++ {
		w := <-p.workers
		err := w.RegisterHostFunctions(externMod)
		if err != nil {
			p.log.Error("RegisterHostFns failed", "fns", fmt.Sprintf("%+v", fns), "error", err)
			return err
		}
		p.workers <- w
	}

	return nil
}

func (p *WasmedgeExecutorPool) Close() {
	for i := 0; i < cap(p.workers); i++ {
		w := <-p.workers
		w.Close()
	}
	close(p.workers)

	p.wasi.Release()
	if p.hostFns != nil {
		p.hostFns.Release()
	}
}
