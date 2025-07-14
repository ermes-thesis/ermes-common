package wasm

import (
	"encoding/binary"
	"encoding/json"
	"github.com/second-state/WasmEdge-go/wasmedge"
	"log"
	"slices"
)

type wasmComponents struct {
	memory     *wasmedge.Memory
	executor   *wasmedge.Executor
	mallocFunc *wasmedge.Function
}

func (wasmComponent *wasmComponents) getWasmString(ptr int32, len int32) (string, error) {
	res, err := wasmComponent.memory.GetData(uint(ptr), uint(len))
	return string(res), err
}

func (wasmComponent *wasmComponents) getWasmObject(ptr int32, len int32, result any) error {
	res, err := wasmComponent.memory.GetData(uint(ptr), uint(len))
	err = json.Unmarshal(res, result)
	return err
}

func (wasmComponent *wasmComponents) writeError(errCode responseCode, err error) int32 {
	response, err := json.Marshal(wasmHostFunctionResponse{
		Code:    errCode,
		Message: err.Error(),
	})
	if err != nil {
		log.Fatal(err)
	}
	mallocRes, err := wasmComponent.executor.Invoke(wasmComponent.mallocFunc, int32(len(response)+4))
	if err != nil {
		log.Fatal(err)
	}

	resPtr := mallocRes[0].(int32)
	codeBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(codeBytes, uint32(len(response)))
	response = slices.Insert(response, 0, codeBytes...)
	err = wasmComponent.memory.SetData(response, uint(resPtr), uint(len(response)))
	if err != nil {
		log.Fatal(err)
	}
	return resPtr
}

func (wasmComponent *wasmComponents) writeResult(value string) int32 {
	response, err := json.Marshal(wasmHostFunctionResponse{
		Code:    SUCCESS,
		Message: value,
	})
	if err != nil {
		return wasmComponent.writeError(WASM_RUNTIME_ERROR, err)
	}

	mallocRes, err := wasmComponent.executor.Invoke(wasmComponent.mallocFunc, int32(len(response)+4))
	if err != nil {
		return wasmComponent.writeError(WASM_RUNTIME_ERROR, err)
	}

	resPtr := mallocRes[0].(int32)
	codeBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(codeBytes, uint32(len(response)))
	response = slices.Insert(response, 0, codeBytes...)
	err = wasmComponent.memory.SetData(response, uint(resPtr), uint(len(response)))
	if err != nil {
		return wasmComponent.writeError(WASM_RUNTIME_ERROR, err)
	}
	return resPtr
}
