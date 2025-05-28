package main

import (
	"encoding/binary"
	"fmt"
	"github.com/redis/go-redis/v9"
	"github.com/second-state/WasmEdge-go/wasmedge"
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
		nil, // <- Qui si possono passare delle stringhe al runtime 🌵
		nil,
		[]string{".:./sandbox"}, // Il runtime può accedere solo alla cartella sandbox (o blocchiamo completamente?)
	)

	// Create the module instance with the module name "extern".
	externMod := wasmedge.NewModule("extern")

	hostGetType := wasmedge.NewFunctionType(
		[]*wasmedge.ValType{wasmedge.NewValTypeI32(), wasmedge.NewValTypeI32(), wasmedge.NewValTypeI32(), wasmedge.NewValTypeI32()},
		[]*wasmedge.ValType{wasmedge.NewValTypeI32()},
	)
	hostPutType := wasmedge.NewFunctionType(
		[]*wasmedge.ValType{wasmedge.NewValTypeI32(), wasmedge.NewValTypeI32(), wasmedge.NewValTypeI32(), wasmedge.NewValTypeI32(), wasmedge.NewValTypeI32(), wasmedge.NewValTypeI32()},
		[]*wasmedge.ValType{wasmedge.NewValTypeI32()},
	)
	hostCreateCollectionType := wasmedge.NewFunctionType(
		[]*wasmedge.ValType{wasmedge.NewValTypeI32(), wasmedge.NewValTypeI32()},
		[]*wasmedge.ValType{wasmedge.NewValTypeI32()},
	)

	// Create a function context with the function type and host function body.
	// The third parameter is the pointer to the additional data.
	// Developers should guarantee the life cycle of the data, and it can be
	// `nil` if the external data is not needed.
	// The last parameter can be 0 if developers do not need the cost measuring.
	hostGetFun := wasmedge.NewFunction(hostGetType, hostGet, nil, 0)
	hostPutFun := wasmedge.NewFunction(hostPutType, hostPut, nil, 0)
	hostCreateCollectionFun := wasmedge.NewFunction(hostCreateCollectionType, hostCreateCollection, nil, 0)

	// If the function object is not added into a module instance object, it should be released.
	hostGetType.Release()
	hostPutType.Release()
	hostCreateCollectionType.Release()
	externMod.AddFunction("hostGet", hostGetFun)
	externMod.AddFunction("hostPut", hostPutFun)
	externMod.AddFunction("hostCreateCollection", hostCreateCollectionFun)

	err := vm.RegisterModule(externMod)
	if err != nil {
		fmt.Println(err)
		return
	}

	//A questo punto in teoria abbiamo il runtime pronto per caricare vari binari wasm e eseguire funzioni
	err = executeWasmBinary(vm, "function.wasm")
	if err != nil {
		fmt.Println(err)
		return
	}

	vm.Release()
	conf.Release()
}

func getTestReqAndCtx() (string, string) {
	req := "{\"prova\": \"ciao\"}"

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
	return req, ctx
}

func executeWasmBinary(vm *wasmedge.VM, name string) error {
	err := vm.LoadWasmFile(name)
	if err != nil {
		fmt.Println("Failed to load wasm", err)
		return err
	}

	err = vm.Validate()
	if err != nil {
		fmt.Println("Validation failed", err)
		return err
	}

	err = vm.Instantiate()
	if err != nil {
		fmt.Println("Instantiation failed", err)
		return err
	}

	_, err = vm.Execute("_initialize")

	req, ctx := getTestReqAndCtx()

	// Allocate memory for the request and the context.
	allocateResult, _ := vm.Execute("malloc", int32(len(req)+len(ctx)))
	reqPointer := allocateResult[0].(int32)

	// Write the request into the memory.
	mod := vm.GetActiveModule()
	mem := mod.FindMemory("memory")
	err = mem.SetData([]byte(req), uint(reqPointer), uint(len(req)))
	if err != nil {
		return err
	}

	// Write the context into the memory.
	err = mem.SetData([]byte(ctx), uint(reqPointer)+uint(len(req)), uint(len(ctx)))
	if err != nil {
		return err
	}

	// Run the `entry` function, passing the request pointer and the length of the buffers.
	greetResult, _ := vm.Execute("entry", reqPointer, int32(len(req)), int32(len(ctx)))
	outputPointer := greetResult[0].(int32)

	outData, _ := mem.GetData(uint(outputPointer), 8)
	resultPointer := binary.LittleEndian.Uint32(outData[:4])
	resultLength := binary.LittleEndian.Uint32(outData[4:])

	// Read the result of the function.
	outData, _ = mem.GetData(uint(resultPointer), uint(resultLength))
	fmt.Println(string(outData))

	// Deallocate the buffers.
	_, err = vm.Execute("free", reqPointer)
	if err != nil {
		return err
	}
	return nil
}
