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
		"\"userId\": \"user\"," +
		"\"groupToken\": [\"test\"]," +
		"\"collections\": {}" +
		"}"
	lengthCtx := len(ctx)

	// Allocate memory for the subject, and get a pointer to it.
	// Include a byte for the NULL terminator we add below.
	allocateResultCtx, _ := vm.Execute("malloc", int32(lengthCtx+1))
	ctxPointer := allocateResultCtx[0].(int32)

	// Allocate memory for the subject, and get a pointer to it.
	// Include a byte for the NULL terminator we add below.
	allocateResultReq, _ := vm.Execute("malloc", int32(lengthReq+1))
	reqPointer := allocateResultReq[0].(int32)

	// Write the subject into the memory.
	mod := vm.GetActiveModule()
	mem := mod.FindMemory("memory")
	memData, _ := mem.GetData(uint(ctxPointer), uint(lengthCtx+1))
	copy(memData, ctx)

	// C-string terminates by NULL.
	memData[lengthCtx] = 0

	// Write the subject into the memory.
	memData2, _ := mem.GetData(uint(reqPointer), uint(lengthReq+1))
	copy(memData2, req)

	// C-string terminates by NULL.
	memData2[lengthReq] = 0

	// Run the `greet` function. Given the pointer to the subject.
	greetResult, _ := vm.Execute("entry", reqPointer, int32(lengthReq), ctxPointer, int32(lengthCtx))
	outputPointer := greetResult[0].(int32)

	memData, _ = mem.GetData(uint(outputPointer), 8)
	resultPointer := binary.LittleEndian.Uint32(memData[:4])
	resultLength := binary.LittleEndian.Uint32(memData[4:])

	// Read the result of the `greet` function.
	memData, _ = mem.GetData(uint(resultPointer), uint(resultLength))
	fmt.Println(string(memData))

	// Deallocate the subject, and the output.
	vm.Execute("free", reqPointer)
	vm.Execute("free", ctxPointer)

	vm.Release()
	conf.Release()
}
