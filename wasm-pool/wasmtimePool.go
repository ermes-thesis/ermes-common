package wasm

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/bytecodealliance/wasmtime-go/v44"
	"github.com/ermes-thesis/ermes-common/caches"
	"github.com/ermes-thesis/ermes-common/logger"
	"github.com/ermes-thesis/ermes-common/model"
)

type WasmtimeExecutorPool struct {
	*WASPBase
	linker  *wasmtime.Linker
	workers chan struct{}
	cache   *caches.BaseCache[*RefCountedItem[*wasmtime.Module]]
}

func NewWasmtimeExecutorPool(cfg WASPConfig, log logger.Logger) *WasmtimeExecutorPool {
	if err := os.MkdirAll("./sandbox", os.ModePerm); err != nil {
		log.Errorf("Failed to create sandbox directory: %v", err)
	}

	poolBase, err := NewWASPBase(cfg, log)
	if err != nil {
		log.Errorf("Failed to create WASP base: %v", err)
		return nil
	}

	config := wasmtime.NewConfig()

	// Wasmtime own cache on filesystem. Has been removed since now we have our custom in-memory cache. But it should not be a problem
	// to add it again in theory

	if cfg.WithCache >= CacheStandard {
		err = config.CacheConfigLoadDefault()
		if err != nil {
			log.Errorf("Failed to load Wasmtime cache config: %v", err)
		}
	}

	// Uncomment these lines if you want to force Wasmtime to run in interpreted mode.
	// log.Debug("Setup environment to work in interpreted mode")
	// config.SetTarget("pulley64")

	engine := wasmtime.NewEngineWithConfig(config)

	// Create a linker with WASI functions defined within it
	linker := wasmtime.NewLinker(engine)
	err = linker.DefineWasi()
	if err != nil {
		log.Errorf("Failed to define WASI in linker: %v", err)
	}

	cache, err := caches.NewBaseCacheWithEviction[*RefCountedItem[*wasmtime.Module]](cfg.CacheSize, 0, func(key string, value *RefCountedItem[*wasmtime.Module]) {
		e := value.Release()
		if e != nil {
			log.ErrorfWith("Failed to release cache item: %v", []any{e}, "error", e)
			return
		}
		log.Debugf("evicted key: %s", key)
	})
	if err != nil {
		log.Errorf("Failed to create cache: %v", err)
	}

	pool := &WasmtimeExecutorPool{
		WASPBase: poolBase,
		linker:   linker,
		workers:  make(chan struct{}, cfg.PoolSize),
		cache:    cache,
	}

	for i := 0; i < cfg.PoolSize; i++ {
		pool.workers <- struct{}{}
	}

	return pool
}

func (p *WasmtimeExecutorPool) InvokeFunction(fnName string, fnBinary []byte, entrypoint string, input ...[]byte) model.WasmResponse {
	select {
	case worker := <-p.workers:
		defer func() {
			p.workers <- worker
		}()
		return p.doInvokeFunction(fnName, fnBinary, entrypoint, input...)
	case <-getTimeoutChannel(p.cfg.Timeout):
		p.log.Error("Local execution failed. The WASM pool was too busy and there weren't any WASM executor available")
		return model.WasmResponse{
			HttpCode: 503,
			Response: "local execution failed. The WASM pool was too busy and there weren't any WASM executor available",
		}
	}
}

