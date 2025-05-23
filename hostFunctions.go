package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"github.com/second-state/WasmEdge-go/wasmedge"
)

func hostGet(data interface{}, callframe *wasmedge.CallingFrame, params []interface{}) ([]interface{}, wasmedge.Result) {
	paramPointer := params[0].(int32)
	paramLength := params[1].(int32)
	mem := callframe.GetMemoryByIndex(0)

	fmt.Println("paramPointer: ", paramPointer)
	fmt.Println("paramLength: ", paramLength)
	// Read the result of the function.
	key, _ := mem.GetData(uint(paramPointer), uint(paramLength))

	value, _ := client.Get(context.Background(), "data:"+string(key)).Result()

	mallocFunc := callframe.GetModule().FindFunction("malloc")
	resPtr, _ := callframe.GetExecutor().Invoke(mallocFunc, int32(len(value)+4))

	memData, _ := mem.GetData(uint(resPtr[0].(int32)), uint(len(value)+4))
	copy(memData[4:], value)
	binary.LittleEndian.PutUint32(memData[:4], uint32(len(value)))

	// Set the returns
	returns := make([]interface{}, 1)
	returns[0] = resPtr[0].(int32)

	// Return
	return returns, wasmedge.Result_Success
}

func hostPut(data interface{}, callframe *wasmedge.CallingFrame, params []interface{}) ([]interface{}, wasmedge.Result) {
	namePointer := params[0].(int32)
	nameLength := params[1].(int32)
	valuePointer := params[2].(int32)
	valueLength := params[3].(int32)

	mem := callframe.GetMemoryByIndex(0)

	fmt.Println("namePointer: ", namePointer)
	fmt.Println("nameLength: ", nameLength)

	key, _ := mem.GetData(uint(namePointer), uint(nameLength))
	value, _ := mem.GetData(uint(valuePointer), uint(valueLength))

	client.Set(context.Background(), "data:"+string(key), string(value), 0).Err()

	return nil, wasmedge.Result_Success
}
