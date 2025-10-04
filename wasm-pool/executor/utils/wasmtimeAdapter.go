package utils

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/bytecodealliance/wasmtime-go/v37"
)

type WasmtimeAdapter struct {
	Store      wasmtime.Storelike
	Memory     *wasmtime.Memory
	MallocFunc *wasmtime.Func
}

func (w WasmtimeAdapter) GetWasmString(ptr int32, length int32) (string, error) {
	s, err := w.GetWasmStringUnsafe(ptr, length)
	if err != nil {
		return "", err
	}
	return strings.Clone(s), nil
}

func (w WasmtimeAdapter) GetWasmStringUnsafe(ptr int32, length int32) (string, error) {
	start := int(ptr)
	end := start + int(length)
	if ptr < 0 || length < 0 {
		return "", fmt.Errorf("invalid ptr or length: %d, %d", ptr, length)
	}

	res := w.Memory.UnsafeData(w.Store)[start:end]
	return string(res), nil
}

func (w WasmtimeAdapter) GetWasmObject(ptr int32, length int32, result any) error {
	start := int(ptr)
	end := start + int(length)

	if ptr < 0 || length < 0 {
		return fmt.Errorf("invalid ptr or length: %d, %d", ptr, length)
	}

	obj := w.Memory.UnsafeData(w.Store)[start:end]

	// make a Go-owned copy so unmarshal doesn't reference C memory
	buf := make([]byte, len(obj))
	copy(buf, obj)

	return json.Unmarshal(buf, result)
}

func (w WasmtimeAdapter) GetWasmObjectUnsafe(ptr int32, length int32, result any) error {
	start := int(ptr)
	end := start + int(length)

	if ptr < 0 || length < 0 {
		return fmt.Errorf("invalid ptr or length: %d, %d", ptr, length)
	}

	obj := w.Memory.UnsafeData(w.Store)[start:end]

	return json.Unmarshal(obj, result)
}

func (w WasmtimeAdapter) WriteError(errCode ResponseCode, err error) int32 {
	functionResponse := WasmHostFunctionResponse[string]{
		Code:    errCode,
		Message: err.Error(),
	}
	response, _ := functionResponse.MarshalBinary()

	mallocRes, err := w.MallocFunc.Call(w.Store, int32(len(response)))
	if err != nil {
		log.Fatal(err)
	}

	resPtr := mallocRes.(int32)
	start := int(resPtr)
	end := start + len(response)
	copy(w.Memory.UnsafeData(w.Store)[start:end], response)

	return resPtr
}

func (w WasmtimeAdapter) WriteResult(value string) int32 {
	functionResponse := WasmHostFunctionResponse[string]{
		Code:    Success,
		Message: value,
	}
	response, err := functionResponse.MarshalBinary()
	if err != nil {
		return w.WriteError(WasmRuntimeError, err)
	}

	mallocRes, err := w.MallocFunc.Call(w.Store, int32(len(response)))
	if err != nil {
		return w.WriteError(WasmRuntimeError, err)
	}

	resPtr := mallocRes.(int32)
	start := int(resPtr)
	end := start + len(response)
	copy(w.Memory.UnsafeData(w.Store)[start:end], response)

	return resPtr
}

func (w WasmtimeAdapter) WriteArrayResult(values []any) int32 {
	functionResponse := WasmHostFunctionResponse[[]any]{
		Code:    Success,
		Message: values,
	}
	response, err := functionResponse.MarshalBinary()

	if err != nil {
		return w.WriteError(WasmRuntimeError, err)
	}

	mallocRes, err := w.MallocFunc.Call(w.Store, int32(len(response)))
	if err != nil {
		return w.WriteError(WasmRuntimeError, err)
	}

	resPtr := mallocRes.(int32)
	start := int(resPtr)
	end := start + len(response)
	copy(w.Memory.UnsafeData(w.Store)[start:end], response)

	return resPtr
}
