package executor

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/bytecodealliance/wasmtime-go/v37"
	"github.com/ermes-thesis/ermes-common/model"

	"log"
)

type WasmtimeExecutor struct {
	linker *wasmtime.Linker
	store  *wasmtime.Store
	inUse  bool
}

func (exec *WasmtimeExecutor) Invoke(fnName string, entrypoint string, input ...[]byte) model.WasmResponse {
	linker := exec.linker
	store := exec.store

	module, err := wasmtime.NewModuleFromFile(linker.Engine, fnName+".wasm")
	if err != nil {
		log.Fatalf("Failed to load wasm file: %v", err)
	}

	instance, err := linker.Instantiate(store, module)
	if err != nil {
		log.Fatalf("Failed to create instance: %v", err)
	}

	if initFunc := instance.GetFunc(store, "_initialize"); initFunc != nil {
		_, err = initFunc.Call(store)
		if err != nil {
			log.Fatalf("Failed to execute _initialize: %v", err)
		}
	}

	var inputData []byte
	var entryArgs []any
	for _, arg := range input {
		entryArgs = append(entryArgs, int32(len(arg)))
		inputData = append(inputData, arg...)
	}

	mallocFunc := instance.GetFunc(store, "malloc")
	freeFunc := instance.GetFunc(store, "free")
	if mallocFunc == nil || freeFunc == nil {
		log.Fatalf("module does not export malloc/free; ensure your TinyGo build exports them")
	}

	allocateResult, err := mallocFunc.Call(store, int32(len(inputData)))
	if err != nil {
		log.Fatalf("Failed to execute malloc: %v", err)
	}
	ptr := allocateResult.(int32)

	memExport := instance.GetExport(store, "memory")
	if memExport == nil {
		log.Fatalf("memory does not export memory")
	}
	mem := memExport.Memory()

	start := int(ptr)
	end := start + len(inputData)
	copy(mem.UnsafeData(store)[start:end], inputData)

	// Run the `entry` function, passing the request pointer and the length of the buffers.
	entryArgs = append([]any{ptr}, entryArgs...)
	entrypointFn := instance.GetFunc(store, entrypoint)
	if entrypointFn == nil {
		log.Fatalf("entrypoint function not found")
	}

	entryResult, err := entrypointFn.Call(store, entryArgs...)
	if err != nil {
		log.Fatalf("Failed execute entry function: %v", err)
	}
	outputPointer := entryResult.(int32)

	outData := make([]byte, 8)
	copy(outData, mem.UnsafeData(store)[int(outputPointer):int(outputPointer)+8])
	resultPointer := binary.LittleEndian.Uint32(outData[:4])
	resultLength := binary.LittleEndian.Uint32(outData[4:])

	// Read the result of the function.
	start = int(resultPointer)
	end = start + int(resultLength)
	outData = make([]byte, resultLength)
	copy(outData, mem.UnsafeData(store)[start:end])
	fmt.Println(string(outData))

	// Deallocate the buffers.
	_, err = freeFunc.Call(store, ptr)
	if err != nil {
		log.Fatalf("Failed to free: %v", err)
	}

	wasmResp := model.WasmResponse{}
	err = json.Unmarshal(outData, &wasmResp)
	if err != nil {
		log.Fatalf("Failed to unmarshal wasm response: %v", err)
	}

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
	exec.linker.Close()
	exec.store.Close()
}

func (exec *WasmtimeExecutor) RegisterHostFns(fns map[string]any) error {
	for fnName, fn := range fns {
		err := exec.linker.DefineFunc(exec.store, "extern", fnName, fn)
		if err != nil {
			return err
		}
	}
	return nil
}