func (p *WasmtimeExecutorPool) doInvokeFunction(fnName string, fnBinary []byte, entrypoint string, input ...[]byte) model.WasmResponse {
	p.log.Info("Starting WASM function execution")

	t := time.Now()
	store := wasmtime.NewStore(p.linker.Engine)
	wasiConfig := wasmtime.NewWasiConfig()
	// wasiConfig.InheritStdout() // Uncomment to redirect WASM stdout to the host's stdout
	store.SetWasi(wasiConfig)
	wasiConfig.Close()
	defer store.Close()
	startUpDuration := time.Since(t)
	if startUpDuration > time.Millisecond*20 {
		p.log.Warnf("WASM store creation took too long: %v", startUpDuration)
	}

	p.log.Debug("Loading WASM module")
	var mod *wasmtime.Module

	if p.cfg.WithCache == CacheHybrid {
		var module *RefCountedItem[*wasmtime.Module]
		for {
			if cachedModule, found := p.cache.Get(fnName); found {
				p.log.Debug("WASM module found in cache")
				module = cachedModule
			} else {
				rcm, err, _ := p.group.Do(fnName, func() (interface{}, error) {
					p.log.Debug("WASM module not found in cache, compiling new module")

					var m *wasmtime.Module
					var e error
					if p.cfg.RunMode == AOT {
						p.log.Debug("Loading module in AOT mode")
						m, e = wasmtime.NewModule(p.linker.Engine, fnBinary)
						fnBinary, e = m.Serialize()
						m, e = wasmtime.NewModuleDeserialize(p.linker.Engine, fnBinary)
					} else {
						p.log.Debug("Loading module in default mode")
						m, e = wasmtime.NewModule(p.linker.Engine, fnBinary)
					}
					if e != nil {
						return nil, fmt.Errorf("failed to load WASM file: %v", e)
					}

					rcm := NewRefCountedItem(m, func(module *wasmtime.Module) error {
						module.Close()
						return nil
					})
					p.cache.Set(fnName, rcm)
					return rcm, nil
				})
				if err != nil {
					return buildErrorResponse(p.log.ErrorfErrWith(err.Error(), []any{}, "error", err))
				}
				module = rcm.(*RefCountedItem[*wasmtime.Module])
			}

			if module.Acquire() {
				break
			}

			p.log.Warnf("Race condition: Module evicted during retrieval. Retrying...")
		}
		mod = module.item
		defer module.Release()
	} else {
		p.log.Debug("WASM module not found in cache, compiling new module")

		var m *wasmtime.Module
		var err error

		if p.cfg.RunMode == AOT {
			p.log.Debug("Loading module in AOT mode")
			m, err = wasmtime.NewModule(p.linker.Engine, fnBinary)
			fnBinary, err = m.Serialize()
			m, err = wasmtime.NewModuleDeserialize(p.linker.Engine, fnBinary)
		} else {
			p.log.Debug("Loading module in interpreted mode")
			m, err = wasmtime.NewModule(p.linker.Engine, fnBinary)
		}

		if err != nil {
			return buildErrorResponse(p.log.ErrorfErrWith("failed to load WASM file: %v", []any{err}, "error", err))
		}

		mod = m
		defer mod.Close()
	}

	t = time.Now()
	p.log.Debug("Instantiating module")
	instance, err := p.linker.Instantiate(store, mod)
	instantiateDuration := time.Since(t)
	if instantiateDuration > time.Millisecond*20 {
		p.log.Warnf("WASM module instantiation took too long: %v", instantiateDuration)
	}

	if err != nil {
		return buildErrorResponse(p.log.ErrorfErrWith("failed to instantiate module: %v", []any{err}, "error", err))
	}

	p.log.Debug("Calling func _initialize")
	if initFunc := instance.GetFunc(store, "_initialize"); initFunc != nil {
		_, err = initFunc.Call(store)
		if err != nil {
			return buildErrorResponse(p.log.ErrorfErrWith("failed to execute _initialize: %v", []any{err}, "error", err))
		}
	}

	var inputData []byte
	entryArgs := make([]any, len(input))
	for i, arg := range input {
		entryArgs[i] = int32(len(arg))
		inputData = append(inputData, arg...)
	}

	p.log.Debug("Getting malloc and free funcs")
	mallocFunc := instance.GetFunc(store, "malloc")
	freeFunc := instance.GetFunc(store, "free")
	if mallocFunc == nil || freeFunc == nil {
		return buildErrorResponse(p.log.ErrorfErrWith("module does not export malloc/free; ensure your TinyGo build exports them: %v", []any{err}, "error", err))
	}

	p.log.Debug("Calling malloc")
	t = time.Now()
	allocateResult, err := mallocFunc.Call(store, int32(len(inputData)))
	allocateDuration := time.Since(t)
	if allocateDuration > time.Millisecond*20 {
		p.log.Warnf("WASM malloc took too long: %v", allocateDuration)
	}
	if err != nil {
		return buildErrorResponse(p.log.ErrorfErrWith("failed to execute malloc: %v", []any{err}, "error", err))
	}
	ptr := allocateResult.(int32)

	p.log.Debug("Writing memory")
	memExport := instance.GetExport(store, "memory")
	if memExport == nil {
		return buildErrorResponse(p.log.ErrorfErrWith("Wasm instance does not export memory object: %v", []any{err}, "error", err))
	}
	mem := memExport.Memory()

	start := int(ptr)
	end := start + len(inputData)
	copy(mem.UnsafeData(store)[start:end], inputData)

	// Run the `entry` function, passing the request pointer and the length of the buffers.
	entryArgs = append([]any{ptr}, entryArgs...)
	entrypointFn := instance.GetFunc(store, entrypoint)
	if entrypointFn == nil {
		return buildErrorResponse(p.log.ErrorfErrWith("entrypoint function not found: %v", []any{err}, "error", err))
	}

	p.log.Debug("Running func")
	t = time.Now()
	entryResult, err := entrypointFn.Call(store, entryArgs...)
	runDuration := time.Since(t)
	if runDuration > time.Millisecond*20 {
		p.log.Warnf("WASM function execution took too long: %v", runDuration)
	}
	if err != nil {
		return buildErrorResponse(p.log.ErrorfErrWith("Failed execute entry function", []any{}, "error", err))
	}
	outputPointer := entryResult.(int32)

	p.log.Debug("Getting output")
	outData := mem.UnsafeData(store)[int(outputPointer) : int(outputPointer)+8]
	resultPointer := binary.LittleEndian.Uint32(outData[:4])
	resultLength := binary.LittleEndian.Uint32(outData[4:])

	// Read the result of the function.
	start = int(resultPointer)
	end = start + int(resultLength)
	outData = mem.UnsafeData(store)[start:end]

	p.log.Debug("Freeing memory")
	// Deallocate the buffers.
	t = time.Now()
	_, err = freeFunc.Call(store, ptr)
	freeDuration := time.Since(t)
	if freeDuration > time.Millisecond*20 {
		p.log.Warnf("WASM free took too long: %v", freeDuration)
	}
	if err != nil {
		return buildErrorResponse(p.log.ErrorfErrWith("Failed to free memory: %v", []any{err}, "error", err))
	}

	p.log.Debug("Unmarshalling response")
	wasmResp := model.WasmResponse{}
	err = json.Unmarshal(outData, &wasmResp)
	if err != nil {
		return buildErrorResponse(p.log.ErrorfErrWith("Failed to unmarshal wasm response\": %v", []any{err}, "error", err))
	}

	p.log.Info("Ending WASM function execution")
	return wasmResp
}

func (p *WasmtimeExecutorPool) RegisterHostFunctions(fns map[string]any) error {
	for fnName, fn := range fns {
		err := p.linker.FuncWrap("extern", fnName, fn)
		if err != nil {
			p.log.Error("RegisterHostFns failed", "fns", fmt.Sprintf("%+v", fns), "error", err)
			return fmt.Errorf("failed to register host functions: %v", err)
		}
	}
	return nil
}

func (p *WasmtimeExecutorPool) Close() {
	for i := 0; i < cap(p.workers); i++ {
		<-p.workers
	}
	close(p.workers)

	p.linker.Close()
}
