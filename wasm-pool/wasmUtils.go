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

func (wasmComponent *wasmComponents) writeError(errCode responseCode, err error) any {
	response, err := json.Marshal(wasmHostFunctionResponse{
		Code:    errCode,
		Message: err.Error(),
	})
	if err != nil {
		log.Fatal(err)
	}
	resPtr, err := wasmComponent.executor.Invoke(wasmComponent.mallocFunc, int32(len(response)+4))
	if err != nil {
		log.Fatal(err)
	}

	codeBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(codeBytes, uint32(len(response)))
	response = slices.Insert(response, 0, codeBytes...)
	err = wasmComponent.memory.SetData(response, uint(resPtr[0].(int32)), uint(len(response)+4))
	if err != nil {
		log.Fatal(err)
	}
	return resPtr[0]
}

func (wasmComponent *wasmComponents) writeResult(value string) any {
	var resPtr []any
	response, err := json.Marshal(wasmHostFunctionResponse{
		Code:    SUCCESS,
		Message: value,
	})
	if err != nil {
		resPtr[0] = wasmComponent.writeError(WASM_RUNTIME_ERROR, err)
		return resPtr[0]
	}

	resPtr, err = wasmComponent.executor.Invoke(wasmComponent.mallocFunc, int32(len(response)+4))
	if err != nil {
		resPtr[0] = wasmComponent.writeError(WASM_RUNTIME_ERROR, err)
		return resPtr[0]
	}

	codeBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(codeBytes, uint32(len(response)))
	response = slices.Insert(response, 0, codeBytes...)
	err = wasmComponent.memory.SetData(response, uint(resPtr[0].(int32)), uint(len(response)+4))
	if err != nil {
		resPtr[0] = wasmComponent.writeError(WASM_RUNTIME_ERROR, err)
	}
	return resPtr[0]
}
