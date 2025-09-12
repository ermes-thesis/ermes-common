package wasm

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/second-state/WasmEdge-go/wasmedge"
	"log"
	"strconv"
	"sync"
)

type WasmExecutor struct {
	vm    *wasmedge.VM
	mu    sync.Mutex
	inUse bool
}

type wasmResponse struct {
	HttpCode int `json:"http_code"`
	Response any `json:"response"`
}

func (exec *WasmExecutor) InvokeFn(fnName string, req string, ctx string) ([]byte, int) {
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

	// Allocate memory for the request and the context.
	allocateResult, err := vm.Execute("malloc", int32(len(req)+len(ctx)))
	if err != nil {
		log.Fatalf("Failed to execute malloc: %v", err)
	}
	reqPointer := allocateResult[0].(int32)

	// Write the request into the memory.
	mod := vm.GetActiveModule()
	mem := mod.FindMemory("memory")
	err = mem.SetData([]byte(req), uint(reqPointer), uint(len(req)))
	if err != nil {
		log.Fatalf("Failed to set memory: %v", err)
	}

	// Write the context into the memory.
	err = mem.SetData([]byte(ctx), uint(reqPointer)+uint(len(req)), uint(len(ctx)))
	if err != nil {
		log.Fatalf("Failed to set memory: %v", err)
	}

	// Run the `entry` function, passing the request pointer and the length of the buffers.
	entryResult, err := vm.Execute("entry", reqPointer, int32(len(req)), int32(len(ctx)))
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
	_, err = vm.Execute("free", reqPointer)
	if err != nil {
		log.Fatalf("Failed to free: %v", err)
	}

	wasmResp := wasmResponse{}
	err = json.Unmarshal(outData, &wasmResp)
	if err != nil {
		log.Fatalf("Failed to unmarshal wasm response: %v", err)
	}

	res, err := json.Marshal(wasmResp.Response)
	if err != nil {
		log.Fatalf("Failed to marshal response body: %v", err)
	}

	return res, wasmResp.HttpCode
}

func (exec *WasmExecutor) InvokeAuthFn(fnName string, authToken string) (string, int) {
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

	authTokenLen := len(authToken)

	mallocRes, err := vm.Execute("malloc", int32(authTokenLen))
	if err != nil {
		log.Fatalf("Failed to execute malloc: %v", err)
	}
	wasmInputMemPtr := mallocRes[0].(int32)

	mod := vm.GetActiveModule()
	mem := mod.FindMemory("memory")
	err = mem.SetData([]byte(authToken), uint(wasmInputMemPtr), uint(authTokenLen))
	if err != nil {
		log.Fatalf("Failed to set memory: %v", err)
	}

	authFnRes, err := vm.Execute("authEntry", wasmInputMemPtr, int32(authTokenLen))
	if err != nil {
		log.Fatalf("Failed execute auth function: %v", err)
	}
	authFnResPtr := authFnRes[0].(int32)

	returnedDataPtrLen, err := mem.GetData(uint(authFnResPtr), 8)
	resultPointer := binary.LittleEndian.Uint32(returnedDataPtrLen[:4])
	resultLength := binary.LittleEndian.Uint32(returnedDataPtrLen[4:])

	result, err := mem.GetData(uint(resultPointer), uint(resultLength))
	if err != nil {
		log.Fatalf("Failed to get memory: %v", err)
	}
	wasmResp := wasmResponse{}
	err = json.Unmarshal(result, &wasmResp)
	if err != nil {
		log.Fatalf("Failed to unmarshal wasm response: %v", err)
	}

	fmt.Println("Value returned from the WASM auth fn: " + wasmResp.Response.(string))
	fmt.Println("HTTP code returned from the WASM auth fn: " + strconv.Itoa(wasmResp.HttpCode))

	_, err = vm.Execute("free", wasmInputMemPtr)
	if err != nil {
		log.Fatalf("Failed to free: %v", err)
	}

	return wasmResp.Response.(string), wasmResp.HttpCode
}

func (exec *WasmExecutor) Release() {
	exec.mu.Lock()
	defer exec.mu.Unlock()

	exec.inUse = false
}
