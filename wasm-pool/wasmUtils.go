package wasm

import (
	"encoding/binary"
	"encoding/json"
	"github.com/second-state/WasmEdge-go/wasmedge"
	"log"
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

func (wasmComponent *wasmComponents) writeError(error errorCode) any {
	resPtr, err := wasmComponent.executor.Invoke(wasmComponent.mallocFunc, int32(1))
	if err != nil {
		log.Fatal(err)
	}
	err = wasmComponent.memory.SetData([]byte{byte(error)}, uint(resPtr[0].(int32)), uint(1))
	if err != nil {
		log.Fatal(err)
	}
	return resPtr[0]
}

func (wasmComponent *wasmComponents) writeResult(value string) (any, error) {
	resPtr, _ := wasmComponent.executor.Invoke(wasmComponent.mallocFunc, int32(len(value)+5))

	memData, err := wasmComponent.memory.GetData(uint(resPtr[0].(int32)), uint(len(value)+5))
	memData[0] = byte(SUCCESS)
	binary.LittleEndian.PutUint32(memData[1:5], uint32(len(value)))
	copy(memData[5:], value)
	return resPtr[0], err
}
