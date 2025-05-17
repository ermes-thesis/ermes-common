package main

import (
	"encoding/binary"
	"fmt"
	"github.com/second-state/WasmEdge-go/wasmedge"
	"os"
)

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
	vm.Instantiate()

	req := "{}"
	lengthReq := len(req)

	ctx := "{" +
		"\"userId\": \"usera\"," +
		"\"groupToken\": [\"test\"]," +
		"\"collections\": {}" +
		"}"
	lengthCtx := len(ctx)

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
