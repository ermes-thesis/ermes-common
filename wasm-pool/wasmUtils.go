package wasm

import (
	"encoding/binary"
	"encoding/json"
	"log"

	"github.com/second-state/WasmEdge-go/wasmedge"
)

type ResponseCode uint8

const (
	Success ResponseCode = iota
	WasmRuntimeError
)

type WasmHostFunctionResponse[T any] struct {
	Code    ResponseCode
	Message T
}

func (r *WasmHostFunctionResponse[T]) MarshalBinary() ([]byte, error) {
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

type WasmComponents struct {
	Memory     *wasmedge.Memory
	Executor   *wasmedge.Executor
	MallocFunc *wasmedge.Function
}

func (wasmComponent *WasmComponents) GetWasmString(ptr int32, len int32) (string, error) {
	res, err := wasmComponent.Memory.GetData(uint(ptr), uint(len))
	return string(res), err
}

func (wasmComponent *WasmComponents) GetWasmObject(ptr int32, len int32, result any) error {
	res, err := wasmComponent.Memory.GetData(uint(ptr), uint(len))
	err = json.Unmarshal(res, result)
	return err
}

func (wasmComponent *WasmComponents) WriteError(errCode ResponseCode, err error) int32 {
	functionResponse := WasmHostFunctionResponse[string]{
		Code:    errCode,
		Message: err.Error(),
	}
	response, _ := functionResponse.MarshalBinary()

	mallocRes, err := wasmComponent.Executor.Invoke(wasmComponent.MallocFunc, int32(len(response)))
	if err != nil {
		log.Fatal(err)
	}

	resPtr := mallocRes[0].(int32)
	err = wasmComponent.Memory.SetData(response, uint(resPtr), uint(len(response)))
	if err != nil {
		log.Fatal(err)
	}
	return resPtr
}

func (wasmComponent *WasmComponents) WriteResult(value string) int32 {
	functionResponse := WasmHostFunctionResponse[string]{
		Code:    Success,
		Message: value,
	}
	response, err := functionResponse.MarshalBinary()
	if err != nil {
		return wasmComponent.WriteError(WasmRuntimeError, err)
	}

	mallocRes, err := wasmComponent.Executor.Invoke(wasmComponent.MallocFunc, int32(len(response)))
	if err != nil {
		return wasmComponent.WriteError(WasmRuntimeError, err)
	}

	resPtr := mallocRes[0].(int32)
	err = wasmComponent.Memory.SetData(response, uint(resPtr), uint(len(response)))
	if err != nil {
		return wasmComponent.WriteError(WasmRuntimeError, err)
	}
	return resPtr
}

func (wasmComponent *WasmComponents) WriteArrayResult(values []any) any {
	functionResponse := WasmHostFunctionResponse[[]any]{
		Code:    Success,
		Message: values,
	}
	response, err := functionResponse.MarshalBinary()

	if err != nil {
		return wasmComponent.WriteError(WasmRuntimeError, err)
	}

	mallocRes, err := wasmComponent.Executor.Invoke(wasmComponent.MallocFunc, int32(len(response)))
	if err != nil {
		return wasmComponent.WriteError(WasmRuntimeError, err)
	}

	resPtr := mallocRes[0].(int32)
	err = wasmComponent.Memory.SetData(response, uint(resPtr), uint(len(response)))
	if err != nil {
		return wasmComponent.WriteError(WasmRuntimeError, err)
	}
	return resPtr
}
