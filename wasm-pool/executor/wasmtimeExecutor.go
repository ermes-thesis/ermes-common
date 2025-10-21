package executor

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bytecodealliance/wasmtime-go/v37"
	"github.com/ermes-thesis/ermes-common/logger"
	"github.com/ermes-thesis/ermes-common/model"
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
	exec.log.Info("Starting WASM function execution", "time", time.Now().Unix())
	linker := exec.linker

	// Configure WASI imports to write stdout into a file, and then create
	// a `Store` using this wasi configuration.
	wasiConfig := wasmtime.NewWasiConfig()
	err := wasiConfig.PreopenDir("./sandbox", ".", wasmtime.DIR_WRITE|wasmtime.DIR_READ, wasmtime.FILE_WRITE|wasmtime.FILE_READ)
	if err != nil {
		exec.log.Error("Failed to bind sandbox directory", "entrypoint", entrypoint, "input", string(fnBuffer), "error", err)
		return model.WasmResponse{
			HttpCode: 500,
			Response: fmt.Sprintf("failed to bind sandbox directory: %v", err),
		}
	}
	wasiConfig.InheritStdout()
	wasiConfig.InheritStderr()
	wasiConfig.InheritStdin()

	store := wasmtime.NewStore(linker.Engine)
	store.SetWasi(wasiConfig)

	wasiConfig.Close()

	exec.log.Debug("Loading WASM module", "time", time.Now().Unix())
	module, err := wasmtime.NewModule(linker.Engine, fnBuffer)
	if err != nil {
		exec.log.Error("Failed to load wasm file", "error", err)
		return model.WasmResponse{
			HttpCode: 500,
			Response: fmt.Sprintf("failed to load wasm file: %v", err),
		}
	}

	exec.log.Debug("Instantiating module", "time", time.Now().Unix())
	instance, err := linker.Instantiate(store, module)
	if err != nil {
		exec.log.Error("Failed to create instance", "error", err)
		return model.WasmResponse{
			HttpCode: 500,
			Response: fmt.Sprintf("failed to instantiating the module: %v", err),
		}
	}

	exec.log.Debug("Calling func _initialize", "time", time.Now().Unix())
	if initFunc := instance.GetFunc(store, "_initialize"); initFunc != nil {
		_, err = initFunc.Call(store)
		if err != nil {
			exec.log.Error("Failed to execute _initialize", "error", err)
			return model.WasmResponse{
				HttpCode: 500,
				Response: fmt.Sprintf("failed to execute initialization function: %v", err),
			}
		}
	}

	var inputData []byte
	var entryArgs []any
	for _, arg := range input {
		entryArgs = append(entryArgs, int32(len(arg)))
		inputData = append(inputData, arg...)
	}

	exec.log.Debug("Getting malloc and free funcs", "time", time.Now().Unix())
	mallocFunc := instance.GetFunc(store, "malloc")
	freeFunc := instance.GetFunc(store, "free")
	if mallocFunc == nil || freeFunc == nil {
		exec.log.Error("module does not export malloc/free; ensure your TinyGo build exports them")
		return model.WasmResponse{
			HttpCode: 500,
			Response: fmt.Sprintf("Module does not export malloc/free functions: %v", err),
		}
	}

	exec.log.Debug("Calling malloc", "time", time.Now().Unix())
	allocateResult, err := mallocFunc.Call(store, int32(len(inputData)))
	if err != nil {
		exec.log.Error("Failed to execute malloc", "error", err)
		return model.WasmResponse{
			HttpCode: 500,
			Response: fmt.Sprintf("Failed to execute malloc function: %v", err),
		}
	}
	ptr := allocateResult.(int32)

	exec.log.Debug("Writing memory", "time", time.Now().Unix())
	memExport := instance.GetExport(store, "memory")
	if memExport == nil {
		exec.log.Error("Wasm instance does not export memory object")
		return model.WasmResponse{
			HttpCode: 500,
			Response: fmt.Sprintf("Wasm instance does not export memory object: %v", err),
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
		exec.log.Error("entrypoint function not found")
		return model.WasmResponse{
			HttpCode: 500,
			Response: fmt.Sprintf("Wasm module does not export entrypoint function: %v", err),
		}
	}

	exec.log.Debug("Running func", "time", time.Now().Unix())
	entryResult, err := entrypointFn.Call(store, entryArgs...)
	if err != nil {
		exec.log.Error("Failed execute entry function", "error", err)
		return model.WasmResponse{
			HttpCode: 500,
			Response: fmt.Sprintf("Failed to execute entrypoint function: %v", err),
		}
	}
	outputPointer := entryResult.(int32)

	exec.log.Debug("Getting output", "time", time.Now().Unix())
	outData := make([]byte, 8)
	copy(outData, mem.UnsafeData(store)[int(outputPointer):int(outputPointer)+8])
	resultPointer := binary.LittleEndian.Uint32(outData[:4])
	resultLength := binary.LittleEndian.Uint32(outData[4:])

	// Read the result of the function.
	start = int(resultPointer)
	end = start + int(resultLength)
	outData = make([]byte, resultLength)
	copy(outData, mem.UnsafeData(store)[start:end])

	exec.log.Debug("Freeing memory", "time", time.Now().Unix())
	// Deallocate the buffers.
	_, err = freeFunc.Call(store, ptr)
	if err != nil {
		exec.log.Error("Failed to free memory", "error", err)
		return model.WasmResponse{
			HttpCode: 500,
			Response: fmt.Sprintf("Failed to free memory: %v", err),
		}
	}

	store.Close()

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
