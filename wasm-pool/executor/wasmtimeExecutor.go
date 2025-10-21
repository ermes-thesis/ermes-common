package executor

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bytecodealliance/wasmtime-go/v37"
	"github.com/ermes-thesis/ermes-common/logger"
	"github.com/ermes-thesis/ermes-common/model"

	"log"
)

type WasmtimeExecutor struct {
	linker *wasmtime.Linker
	inUse  bool
	log    *logger.Logger
}

func NewWasmtimeExecutor(linker *wasmtime.Linker, log *logger.Logger) *WasmtimeExecutor {
	return &WasmtimeExecutor{
		linker: linker,
		inUse:  false,
		log:    log,
	}
}

func (exec *WasmtimeExecutor) Invoke(fnBuffer []byte, entrypoint string, input ...[]byte) model.WasmResponse {
	linker := exec.linker

	// Configure WASI imports to write stdout into a file, and then create
	// a `Store` using this wasi configuration.
	wasiConfig := wasmtime.NewWasiConfig()
	err := wasiConfig.PreopenDir("./sandbox", ".", wasmtime.DIR_WRITE|wasmtime.DIR_READ, wasmtime.FILE_WRITE|wasmtime.FILE_READ)
	wasiConfig.InheritStdout()
	wasiConfig.InheritStderr()
	wasiConfig.InheritStdin()
	if err != nil {
		log.Fatal(err)
	}

	store := wasmtime.NewStore(linker.Engine)
	store.SetWasi(wasiConfig)

	wasiConfig.Close()

	fmt.Printf("[%s] Loading module\n", time.Now())
	module, err := wasmtime.NewModule(linker.Engine, fnBuffer)
	if err != nil {
		exec.log.Errorf("Failed to load wasm file: %v", err)
	}

	fmt.Printf("[%s] Instantiating module\n", time.Now())
	instance, err := linker.Instantiate(store, module)
	if err != nil {
		exec.log.Errorf("Failed to create instance: %v", err)
	}

	exec.log.Debugf("[%s] Calling func _initialize\n", time.Now())
	if initFunc := instance.GetFunc(store, "_initialize"); initFunc != nil {
		_, err = initFunc.Call(store)
		if err != nil {
			exec.log.Errorf("Failed to execute _initialize: %v", err)
		}
	}

	var inputData []byte
	var entryArgs []any
	for _, arg := range input {
		entryArgs = append(entryArgs, int32(len(arg)))
		inputData = append(inputData, arg...)
	}

	exec.log.Debugf("[%s] Getting malloc and free funcs\n", time.Now())
	mallocFunc := instance.GetFunc(store, "malloc")
	freeFunc := instance.GetFunc(store, "free")
	if mallocFunc == nil || freeFunc == nil {
		exec.log.Errorf("module does not export malloc/free; ensure your TinyGo build exports them")
	}

	exec.log.Debugf("[%s] Calling malloc\n", time.Now())
	allocateResult, err := mallocFunc.Call(store, int32(len(inputData)))
	if err != nil {
		exec.log.Errorf("Failed to execute malloc: %v", err)
	}
	ptr := allocateResult.(int32)

	exec.log.Debugf("[%s] Writing memory\n", time.Now())
	memExport := instance.GetExport(store, "memory")
	if memExport == nil {
		exec.log.Errorf("memory does not export memory")
		return model.WasmResponse{
			HttpCode: 500,
			Response: []byte("memory export not found"),
		}
	}
	mem := memExport.Memory()

	start := int(ptr)
	end := start + len(inputData)
	copy(mem.UnsafeData(store)[start:end], inputData)

	// Run the `entry` function, passing the request pointer and the length of the buffers.
	entryArgs = append([]any{ptr}, entryArgs...)
	entrypointFn := instance.GetFunc(store, entrypoint)
	if entrypointFn == nil {
		exec.log.Errorf("entrypoint function not found")
	}

	exec.log.Debugf("[%s] Running func\n", time.Now())
	entryResult, err := entrypointFn.Call(store, entryArgs...)
	if err != nil {
		exec.log.Errorf("Failed execute entry function: %v", err)
	}
	outputPointer := entryResult.(int32)

	exec.log.Debugf("[%s] Getting output\n", time.Now())
	outData := make([]byte, 8)
	copy(outData, mem.UnsafeData(store)[int(outputPointer):int(outputPointer)+8])
	resultPointer := binary.LittleEndian.Uint32(outData[:4])
	resultLength := binary.LittleEndian.Uint32(outData[4:])

	// Read the result of the function.
	start = int(resultPointer)
	end = start + int(resultLength)
	outData = make([]byte, resultLength)
	copy(outData, mem.UnsafeData(store)[start:end])
	exec.log.Debug(string(outData))

	exec.log.Debugf("[%s] Freeing memory\n", time.Now())
	// Deallocate the buffers.
	_, err = freeFunc.Call(store, ptr)
	if err != nil {
		exec.log.Errorf("Failed to free: %v", err)
	}

	store.Close()

	exec.log.Debugf("[%s] Unmarshal\n", time.Now())
	wasmResp := model.WasmResponse{}
	err = json.Unmarshal(outData, &wasmResp)
	if err != nil {
		exec.log.Errorf("Failed to unmarshal wasm response: %v", err)
	}

	exec.log.Debugf("[%s] ------ END ------\n", time.Now())

	return wasmResp
}

func (exec *WasmtimeExecutor) Release() {
	exec.inUse = false
}

func (exec *WasmtimeExecutor) IsInUse() bool {
	return exec.inUse
}

func (exec *WasmtimeExecutor) SetInUse() {
	exec.inUse = true
}

func (exec *WasmtimeExecutor) Close() {
}

func (exec *WasmtimeExecutor) RegisterHostFns(fns map[string]any) error {
	return nil
}
