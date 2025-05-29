package wasm

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
)

type wasmResponse struct {
	HttpCode int `json:"http_code"`
	Response any `json:"response"`
}

func (exec *WasmExecutor) InvokeFn(fnName string) {}

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
	authFnResPtr := authFnRes[0].(int32)

	returnedDataPtrLen, err := mem.GetData(uint(authFnResPtr), 8)
	resultPointer := binary.LittleEndian.Uint32(returnedDataPtrLen[:4])
	resultLength := binary.LittleEndian.Uint32(returnedDataPtrLen[4:])

	result, _ := mem.GetData(uint(resultPointer), uint(resultLength))
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
