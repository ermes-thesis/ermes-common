package main

import (
	"encoding/binary"
	"fmt"
	"github.com/redis/go-redis/v9"
	"github.com/second-state/WasmEdge-go/wasmedge"
	"os"
	"strconv"
	"time"
)

var client = redis.NewClient(&redis.Options{
	Addr:     "redis:6379",                       // use default Addr
	Password: "eYVX7EwVmmxKPCDmwMtyKVge8oLd2t81", // no password set
	DB:       0,                                  // use default DB
})

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
		"\"collections\": {" +
		"\"test_collection\": [" +
		"{" +
		"\"CollectionId\": \"255ef4af-3a49-4382-a90d-6524bdee5ad1\"," +
		"\"Metadata\": {" +
		"\"name\": \"dati\"," +
		"\"owner\": \"pippo\"," +
		"\"created_at\": " + strconv.FormatInt(time.Now().UnixMilli(), 10) +
		"}" +
		"}" +
		"]" +
		"}" +
		"}"
	lengthCtx := len(ctx)

	// Create the module instance with the module name "extern".
	impmod := wasmedge.NewModule("extern")

	hostGetType := wasmedge.NewFunctionType(
		[]*wasmedge.ValType{wasmedge.NewValTypeI32(), wasmedge.NewValTypeI32()},
		[]*wasmedge.ValType{wasmedge.NewValTypeI32()},
	)
	hostPutType := wasmedge.NewFunctionType(
		[]*wasmedge.ValType{wasmedge.NewValTypeI32(), wasmedge.NewValTypeI32(), wasmedge.NewValTypeI32(), wasmedge.NewValTypeI32()},
		[]*wasmedge.ValType{},
	)

	// Create a function context with the function type and host function body.
	// The third parameter is the pointer to the additional data.
	// Developers should guarantee the life cycle of the data, and it can be
	// `nil` if the external data is not needed.
	// The last parameter can be 0 if developers do not need the cost measuring.
	hostGetFun := wasmedge.NewFunction(hostGetType, hostGet, nil, 0)
	hostPutFun := wasmedge.NewFunction(hostPutType, hostPut, nil, 0)

	// If the function object is not added into an module instance object, it should be released.
	hostGetType.Release()
	hostPutType.Release()
	impmod.AddFunction("HostGet", hostGetFun)
	impmod.AddFunction("HostPut", hostPutFun)

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
