package executor

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"log"

	"github.com/ermes-thesis/ermes-common/model"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

type WazeroExecutor struct {
	runtime   wazero.Runtime
	modConfig wazero.ModuleConfig
	inUse     bool
}

func (exec *WazeroExecutor) Invoke(fnBuffer []byte, entrypoint string, input ...[]byte) model.WasmResponse {
	runtime := exec.runtime
	ctx := context.Background()

	//fmt.Printf("[%s] Instantiating module\n", time.Now())
	// Need to call CompileModule and InstantiateModule separately and not with the convenience method InstantiateWithConfig
	// because the latter removes from the in-memory cache the compiled module, meaning that even if the module is
	compiledMod, err := runtime.CompileModule(ctx, fnBuffer)
	instance, err := runtime.InstantiateModule(ctx, compiledMod, exec.modConfig)
	if err != nil {
		log.Fatalf("Failed to create instance: %v", err)
	}
	defer instance.Close(ctx)

	//fmt.Printf("[%s] Calling func _initialize\n", time.Now())
	if initFunc := instance.ExportedFunction("_initialize"); initFunc != nil {
		_, err = initFunc.Call(ctx)
		if err != nil {
			log.Fatalf("Failed to execute _initialize: %v", err)
		}
	}

	var inputData []byte
	var entryArgs []uint64
	for _, arg := range input {
		entryArgs = append(entryArgs, api.EncodeI32(int32(len(arg))))
		inputData = append(inputData, arg...)
	}

	//fmt.Printf("[%s] Getting malloc and free funcs\n", time.Now())
	mallocFunc := instance.ExportedFunction("malloc")
	freeFunc := instance.ExportedFunction("free")
	if mallocFunc == nil || freeFunc == nil {
		log.Fatalf("module does not export malloc/free; ensure your TinyGo build exports them")
	}

	//fmt.Printf("[%s] Calling malloc\n", time.Now())
	allocateResult, err := mallocFunc.Call(ctx, api.EncodeI32(int32(len(inputData))))
	if err != nil {
		log.Fatalf("Failed to execute malloc: %v", err)
	}

	ptr := api.DecodeI32(allocateResult[0])

	mem := instance.Memory()
	if mem == nil {
		log.Fatalf("memory does not export memory")
	}

	//fmt.Printf("[%s] Writing memory\n", time.Now())
	ok := mem.Write(uint32(ptr), inputData)
	if !ok {
		log.Fatal("Failed to set memory")
	}

	// Run the `entry` function, passing the request pointer and the length of the buffers.
	entryArgs = append([]uint64{api.EncodeI32(ptr)}, entryArgs...)
	entrypointFn := instance.ExportedFunction(entrypoint)
	if entrypointFn == nil {
		log.Fatalf("entrypoint function not found")
	}

	//fmt.Printf("[%s] Running func\n", time.Now())
	entryResult, err := entrypointFn.Call(ctx, entryArgs...)
	if err != nil {
		log.Fatalf("Failed execute entry function: %v", err)
	}
	outputPointer := api.DecodeI32(entryResult[0])

	//fmt.Printf("[%s] Getting output\n", time.Now())
	outData, ok := mem.Read(uint32(outputPointer), 8)
	if !ok {
		log.Fatalf("Failed to read memory")
	}
	resultPointer := binary.LittleEndian.Uint32(outData[:4])
	resultLength := binary.LittleEndian.Uint32(outData[4:])

	// Read the result of the function.
	outData, ok = mem.Read(resultPointer, resultLength)
	if !ok {
		log.Fatalf("Failed to read result")
	}
	//fmt.Println(string(outData))

	//fmt.Printf("[%s] Freeing memory\n", time.Now())
	// Deallocate the buffers.
	_, err = freeFunc.Call(ctx, uint64(ptr))
	if err != nil {
		log.Fatalf("Failed to free: %v", err)
	}

	//fmt.Printf("[%s] Unmarshal\n", time.Now())
	wasmResp := model.WasmResponse{}
	err = json.Unmarshal(outData, &wasmResp)
	if err != nil {
		log.Fatalf("Failed to unmarshal wasm response: %v", err)
	}

	//fmt.Printf("[%s] ------ END ------\n", time.Now())

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
	err := exec.runtime.Close(context.Background())
	if err != nil {
		log.Fatalf("Failed to close runtime: %v", err)
	}
}

func (exec *WazeroExecutor) RegisterHostFns(fns map[string]any) error {
	builder := exec.runtime.NewHostModuleBuilder("extern")

	for fnName, fn := range fns {

		builder.NewFunctionBuilder().WithFunc(fn).Export(fnName)
	}

	_, err := builder.Instantiate(context.Background())
	return err
}
