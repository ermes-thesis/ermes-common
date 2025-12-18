package wasm

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/bytecodealliance/wasmtime-go/v37"
	"github.com/ermes-thesis/ermes-common/caches"
	"github.com/ermes-thesis/ermes-common/logger"
	"github.com/ermes-thesis/ermes-common/model"
	"golang.org/x/sync/singleflight"
)

type WasmtimeExecutorPool struct {
	linker  *wasmtime.Linker
	workers chan struct{}
	isAot   bool
	log     logger.Logger
	timeout time.Duration
	cache   *caches.BaseCache[*wasmtime.Module]
	group   singleflight.Group
}

func NewWasmtimeExecutorPool(cfg Config, log logger.Logger) *WasmtimeExecutorPool {
	if err := os.MkdirAll("./sandbox", os.ModePerm); err != nil {
		log.Errorf("Failed to create sandbox directory: %v", err)
	}

	config := wasmtime.NewConfig()

	var err error
	if cfg.WithCache {
		err = config.CacheConfigLoadDefault()
		if err != nil {
			log.Errorf("Failed to load Wasmtime cache config: %v", err)
		}
	}

	engine := wasmtime.NewEngineWithConfig(config)

	// Create a linker with WASI functions defined within it
	linker := wasmtime.NewLinker(engine)
	err = linker.DefineWasi()
	if err != nil {
		log.Errorf("Failed to define WASI in linker: %v", err)
	}

	cache, err := caches.NewBaseCacheWithEviction[*wasmtime.Module](10, 0, func(key string, value caches.CacheItem[*wasmtime.Module]) {
		value.Value.Close()
	})

	pool := &WasmtimeExecutorPool{
		linker:  linker,
		workers: make(chan struct{}, cfg.PoolSize),
		isAot:   cfg.IsAot,
		log:     log,
		timeout: cfg.Timeout,
		cache:   cache,
	}

	for i := 0; i < cfg.PoolSize; i++ {
		pool.workers <- struct{}{}
	}

	return pool
}

func (p *WasmtimeExecutorPool) Invoke(fnName string, fnBuffer []byte, entrypoint string, input ...[]byte) model.WasmResponse {
	select {
	case worker := <-p.workers:
		defer func() {
			p.workers <- worker
		}()
		return p.doInvoke(fnName, fnBuffer, entrypoint, input...)
	case <-getTimeoutChannel(p.timeout):
		p.log.Error("Local execution failed. The WASM pool was too busy and there weren't any WASM executor available")
		return model.WasmResponse{
			HttpCode: 503,
			Response: fmt.Sprintf("local execution failed. The WASM pool was too busy and there weren't any WASM executor available"),
		}
	}
}

func (p *WasmtimeExecutorPool) doInvoke(fnName string, fnBuffer []byte, entrypoint string, input ...[]byte) model.WasmResponse {
	p.log.Info("Starting WASM function execution")

	store := wasmtime.NewStore(p.linker.Engine)
	wasiConfig := wasmtime.NewWasiConfig()
	store.SetWasi(wasiConfig)
	wasiConfig.Close()
	defer store.Close()

	p.log.Debug("Loading WASM module")
	var module *wasmtime.Module
	var err error
	if p.isAot {
		module, err = wasmtime.NewModuleDeserialize(p.linker.Engine, fnBuffer)
		if err != nil {
			return buildErrorResponse(p.log.ErrorfErrWith("failed to load wasm file: %v", []any{err}, "error", err))
		}
	} else {
		if cachedModule, found := p.cache.Get(fnName); found {
			p.log.Debug("WASM module found in cache")
			module = cachedModule
		} else {
			mod, err, _ := p.group.Do(fnName, func() (interface{}, error) {
				p.log.Debug("WASM module not found in cache, compiling new module")
				module, err = wasmtime.NewModule(p.linker.Engine, fnBuffer)
				if err != nil {
					return nil, err
				}

				p.cache.Set(fnName, module)
				return module, nil
			})
			if err != nil {
				return buildErrorResponse(p.log.ErrorfErrWith("failed to load wasm file: %v", []any{err}, "error", err))
			}
			module = mod.(*wasmtime.Module)
		}
	}

	p.log.Debug("Instantiating module")
	instance, err := p.linker.Instantiate(store, module)
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
	allocateResult, err := mallocFunc.Call(store, int32(len(inputData)))
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
	entryResult, err := entrypointFn.Call(store, entryArgs...)
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
	_, err = freeFunc.Call(store, ptr)
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

func (p *WasmtimeExecutorPool) RegisterHostFns(fns map[string]any) error {
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
