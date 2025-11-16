package wasm

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/ermes-thesis/ermes-common/logger"
	"github.com/ermes-thesis/ermes-common/model"
	"github.com/google/uuid"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

type WazeroExecutorPool struct {
	runtime   wazero.Runtime
	modConfig wazero.ModuleConfig
	ctx       context.Context
	log       logger.Logger
	workers   chan struct{}
	withCache bool
	timeout   time.Duration
}

func NewWazeroExecutorPool(cfg Config, ctx context.Context, log logger.Logger) *WazeroExecutorPool {
	if err := os.MkdirAll("./sandbox", os.ModePerm); err != nil {
		log.Errorf("Failed to create sandbox directory: %v", err)
	}

	var config wazero.RuntimeConfig
	if cfg.IsAot {
		config = wazero.NewRuntimeConfig().WithCompilationCache(wazero.NewCompilationCache())
	} else {
		config = wazero.NewRuntimeConfigInterpreter()
	}

	runtime := wazero.NewRuntimeWithConfig(ctx, config)

	modConf := wazero.NewModuleConfig().
		WithSysNanosleep()

	wasi_snapshot_preview1.MustInstantiate(ctx, runtime)

	pool := &WazeroExecutorPool{
		ctx:       ctx,
		runtime:   runtime,
		modConfig: modConf,
		log:       log,
		withCache: cfg.WithCache,
		workers:   make(chan struct{}, cfg.PoolSize),
		//stdOut:    stdOut,
		//stdErr:    stdErr,
		timeout: cfg.Timeout,
	}

	for i := 0; i < cfg.PoolSize; i++ {
		pool.workers <- struct{}{}
	}
	return pool
}

func (p *WazeroExecutorPool) Invoke(fnBuffer []byte, entrypoint string, input ...[]byte) model.WasmResponse {
	select {
	case worker := <-p.workers:
		defer func() {
			p.workers <- worker
		}()
		return p.doInvoke(fnBuffer, entrypoint, input...)
	case <-getTimeoutChannel(p.timeout):
		return model.WasmResponse{
			HttpCode: 500,
			Response: fmt.Sprintf("no available wasm executors in pool"),
		}
	}
}

