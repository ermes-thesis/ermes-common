package main

import (
	"encoding/binary"
	"fmt"
	"github.com/second-state/WasmEdge-go/wasmedge"
	"os"
)

var dataMap = map[string]string{
	"data:1234:battiti": "80",
	"data:1234:peso":    "60",
}

func hostGet(data interface{}, callframe *wasmedge.CallingFrame, params []interface{}) ([]interface{}, wasmedge.Result) {
	// add: i32, i32 -> i32
	paramPointer := params[0].(int32)
	paramLength := params[1].(int32)
	mem := callframe.GetMemoryByIndex(0)

	fmt.Println("paramPointer: ", paramPointer)
	fmt.Println("paramLength: ", paramLength)
	// Read the result of the function.
	key, _ := mem.GetData(uint(paramPointer), uint(paramLength))
	fmt.Println(string(key))

	value := data.(map[string]string)[string(key)]

	mallocFunc := callframe.GetModule().FindFunction("malloc")
	resPtr, _ := callframe.GetExecutor().Invoke(mallocFunc, int32(len(value)+4))

	memData, _ := mem.GetData(uint(resPtr[0].(int32)), uint(len(value)+4))
	copy(memData[4:], value)
	binary.LittleEndian.PutUint32(memData[:4], uint32(len(value)))

	// Set the returns
	returns := make([]interface{}, 1)
	returns[0] = resPtr[0].(int32)

	// Return
	return returns, wasmedge.Result_Success
}

func main() {
	wasmedge.SetLogErrorLevel()
	conf := wasmedge.NewConfigure(wasmedge.WASI)
	vm := wasmedge.NewVMWithConfig(conf)

	wasi := vm.GetImportModule(wasmedge.WASI)
	wasi.InitWasi(
		os.Args[1:],
		os.Environ(),
		[]string{".:."},
	)

	err := vm.LoadWasmFile("function.wasm")
	if err != nil {
		fmt.Println("failed to load wasm")
	}
	vm.Validate()

	req := "{}"
	lengthReq := len(req)

	ctx := "{" +
		"\"userId\": \"user\"," +
		"\"groupToken\": [\"test\"]," +
		"\"collections\": {}" +
		"}"
	lengthCtx := len(ctx)

	// Create the module instance with the module name "extern".
	impmod := wasmedge.NewModule("extern")

	// Create a function type: {i32, i32} -> {i32}.
	functype := wasmedge.NewFunctionType(
		[]*wasmedge.ValType{wasmedge.NewValTypeI32(), wasmedge.NewValTypeI32()},
		[]*wasmedge.ValType{wasmedge.NewValTypeI32()},
	)

	// Create a function context with the function type and host function body.
	// The third parameter is the pointer to the additional data.
	// Developers should guarantee the life cycle of the data, and it can be
	// `nil` if the external data is not needed.
	// The last parameter can be 0 if developers do not need the cost measuring.
	func_get := wasmedge.NewFunction(functype, hostGet, dataMap, 0)

	// If the function object is not added into an module instance object, it should be released.
	functype.Release()
	impmod.AddFunction("HostGet", func_get)

	err = vm.RegisterModule(impmod)
	if err != nil {
		fmt.Println(err)
		return
	}

	err = vm.Instantiate()
	if err != nil {
		fmt.Println(err)
		return
	}
	// Allocate memory for the request, and get a pointer to it.
	allocateResultCtx, _ := vm.Execute("malloc", int32(lengthCtx))
	ctxPointer := allocateResultCtx[0].(int32)

	// Allocate memory for the context, and get a pointer to it.
	allocateResultReq, _ := vm.Execute("malloc", int32(lengthReq))
	reqPointer := allocateResultReq[0].(int32)

	// Write the request into the memory.
	mod := vm.GetActiveModule()
	mem := mod.FindMemory("memory")
	memData2, _ := mem.GetData(uint(reqPointer), uint(lengthReq))
	copy(memData2, req)

	// Write the context into the memory.
	memData, _ := mem.GetData(uint(ctxPointer), uint(lengthCtx))
	copy(memData, ctx)

	// Run the `entry` function, passing the pointers and the length of the buffers.
	greetResult, _ := vm.Execute("entry", reqPointer, int32(lengthReq), ctxPointer, int32(lengthCtx))
	outputPointer := greetResult[0].(int32)

	memData, _ = mem.GetData(uint(outputPointer), 8)
	resultPointer := binary.LittleEndian.Uint32(memData[:4])
	resultLength := binary.LittleEndian.Uint32(memData[4:])

	// Read the result of the function.
	memData, _ = mem.GetData(uint(resultPointer), uint(resultLength))
	fmt.Println(string(memData))

	// Deallocate the buffers.
	vm.Execute("free", reqPointer)
	vm.Execute("free", ctxPointer)

	vm.Release()
	conf.Release()
}
