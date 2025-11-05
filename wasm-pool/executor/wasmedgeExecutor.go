package executor

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/ermes-thesis/ermes-common/logger"
	"log"
	"time"

	"github.com/ermes-thesis/ermes-common/model"
	"github.com/second-state/WasmEdge-go/wasmedge"
)

type WasmedgeExecutor struct {
	vm    *wasmedge.VM
	inUse bool
	log   logger.Logger
}

func NewWasmedgeExecutor(conf *wasmedge.Configure, log logger.Logger) *WasmedgeExecutor {
	vm := wasmedge.NewVMWithConfig(conf)
	wasi := vm.GetImportModule(wasmedge.WASI)

	wasi.InitWasi(
		nil,
		nil,
		[]string{".:./sandbox"},
	)
	exec := &WasmedgeExecutor{vm: vm, inUse: false, log: log}

	return exec
}

func (exec *WasmedgeExecutor) Invoke(fnBuffer []byte, entrypoint string, input ...[]byte) model.WasmResponse {
	exec.log.Info("Starting WASM function execution", "time", time.Now().Unix())
	vm := exec.vm

	//fmt.Printf("[%s] Loading module\n", time.Now())
	err := vm.LoadWasmBuffer(fnBuffer)
	if err != nil {
		exec.log.Error("Failed to load wasm file", "error", err)
		return model.WasmResponse{
			HttpCode: 500,
			Response: fmt.Sprintf("failed to load wasm file: %v", err),
		}
	}

	err = vm.Validate()
	if err != nil {
		exec.log.Error("Failed to validate WASM module", "error", err)
		return model.WasmResponse{
			HttpCode: 500,
			Response: fmt.Sprintf("failed to validate module: %v", err),
		}
	}

	exec.log.Debug("Instantiating module", "time", time.Now().Unix())
	err = vm.Instantiate()
	if err != nil {
		exec.log.Error("Failed to create instance", "error", err)
		return model.WasmResponse{
			HttpCode: 500,
			Response: fmt.Sprintf("failed to instantiating the module: %v", err),
		}
	}

	exec.log.Debug("Calling func _initialize", "time", time.Now().Unix())
	_, err = vm.Execute("_initialize")
	if err != nil {
		exec.log.Error("Failed to execute _initialize", "error", err)
		return model.WasmResponse{
			HttpCode: 500,
			Response: fmt.Sprintf("failed to execute initialization function: %v", err),
		}
	}

	var inputData []byte
	var entryArgs []any
	for _, arg := range input {
		entryArgs = append(entryArgs, int32(len(arg)))
		inputData = append(inputData, arg...)
	}

	exec.log.Debug("Calling malloc", "time", time.Now().Unix())
	// Allocate memory for the request and the context.
	allocateResult, err := vm.Execute("malloc", int32(len(inputData)))
	if err != nil {
		exec.log.Error("Failed to execute malloc", "error", err)
		return model.WasmResponse{
			HttpCode: 500,
			Response: fmt.Sprintf("Failed to execute malloc function: %v", err),
		}
	}
	ptr := allocateResult[0].(int32)

	exec.log.Debug("Writing memory", "time", time.Now().Unix())
	// Write the input into the memory.
	mod := vm.GetActiveModule()
	mem := mod.FindMemory("memory")
	err = mem.SetData(inputData, uint(ptr), uint(len(inputData)))
	if err != nil {
		exec.log.Error("Failed to writing WASM memory", "error", err)
		return model.WasmResponse{
			HttpCode: 500,
			Response: fmt.Sprintf("Failed to writing WASM memory: %v", err),
		}
	}

	exec.log.Debug("Running func", "time", time.Now().Unix())
	// Run the `entry` function, passing the request pointer and the length of the buffers.
	entryArgs = append([]any{ptr}, entryArgs...)
	entryResult, err := vm.Execute(entrypoint, entryArgs...)
	if err != nil {
		exec.log.Error("Failed execute entry function", "error", err)
		return model.WasmResponse{
			HttpCode: 500,
			Response: fmt.Sprintf("Failed to execute entrypoint function: %v", err),
		}
	}
	outputPointer := entryResult[0].(int32)

	exec.log.Debug("Getting output", "time", time.Now().Unix())
	outData, err := mem.GetData(uint(outputPointer), 8)
	if err != nil {
		exec.log.Error("Failed to reading WASM memory", "error", err)
		return model.WasmResponse{
			HttpCode: 500,
			Response: fmt.Sprintf("Failed to reading WASM memory: %v", err),
		}
	}
	resultPointer := binary.LittleEndian.Uint32(outData[:4])
	resultLength := binary.LittleEndian.Uint32(outData[4:])

	// Read the result of the function.
	outData, err = mem.GetData(uint(resultPointer), uint(resultLength))
	if err != nil {
		exec.log.Error("Failed to reading WASM memory", "error", err)
		return model.WasmResponse{
			HttpCode: 500,
			Response: fmt.Sprintf("Failed to reading WASM memory: %v", err),
		}
	}

	exec.log.Debug("Freeing memory", "time", time.Now().Unix())
	// Deallocate the buffers.
	_, err = vm.Execute("free", ptr)
	if err != nil {
		exec.log.Error("Failed to free memory", "error", err)
		return model.WasmResponse{
			HttpCode: 500,
			Response: fmt.Sprintf("Failed to free memory: %v", err),
		}
	}

	exec.log.Debug("Unmarshalling response", "time", time.Now().Unix())
	wasmResp := model.WasmResponse{}
	err = json.Unmarshal(outData, &wasmResp)
	if err != nil {
		exec.log.Error("Failed to unmarshal wasm response", "error", err)
		return model.WasmResponse{
			HttpCode: 500,
			Response: fmt.Sprintf("Failed to unmarshal wasm response: %v", err),
		}
	}

	exec.log.Info("Ending WASM function execution", "time", time.Now().Unix())
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
