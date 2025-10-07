package utils

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/tetratelabs/wazero/api"
	"log"
	"strings"

	"github.com/ermes-thesis/ermes-common/model"
)

type WazeroAdapter struct {
	Memory     api.Memory
	MallocFunc api.Function
}

func (w WazeroAdapter) GetWasmString(ptr int32, length int32) (string, error) {
	s, err := w.GetWasmStringUnsafe(ptr, length)
	if err != nil {
		return "", err
	}
	return strings.Clone(s), nil
}

func (w WazeroAdapter) GetWasmStringUnsafe(ptr int32, length int32) (string, error) {
	res, ok := w.Memory.Read(uint32(ptr), uint32(length))
	if !ok {
		return "", errors.New("could not get wasm string")
	}

	return string(res), nil
}

func (w WazeroAdapter) GetWasmObject(ptr int32, length int32, result any) error {
	obj, ok := w.Memory.Read(uint32(ptr), uint32(length))
	if !ok {
		return errors.New("could not get wasm object")
	}

	// make a Go-owned copy so unmarshal doesn't reference C memory
	buf := make([]byte, len(obj))
	copy(buf, obj)

	return json.Unmarshal(buf, result)
}

func (w WazeroAdapter) GetWasmObjectUnsafe(ptr int32, length int32, result any) error {
	obj, ok := w.Memory.Read(uint32(ptr), uint32(length))
	if !ok {
		return errors.New("could not get wasm object")
	}

	return json.Unmarshal(obj, result)
}

func (w WazeroAdapter) WriteError(errCode model.ResponseCode, err error) int32 {
	functionResponse := model.WasmHostFunctionResponse[string]{
		Code:    errCode,
		Message: err.Error(),
	}
	response, _ := functionResponse.MarshalBinary()

	mallocRes, err := w.MallocFunc.Call(context.Background(), api.EncodeI32(int32(len(response))))
	if err != nil {
		log.Fatal(err)
	}

	resPtr := api.DecodeI32(mallocRes[0])
	ok := w.Memory.Write(uint32(resPtr), response)
	if !ok {
		log.Fatal("could not write wasm error")
	}
	return resPtr
}

func (w WazeroAdapter) WriteResult(value string) int32 {
	functionResponse := model.WasmHostFunctionResponse[string]{
		Code:    model.Success,
		Message: value,
	}
	response, err := functionResponse.MarshalBinary()
	if err != nil {
		return w.WriteError(model.WasmRuntimeError, err)
	}

	mallocRes, err := w.MallocFunc.Call(context.Background(), api.EncodeI32(int32(len(response))))
	if err != nil {
		return w.WriteError(model.WasmRuntimeError, err)
	}

	resPtr := api.DecodeI32(mallocRes[0])
	ok := w.Memory.Write(uint32(resPtr), response)
	if !ok {
		log.Fatal("could not write wasm result")
	}

	return resPtr
}

func (w WazeroAdapter) WriteArrayResult(values []any) int32 {
	functionResponse := model.WasmHostFunctionResponse[[]any]{
		Code:    model.Success,
		Message: values,
	}
	response, err := functionResponse.MarshalBinary()

	if err != nil {
		return w.WriteError(model.WasmRuntimeError, err)
	}

	mallocRes, err := w.MallocFunc.Call(context.Background(), api.EncodeI32(int32(len(response))))
	if err != nil {
		return w.WriteError(model.WasmRuntimeError, err)
	}

	resPtr := api.DecodeI32(mallocRes[0])
	ok := w.Memory.Write(uint32(resPtr), response)
	if !ok {
		log.Fatal("could not write wasm result")
	}

	return resPtr
}