func (p *WazeroExecutorPool) doInvoke(fnBuffer []byte, entrypoint string, input ...[]byte) model.WasmResponse {
	p.log.Info("Starting WASM function execution")
	runtime := p.runtime

	p.log.Debug("Instantiating module")
	// Wazero API offers another convenience method called InstantiateWithConfig(). This method perform both the
	// compilation and instantiation of a module, combining the effects of calling CompileModule() and InstantiateModule one after the other.
	// The only difference, in behaviour, between calling InstantiateWithConfig() and invoking CompileModule() followed by
	// InstantiateModule() separately is that InstantiateWithConfig, automatically closes the module when we close the instance.
	// When the module is closed, Wazero removes the compiled binary from the in-memory cache. This lead to recompile multiple times the same binary even if it was
	// executed before and for this reason we would like to use the two methods separately.
	//
	// Unfortunately, there is no way to remove a compiled binary from the Wazero's in-memory cache without closing the module.
	// This is a problem for us, because in our system we will likely execute multiple time the same binaries, but as time goes on
	// there could appear new binaries, meaning that the in-memory cache will progressively grow without the possibility to shrink it.
	// For this reason we opted to close manually close the module anyway at the end of the Invoke() execution (meaning that the next time
	// the binary will be recompiled again)
	//
	// A solution to this problem would be to implement a custom LRU cache that allow us to keep track of the different modules used over
	// time and call Close() on the modules used least recently, but we decided not to implement a custom cache for now
	compiledMod, err := runtime.CompileModule(p.ctx, fnBuffer)
	if err != nil {
		return buildErrorResponse(p.log.ErrorfErrWith("Failed to compile module: %v", []any{err}, "error", err))
	}
	instance, err := runtime.InstantiateModule(p.ctx, compiledMod, p.modConfig.WithName(uuid.NewString()))
	if err != nil {
		return buildErrorResponse(p.log.ErrorfErrWith("failed to instantiate module: %v", []any{err}, "error", err))
	}

	// If we close the compiled module at the end of the execution, it is removed from the in-memory cache and
	// subsequent calls need to recompile the module. This is suboptimal, but it avoids to make the wazero's
	// cache grow indefinitely
	if !p.withCache {
		defer compiledMod.Close(p.ctx)
	}
	defer instance.Close(p.ctx)

	p.log.Debug("Getting _initialize, malloc and free funcs")
	initFunc := instance.ExportedFunction("_initialize")
	mallocFunc := instance.ExportedFunction("malloc")
	freeFunc := instance.ExportedFunction("free")
	if initFunc == nil || mallocFunc == nil || freeFunc == nil {
		return buildErrorResponse(p.log.ErrorfErrWith("module does not export _initialize/malloc/free; ensure your TinyGo build exports them: %v", []any{err}, "error", err))
	}

	p.log.Debug("Calling func _initialize")
	_, err = initFunc.Call(p.ctx)
	if err != nil {
		return buildErrorResponse(p.log.ErrorfErrWith("failed to execute _initialize: %v", []any{err}, "error", err))
	}

	var inputData []byte
	var entryArgs []uint64
	for _, arg := range input {
		entryArgs = append(entryArgs, api.EncodeI32(int32(len(arg))))
		inputData = append(inputData, arg...)
	}

	p.log.Debug("Calling malloc")
	allocateResult, err := mallocFunc.Call(p.ctx, api.EncodeI32(int32(len(inputData))))
	if err != nil {
		return buildErrorResponse(p.log.ErrorfErrWith("failed to execute malloc: %v", []any{err}, "error", err))
	}

	ptr := api.DecodeI32(allocateResult[0])

	p.log.Debug("Writing memory")
	mem := instance.Memory()
	if mem == nil {
		return buildErrorResponse(p.log.ErrorfErrWith("Wasm instance does not export memory object: %v", []any{err}, "error", err))
	}

	ok := mem.Write(uint32(ptr), inputData)
	if !ok {
		return buildErrorResponse(p.log.ErrorfErrWith("Failed to writing WASM memory: %v", []any{err}, "error", err))
	}

	// Run the `entry` function, passing the request pointer and the length of the buffers.
	entryArgs = append([]uint64{api.EncodeI32(ptr)}, entryArgs...)
	entrypointFn := instance.ExportedFunction(entrypoint)
	if entrypointFn == nil {
		return buildErrorResponse(p.log.ErrorfErrWith("entrypoint function not found: %v", []any{err}, "error", err))
	}

	p.log.Debug("Running func")
	entryResult, err := entrypointFn.Call(p.ctx, entryArgs...)
	if err != nil {
		return buildErrorResponse(p.log.ErrorfErr("Failed execute entry function: %v", []any{err}, "error", err))
	}
	outputPointer := api.DecodeI32(entryResult[0])

	p.log.Debug("Getting output")
	outData, ok := mem.Read(uint32(outputPointer), 8)
	if !ok {
		return buildErrorResponse(p.log.ErrorfErrWith("Failed to reading WASM memory: %v", []any{err}, "error", err))
	}
	resultPointer := binary.LittleEndian.Uint32(outData[:4])
	resultLength := binary.LittleEndian.Uint32(outData[4:])

	// Read the result of the function.
	outData, ok = mem.Read(resultPointer, resultLength)
	if !ok {
		return buildErrorResponse(p.log.ErrorfErrWith("Failed to reading WASM memory: %v", []any{err}, "error", err))
	}

	p.log.Debug("Freeing memory")
	// Deallocate the buffers.
	_, err = freeFunc.Call(p.ctx, uint64(ptr))
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

func (p *WazeroExecutorPool) RegisterHostFns(fns map[string]any) error {
	builder := p.runtime.NewHostModuleBuilder("extern")

	for fnName, fn := range fns {
		builder.NewFunctionBuilder().WithFunc(fn).Export(fnName)
	}

	_, err := builder.Instantiate(p.ctx)
	if err != nil {
		p.log.Error("RegisterHostFns failed", "fns", fmt.Sprintf("%+v", fns), "error", err)
		return fmt.Errorf("failed to register host functions: %v", err)
	}
	return nil
}

func (p *WazeroExecutorPool) Close() {
	for i := 0; i < cap(p.workers); i++ {
		<-p.workers
	}
	close(p.workers)

	err := p.runtime.Close(p.ctx)
	if err != nil {
		p.log.Errorf("Failed to close wazero runtime: %v", err)
	}
}
