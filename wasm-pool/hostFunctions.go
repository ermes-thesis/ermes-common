package wasm

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/second-state/WasmEdge-go/wasmedge"
	"slices"
	"strings"
)

type errorCode uint8

const (
	SUCCESS errorCode = iota
	UNAUTHORIZED
	NOT_FOUND
	REDIS_ERROR
)

func hostGet(data interface{}, callframe *wasmedge.CallingFrame, params []interface{}) ([]interface{}, wasmedge.Result) {
	paramPointer := params[0].(int32)
	paramLength := params[1].(int32)
	tokenPointer := params[2].(int32)
	tokenLength := params[3].(int32)

	client := data.(*redis.Client)

	wasmComponent := wasmComponents{
		memory:     callframe.GetMemoryByIndex(0),
		executor:   callframe.GetExecutor(),
		mallocFunc: callframe.GetModule().FindFunction("malloc"),
	}

	var groupToken []string
	err := wasmComponent.getWasmObject(tokenPointer, tokenLength, &groupToken)

	fmt.Println("paramPointer: ", paramPointer)
	fmt.Println("paramLength: ", paramLength)
	// Read the result of the function.
	key, err := wasmComponent.getWasmString(paramPointer, paramLength)

	collId := strings.Split(key, ":")[0]
	res, err := client.SMIsMember(context.Background(), "data:"+collId+":groups", groupToken).Result()
	authorized := slices.Contains(res, true)

	var resPtr any
	if err != nil {
		resPtr, _ = wasmComponent.writeError(REDIS_ERROR)
	} else if !authorized {
		resPtr, _ = wasmComponent.writeError(UNAUTHORIZED)
	} else {
		value, err := client.Get(context.Background(), "data:"+key).Result()
		if err != nil {
			resPtr, _ = wasmComponent.writeError(NOT_FOUND)
		} else {
			resPtr, _ = wasmComponent.writeResult(value)
		}
	}

	// Set the returns
	returns := make([]interface{}, 1)
	returns[0] = resPtr.(int32)

	// Return
	return returns, wasmedge.Result_Success
}

func hostPut(data interface{}, callframe *wasmedge.CallingFrame, params []interface{}) ([]interface{}, wasmedge.Result) {
	namePointer := params[0].(int32)
	nameLength := params[1].(int32)
	valuePointer := params[2].(int32)
	valueLength := params[3].(int32)
	tokenPointer := params[4].(int32)
	tokenLength := params[5].(int32)

	client := data.(*redis.Client)

	wasmComponent := wasmComponents{
		memory:     callframe.GetMemoryByIndex(0),
		executor:   callframe.GetExecutor(),
		mallocFunc: callframe.GetModule().FindFunction("malloc"),
	}

	fmt.Println("namePointer: ", namePointer)
	fmt.Println("nameLength: ", nameLength)

	var groupToken []string
	err := wasmComponent.getWasmObject(tokenPointer, tokenLength, &groupToken)
	key, _ := wasmComponent.getWasmString(namePointer, nameLength)

	collId := strings.Split(key, ":")[0]
	res, err := client.SMIsMember(context.Background(), "data:"+collId+":groups", groupToken).Result()
	authorized := slices.Contains(res, true)

	returns := make([]interface{}, 1)
	if !authorized {
		returns[0] = int32(UNAUTHORIZED)
	} else {
		value, _ := wasmComponent.getWasmString(valuePointer, valueLength)

		err = client.Set(context.Background(), "data:"+key, value, 0).Err()
		if err != nil {
			returns[0] = int32(REDIS_ERROR)
		} else {
			returns[0] = int32(SUCCESS)
		}
	}

	return returns, wasmedge.Result_Success
}

func hostCreateCollection(data interface{}, callframe *wasmedge.CallingFrame, params []interface{}) ([]interface{}, wasmedge.Result) {
	metadataPointer := params[0].(int32)
	metadataLength := params[1].(int32)

	client := data.(*redis.Client)

	wasmComponent := wasmComponents{
		memory:     callframe.GetMemoryByIndex(0),
		executor:   callframe.GetExecutor(),
		mallocFunc: callframe.GetModule().FindFunction("malloc"),
	}

	var metadata map[string]any
	err := wasmComponent.getWasmObject(metadataPointer, metadataLength, &metadata)

	groups := metadata["groups"].([]any)
	collectionId := uuid.NewString()
	key := "data:" + collectionId + ":groups"

	//TODO: mandare i metadati al cloud, che poi li manderà a sua volta al nodo con la geolocation specificata

	err = client.SAdd(context.Background(), key, groups).Err()

	var resPtr any
	if err != nil {
		resPtr, _ = wasmComponent.writeError(REDIS_ERROR)
	} else {
		resPtr, _ = wasmComponent.writeResult(collectionId)
	}

	// Set the returns
	returns := make([]interface{}, 1)
	returns[0] = resPtr.(int32)

	// Return
	return returns, wasmedge.Result_Success
}
