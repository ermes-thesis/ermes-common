package wasm

import (
	"encoding/binary"
	"encoding/json"
	"github.com/second-state/WasmEdge-go/wasmedge"
	"log"
)

type responseCode uint8

const (
	SUCCESS responseCode = iota
	UNAUTHORIZED
	NOT_FOUND
	REDIS_ERROR
	WASM_RUNTIME_ERROR
	HTTP_ERROR
	CENTRAL_REPO_ERROR
)

type wasmHostFunctionResponse[T any] struct {
	Code    responseCode
	Message T
}

func (r *wasmHostFunctionResponse[T]) MarshalBinary() ([]byte, error) {
	var messageBytes []byte
	var err error

	switch v := any(r.Message).(type) {
	case string:
		messageBytes = []byte(v) // use raw string bytes, no quotes
	default:
		messageBytes, err = json.Marshal(v)
		if err != nil {
			return nil, err
		}
	}

	bufLen := 5 + len(messageBytes)
	buf := make([]byte, bufLen)

	binary.LittleEndian.PutUint32(buf, uint32(bufLen))
	buf[4] = byte(r.Code)
	copy(buf[5:], messageBytes)

	return buf, nil
}

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
	functionResponse := wasmHostFunctionResponse[string]{
		Code:    errCode,
		Message: err.Error(),
	}
	response, _ := functionResponse.MarshalBinary()

	mallocRes, err := wasmComponent.executor.Invoke(wasmComponent.mallocFunc, int32(len(response)))
	if err != nil {
		log.Fatal(err)
	}

	resPtr := mallocRes[0].(int32)
	err = wasmComponent.memory.SetData(response, uint(resPtr), uint(len(response)))
	if err != nil {
		log.Fatal(err)
	}
	return resPtr
}

func (wasmComponent *wasmComponents) writeResult(value string) int32 {
	functionResponse := wasmHostFunctionResponse[string]{
		Code:    SUCCESS,
		Message: value,
	}
	response, err := functionResponse.MarshalBinary()
	if err != nil {
		return wasmComponent.writeError(WASM_RUNTIME_ERROR, err)
	}

	mallocRes, err := wasmComponent.executor.Invoke(wasmComponent.mallocFunc, int32(len(response)))
	if err != nil {
		return wasmComponent.writeError(WASM_RUNTIME_ERROR, err)
	}

	resPtr := mallocRes[0].(int32)
	err = wasmComponent.memory.SetData(response, uint(resPtr), uint(len(response)))
	if err != nil {
		return wasmComponent.writeError(WASM_RUNTIME_ERROR, err)
	}
	return resPtr
}

func (wasmComponent *wasmComponents) writeArrayResult(values []any) any {
	functionResponse := wasmHostFunctionResponse[[]any]{
		Code:    SUCCESS,
		Message: values,
	}
	response, err := functionResponse.MarshalBinary()

	if err != nil {
		return wasmComponent.writeError(WASM_RUNTIME_ERROR, err)
	}

	mallocRes, err := wasmComponent.executor.Invoke(wasmComponent.mallocFunc, int32(len(response)))
	if err != nil {
		return wasmComponent.writeError(WASM_RUNTIME_ERROR, err)
	}

	resPtr := mallocRes[0].(int32)
	err = wasmComponent.memory.SetData(response, uint(resPtr), uint(len(response)))
	if err != nil {
		return wasmComponent.writeError(WASM_RUNTIME_ERROR, err)
	}
	return resPtr
}
