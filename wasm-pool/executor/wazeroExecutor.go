package executor

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/ermes-thesis/ermes-common/logger"
	"log"
	"time"

	"github.com/ermes-thesis/ermes-common/model"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

type WazeroExecutor struct {
	runtime   wazero.Runtime
	modConfig wazero.ModuleConfig
	inUse     bool
	log       logger.Logger
}

func NewWazeroExecutor(runtime wazero.Runtime, modConfig wazero.ModuleConfig, log logger.Logger) *WazeroExecutor {
	return &WazeroExecutor{
		runtime:   runtime,
		modConfig: modConfig,
		inUse:     false,
		log:       log,
	}
}

func (exec *WazeroExecutor) Invoke(fnBuffer []byte, entrypoint string, input ...[]byte) model.WasmResponse {
	exec.log.Info("Starting WASM function execution", "time", time.Now().Unix())
	runtime := exec.runtime
	ctx := context.Background()

	exec.log.Debug("Instantiating module", "time", time.Now().Unix())
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
	compiledMod, err := runtime.CompileModule(ctx, fnBuffer)
	if err != nil {
		exec.log.Error("Failed to compile module", "error", err)
		return model.WasmResponse{
			HttpCode: 500,
			Response: fmt.Sprintf("Failed to compile module: %v", err),
		}
	}
	instance, err := runtime.InstantiateModule(ctx, compiledMod, exec.modConfig)
	if err != nil {
		exec.log.Error("Failed to create instance", "error", err)
		return model.WasmResponse{
			HttpCode: 500,
			Response: fmt.Sprintf("failed to instantiating the module: %v", err),
		}
	}

	// If we close the compiled module at the end of the execution, it is removed from the in-memory cache and
	// subsequent calls need to recompile the module. This is suboptimal, but it avoids to make the wazero's
	// cache grow indefinitely
	defer compiledMod.Close(ctx)
	defer instance.Close(ctx)

	exec.log.Debug("Calling func _initialize", "time", time.Now().Unix())
	if initFunc := instance.ExportedFunction("_initialize"); initFunc != nil {
		_, err = initFunc.Call(ctx)
		if err != nil {
			exec.log.Error("Failed to execute _initialize", "error", err)
			return model.WasmResponse{
				HttpCode: 500,
				Response: fmt.Sprintf("failed to execute initialization function: %v", err),
			}
		}
	}

	var inputData []byte
	var entryArgs []uint64
	for _, arg := range input {
		entryArgs = append(entryArgs, api.EncodeI32(int32(len(arg))))
		inputData = append(inputData, arg...)
	}

	exec.log.Debug("Getting malloc and free funcs", "time", time.Now().Unix())
	mallocFunc := instance.ExportedFunction("malloc")
	freeFunc := instance.ExportedFunction("free")
	if mallocFunc == nil || freeFunc == nil {
		exec.log.Error("module does not export malloc/free; ensure your TinyGo build exports them")
		return model.WasmResponse{
			HttpCode: 500,
			Response: fmt.Sprintf("Module does not export malloc/free functions: %v", err),
		}
	}

	exec.log.Debug("Calling malloc", "time", time.Now().Unix())
	allocateResult, err := mallocFunc.Call(ctx, api.EncodeI32(int32(len(inputData))))
	if err != nil {
		log.Fatalf("Failed to execute malloc: %v", err)
	}

	ptr := api.DecodeI32(allocateResult[0])

	exec.log.Debug("Writing memory", "time", time.Now().Unix())
	mem := instance.Memory()
	if mem == nil {
		exec.log.Error("Wasm instance does not export memory object")
		return model.WasmResponse{
			HttpCode: 500,
			Response: fmt.Sprintf("Wasm instance does not export memory object: %v", err),
		}
	}

	ok := mem.Write(uint32(ptr), inputData)
	if !ok {
		exec.log.Error("Failed to writing WASM memory", "error", err)
		return model.WasmResponse{
			HttpCode: 500,
			Response: fmt.Sprintf("Failed to writing WASM memory: %v", err),
		}
	}

	// Run the `entry` function, passing the request pointer and the length of the buffers.
	entryArgs = append([]uint64{api.EncodeI32(ptr)}, entryArgs...)
	entrypointFn := instance.ExportedFunction(entrypoint)
	if entrypointFn == nil {
		exec.log.Error("entrypoint function not found")
		return model.WasmResponse{
			HttpCode: 500,
			Response: fmt.Sprintf("Wasm module does not export entrypoint function: %v", err),
		}
	}

	exec.log.Debug("Running func", "time", time.Now().Unix())
	entryResult, err := entrypointFn.Call(ctx, entryArgs...)
	if err != nil {
		exec.log.Error("Failed execute entry function", "error", err)
		return model.WasmResponse{
			HttpCode: 500,
			Response: fmt.Sprintf("Failed to execute entrypoint function: %v", err),
		}
	}
	outputPointer := api.DecodeI32(entryResult[0])

	exec.log.Debug("Getting output", "time", time.Now().Unix())
	outData, ok := mem.Read(uint32(outputPointer), 8)
	if !ok {
		exec.log.Error("Failed to reading WASM memory", "error", err)
		return model.WasmResponse{
			HttpCode: 500,
			Response: fmt.Sprintf("Failed to reading WASM memory: %v", err),
		}
	}
	resultPointer := binary.LittleEndian.Uint32(outData[:4])
	resultLength := binary.LittleEndian.Uint32(outData[4:])

	// Read the result of the function.
	outData, ok = mem.Read(resultPointer, resultLength)
	if !ok {
		exec.log.Error("Failed to reading WASM memory", "error", err)
		return model.WasmResponse{
			HttpCode: 500,
			Response: fmt.Sprintf("Failed to reading WASM memory: %v", err),
		}
	}

	exec.log.Debug("Freeing memory", "time", time.Now().Unix())
	// Deallocate the buffers.
	_, err = freeFunc.Call(ctx, uint64(ptr))
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

func (exec *WazeroExecutor) Release() {
	exec.inUse = false
}

func (exec *WazeroExecutor) IsInUse() bool {
	return exec.inUse
}

func (exec *WazeroExecutor) SetInUse() {
	exec.inUse = true
}

func (exec *WazeroExecutor) Close() {
}

func (exec *WazeroExecutor) RegisterHostFns(fns map[string]any) error {
	return nil
}
