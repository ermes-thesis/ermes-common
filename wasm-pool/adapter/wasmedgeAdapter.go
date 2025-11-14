package adapter

import (
	"encoding/json"
	"log"

	"github.com/ermes-thesis/ermes-common/model"
	"github.com/second-state/WasmEdge-go/wasmedge"
)

type WasmedgeAdapter struct {
	Memory     *wasmedge.Memory
	Executor   *wasmedge.Executor
	MallocFunc *wasmedge.Function
}

func (wasmComponent *WasmedgeAdapter) GetWasmString(ptr int32, length int32) (string, error) {
	res, err := wasmComponent.Memory.GetData(uint(ptr), uint(length))
	// With append we create a Go-owned copy so that we don't directly
	// reference the WASM memory. Which later can become invalid
	return string(append([]byte(nil), res...)), err
}

func (wasmComponent *WasmedgeAdapter) GetWasmStringUnsafe(ptr int32, length int32) (string, error) {
	res, err := wasmComponent.Memory.GetData(uint(ptr), uint(length))
	return string(res), err
}

func (wasmComponent *WasmedgeAdapter) GetWasmObject(ptr int32, length int32, result any) error {
	res, err := wasmComponent.Memory.GetData(uint(ptr), uint(length))
	if err != nil {
		return err
	}
	// make a Go-owned copy so unmarshal doesn't reference C memory
	buf := make([]byte, len(res))
	copy(buf, res)

	return json.Unmarshal(buf, result)
}

func (wasmComponent *WasmedgeAdapter) GetWasmObjectUnsafe(ptr int32, length int32, result any) error {
	res, err := wasmComponent.Memory.GetData(uint(ptr), uint(length))
	if err != nil {
		return err
	}
	return json.Unmarshal(res, result)
}

func (wasmComponent *WasmedgeAdapter) WriteError(errCode model.ResponseCode, err error) int32 {
	functionResponse := model.WasmHostFunctionResponse[string]{
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

func (wasmComponent *WasmedgeAdapter) WriteResult(value string) int32 {
	functionResponse := model.WasmHostFunctionResponse[string]{
		Code:    model.Success,
		Message: value,
	}
	response, err := functionResponse.MarshalBinary()
	if err != nil {
		return wasmComponent.WriteError(model.WasmRuntimeError, err)
	}

	mallocRes, err := wasmComponent.Executor.Invoke(wasmComponent.MallocFunc, int32(len(response)))
	if err != nil {
		return wasmComponent.WriteError(model.WasmRuntimeError, err)
	}

	resPtr := mallocRes[0].(int32)
	err = wasmComponent.Memory.SetData(response, uint(resPtr), uint(len(response)))
	if err != nil {
		return wasmComponent.WriteError(model.WasmRuntimeError, err)
	}
	return resPtr
}

func (wasmComponent *WasmedgeAdapter) WriteArrayResult(values []any) int32 {
	functionResponse := model.WasmHostFunctionResponse[[]any]{
		Code:    model.Success,
		Message: values,
	}
	response, err := functionResponse.MarshalBinary()

	if err != nil {
		return wasmComponent.WriteError(model.WasmRuntimeError, err)
	}

	mallocRes, err := wasmComponent.Executor.Invoke(wasmComponent.MallocFunc, int32(len(response)))
	if err != nil {
		return wasmComponent.WriteError(model.WasmRuntimeError, err)
	}

	resPtr := mallocRes[0].(int32)
	err = wasmComponent.Memory.SetData(response, uint(resPtr), uint(len(response)))
	if err != nil {
		return wasmComponent.WriteError(model.WasmRuntimeError, err)
	}
	return resPtr
}
