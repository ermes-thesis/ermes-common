package wasm

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/ermes-thesis/ermes-common/logger"
	"github.com/ermes-thesis/ermes-common/model"
	"github.com/second-state/WasmEdge-go/wasmedge"
)

type WasmedgeExecutorPool struct {
	wasi    *wasmedge.Module
	hostFns *wasmedge.Module
	workers chan *wasmedgeExecutor
	timeout time.Duration
	log     logger.Logger
}

func NewWasmedgeExecutorPool(cfg Config, log logger.Logger) *WasmedgeExecutorPool {
	if err := os.MkdirAll("./sandbox", os.ModePerm); err != nil {
		log.Errorf("Failed to create sandbox directory: %v", err)
	}

	wasmedge.SetLogErrorLevel()

	wasi := wasmedge.NewWasiModule(
		nil,
		nil,
		nil,
		//[]string{".:./sandbox"},
	)

	pool := &WasmedgeExecutorPool{
		workers: make(chan *wasmedgeExecutor, cfg.PoolSize),
		log:     log,
		timeout: cfg.Timeout,
		wasi:    wasi,
	}

	for i := 0; i < cfg.PoolSize; i++ {
		pool.workers <- newWasmedgeExecutor(wasi, log)
	}
	return pool
}

func (p *WasmedgeExecutorPool) Invoke(fnName string, fnBuffer []byte, entrypoint string, input ...[]byte) model.WasmResponse {
	select {
	case worker := <-p.workers:
		defer func() {
			p.workers <- worker
		}()
		return p.doInvoke(worker, fnBuffer, entrypoint, input...)
	case <-getTimeoutChannel(p.timeout):
		return model.WasmResponse{
			HttpCode: 503,
			Response: fmt.Sprintf("no available wasm executors in pool"),
		}
	}
}

func (p *WasmedgeExecutorPool) doInvoke(worker *wasmedgeExecutor, fnBuffer []byte, entrypoint string, input ...[]byte) model.WasmResponse {
	p.log.Info("Starting WASM function execution")

	ast, err := worker.loader.LoadBuffer(fnBuffer)
	if err != nil {
		return buildErrorResponse(p.log.ErrorfErrWith("failed to load wasm file: %v", []any{err}, "error", err))
	}

	err = worker.validator.Validate(ast)
	if err != nil {
		return buildErrorResponse(p.log.ErrorfErrWith("failed to validate WASM module: %v", []any{err}, "error", err))
	}

	p.log.Debug("Instantiating module")
	mod, err := worker.exec.Instantiate(worker.store, ast)
	if err != nil {
		return buildErrorResponse(p.log.ErrorfErrWith("failed to instantiate module: %v", []any{err}, "error", err))
	}

	p.log.Debug("Getting _initialize, malloc and free funcs")
	initFunc := mod.FindFunction("_initialize")
	mallocFunc := mod.FindFunction("malloc")
	freeFunc := mod.FindFunction("free")
	if initFunc == nil || mallocFunc == nil || freeFunc == nil {
		return buildErrorResponse(p.log.ErrorfErrWith("module does not export _initialize/malloc/free; ensure your TinyGo build exports them: %v", []any{err}, "error", err))
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

	ast.Release()
	mod.Release()
	p.log.Info("Ending WASM function execution")
	return wasmResp
}

func (p *WasmedgeExecutorPool) RegisterHostFns(fns map[string]any) error {
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
		err := w.RegisterHostFns(externMod)
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
