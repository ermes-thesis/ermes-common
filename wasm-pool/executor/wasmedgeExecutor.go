package executor

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"

	"github.com/ermes-thesis/ermes-common/model"
	"github.com/second-state/WasmEdge-go/wasmedge"
)

type WasmedgeExecutor struct {
	vm    *wasmedge.VM
	inUse bool
}

func (exec *WasmedgeExecutor) Invoke(fnName string, entrypoint string, input ...[]byte) model.WasmResponse {
	vm := exec.vm
	err := vm.LoadWasmFile(fnName + ".wasm")
	if err != nil {
		log.Fatalf("Failed to load wasm file: %v", err)
	}

	err = vm.Validate()
	if err != nil {
		log.Fatalf("Failed to validate wasm file: %v", err)
	}

	err = vm.Instantiate()
	if err != nil {
		log.Fatalf("Failed to instantiate wasm file: %v", err)
	}

	_, err = vm.Execute("_initialize")
	if err != nil {
		log.Fatalf("Failed to execute wasm file: %v", err)
	}

	var inputData []byte
	var entryArgs []any
	for _, arg := range input {
		entryArgs = append(entryArgs, int32(len(arg)))
		inputData = append(inputData, arg...)
	}

	// Allocate memory for the request and the context.
	allocateResult, err := vm.Execute("malloc", int32(len(inputData)))
	if err != nil {
		log.Fatalf("Failed to execute malloc: %v", err)
	}
	ptr := allocateResult[0].(int32)

	// Write the input into the memory.
	mod := vm.GetActiveModule()
	mem := mod.FindMemory("memory")
	err = mem.SetData(inputData, uint(ptr), uint(len(inputData)))
	if err != nil {
		log.Fatalf("Failed to set memory: %v", err)
	}

	// Run the `entry` function, passing the request pointer and the length of the buffers.
	entryArgs = append([]any{ptr}, entryArgs...)
	entryResult, err := vm.Execute(entrypoint, entryArgs...)
	if err != nil {
		log.Fatalf("Failed execute entry function: %v", err)
	}
	outputPointer := entryResult[0].(int32)

	outData, err := mem.GetData(uint(outputPointer), 8)
	if err != nil {
		log.Fatalf("Failed to get memory: %v", err)
	}
	resultPointer := binary.LittleEndian.Uint32(outData[:4])
	resultLength := binary.LittleEndian.Uint32(outData[4:])

	// Read the result of the function.
	outData, err = mem.GetData(uint(resultPointer), uint(resultLength))
	if err != nil {
		log.Fatalf("Failed to get memory: %v", err)
	}
	fmt.Println(string(outData))

	// Deallocate the buffers.
	_, err = vm.Execute("free", ptr)
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

func (exec *WasmedgeExecutor) Release() {
	exec.inUse = false
}

func (exec *WasmedgeExecutor) IsInUse() bool {
	return exec.inUse
}

func (exec *WasmedgeExecutor) SetInUse() {
	exec.inUse = true
}

func (exec *WasmedgeExecutor) Close() {
	exec.vm.Release()
}

func (exec *WasmedgeExecutor) RegisterHostFns(hostFns map[string]any) error {
	// Create the module instance with the module name "extern".
	externMod := wasmedge.NewModule("extern")

	for fnName, fn := range hostFns {
		wasmedgeFn, ok := fn.(*wasmedge.Function)
		if !ok {
			log.Fatalf("Function %s is not a function", fnName)
		}
		externMod.AddFunction(fnName, wasmedgeFn)
	}

	return exec.vm.RegisterModule(externMod)
}
