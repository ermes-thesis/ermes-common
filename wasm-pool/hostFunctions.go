package wasm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/second-state/WasmEdge-go/wasmedge"
	"net/http"
	"slices"
	"strings"
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

type wasmHostFunctionResponse struct {
	Code    responseCode
	Message string
}

func hostGet(data any, callframe *wasmedge.CallingFrame, params []any) ([]any, wasmedge.Result) {
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

	returns := make([]any, 1)
	var groupToken []string
	err := wasmComponent.getWasmObject(tokenPointer, tokenLength, &groupToken)
	if err != nil {
		returns[0] = wasmComponent.writeError(WASM_RUNTIME_ERROR, err)
		return returns, wasmedge.Result_Success
	}

	// Read the result of the function.
	key, err := wasmComponent.getWasmString(paramPointer, paramLength)
	if err != nil {
		returns[0] = wasmComponent.writeError(WASM_RUNTIME_ERROR, err)
		return returns, wasmedge.Result_Success
	}

	collId := strings.Split(key, ":")[1]
	res, err := client.SMIsMember(context.Background(), "data:"+collId+":groups", groupToken).Result()
	authorized := slices.Contains(res, true)

	var resPtr any
	if err != nil {
		resPtr = wasmComponent.writeError(REDIS_ERROR, err)
	} else if !authorized {
		resPtr = wasmComponent.writeError(UNAUTHORIZED, errors.New("current user is not authorized"))
	} else {
		value, err := client.Get(context.Background(), key).Result()
		if err != nil {
			resPtr = wasmComponent.writeError(NOT_FOUND, err)
		} else {
			resPtr = wasmComponent.writeResult(value)
		}
	}

	// Set the returns
	returns[0] = resPtr.(int32)

	// Return
	return returns, wasmedge.Result_Success
}

func hostPut(data any, callframe *wasmedge.CallingFrame, params []any) ([]any, wasmedge.Result) {
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

	returns := make([]any, 1)
	var groupToken []string
	err := wasmComponent.getWasmObject(tokenPointer, tokenLength, &groupToken)
	if err != nil {
		returns[0] = wasmComponent.writeError(WASM_RUNTIME_ERROR, err)
		return returns, wasmedge.Result_Success
	}

	key, err := wasmComponent.getWasmString(namePointer, nameLength)
	if err != nil {
		returns[0] = wasmComponent.writeError(WASM_RUNTIME_ERROR, err)
		return returns, wasmedge.Result_Success
	}

	collId := strings.Split(key, ":")[1]
	res, err := client.SMIsMember(context.Background(), "data:"+collId+":groups", groupToken).Result()
	if err != nil {
		returns[0] = wasmComponent.writeError(REDIS_ERROR, err)
		return returns, wasmedge.Result_Success
	}

	authorized := slices.Contains(res, true)

	if !authorized {
		returns[0] = wasmComponent.writeError(UNAUTHORIZED, errors.New("current user is not authorized"))
	} else {
		value, err := wasmComponent.getWasmString(valuePointer, valueLength)
		if err != nil {
			returns[0] = wasmComponent.writeError(WASM_RUNTIME_ERROR, err)
			return returns, wasmedge.Result_Success
		}
		err = client.Set(context.Background(), key, value, 0).Err()
		if err != nil {
			returns[0] = wasmComponent.writeError(REDIS_ERROR, err)
		} else {
			returns[0] = wasmComponent.writeResult("")
		}
	}

	return returns, wasmedge.Result_Success
}

func hostDelete(data any, callframe *wasmedge.CallingFrame, params []any) ([]any, wasmedge.Result) {
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

	// Read the result of the function.
	key, err := wasmComponent.getWasmString(paramPointer, paramLength)

	collId := strings.Split(key, ":")[1]
	res, err := client.SMIsMember(context.Background(), "data:"+collId+":groups", groupToken).Result()
	authorized := slices.Contains(res, true)

	var resPtr any
	if err != nil {
		resPtr = wasmComponent.writeError(REDIS_ERROR, err)
	} else if !authorized {
		resPtr = wasmComponent.writeError(UNAUTHORIZED, errors.New("current user is not authorized"))
	} else {
		err = client.Del(context.Background(), key).Err()
		if err != nil {
			resPtr = wasmComponent.writeError(NOT_FOUND, err)
		} else {
			resPtr = wasmComponent.writeResult("")
		}
	}

	// Set the returns
	returns := make([]any, 1)
	returns[0] = resPtr.(int32)

	// Return
	return returns, wasmedge.Result_Success
}

func hostCreateCollection(data any, callframe *wasmedge.CallingFrame, params []any) ([]any, wasmedge.Result) {
	metadataPointer := params[0].(int32)
	metadataLength := params[1].(int32)

	client := data.(*redis.Client)

	wasmComponent := wasmComponents{
		memory:     callframe.GetMemoryByIndex(0),
		executor:   callframe.GetExecutor(),
		mallocFunc: callframe.GetModule().FindFunction("malloc"),
	}

	var metadata map[string]any
	var resPtr any

	err := wasmComponent.getWasmObject(metadataPointer, metadataLength, &metadata)
	if err != nil {
		resPtr = wasmComponent.writeError(WASM_RUNTIME_ERROR, err)
	}

	groups := metadata["groups"].([]any)
	collectionId := uuid.NewString()
	key := "data:" + collectionId + ":groups"

	//TODO: mandare i metadati al cloud, che poi li manderà a sua volta al nodo con la geolocation specificata

	err = client.SAdd(context.Background(), key, groups).Err()

	if err != nil {
		resPtr = wasmComponent.writeError(REDIS_ERROR, err)
	} else {
		resPtr = wasmComponent.writeResult(collectionId)
	}

	// Set the returns
	returns := make([]any, 1)
	returns[0] = resPtr.(int32)

	// Return
	return returns, wasmedge.Result_Success
}

func hostDeleteCollection(data any, callframe *wasmedge.CallingFrame, params []any) ([]any, wasmedge.Result) {
	collectionIdPtr := params[0].(int32)
	collectionIdLength := params[1].(int32)

	client := data.(*redis.Client)

	wasmComponent := wasmComponents{
		memory:     callframe.GetMemoryByIndex(0),
		executor:   callframe.GetExecutor(),
		mallocFunc: callframe.GetModule().FindFunction("malloc"),
	}

	collectionId, err := wasmComponent.getWasmString(collectionIdPtr, collectionIdLength)
	returns := make([]any, 1)
	if err != nil {
		returns[0] = wasmComponent.writeError(WASM_RUNTIME_ERROR, err)
		return returns, wasmedge.Result_Success
	}

	var cursor uint64
	for {
		var keys []string
		keys, cursor, err = client.Scan(context.Background(), cursor, "data:"+collectionId+":*", 10).Result()
		if err != nil {
			returns[0] = wasmComponent.writeError(REDIS_ERROR, err)
		}

		client.Del(context.Background(), keys...)
		if cursor == 0 {
			break
		}
	}

	//TODO: Forward the deletion process to other nodes with HTTP

	// Return
	returns[0] = wasmComponent.writeResult("")
	return returns, wasmedge.Result_Success
}

func hostRemoveUserFromGroup(data any, callframe *wasmedge.CallingFrame, params []any) ([]any, wasmedge.Result) {
	userIdPtr := params[0].(int32)
	userIdLength := params[1].(int32)
	groupNamePtr := params[2].(int32)
	groupNameLength := params[3].(int32)

	wasmComponent := wasmComponents{
		memory:     callframe.GetMemoryByIndex(0),
		executor:   callframe.GetExecutor(),
		mallocFunc: callframe.GetModule().FindFunction("malloc"),
	}

	returns := make([]any, 1)
	userId, err := wasmComponent.getWasmString(userIdPtr, userIdLength)
	if err != nil {
		returns[0] = wasmComponent.writeError(WASM_RUNTIME_ERROR, err)
		return returns, wasmedge.Result_Success
	}
	groupName, err := wasmComponent.getWasmString(groupNamePtr, groupNameLength)

	centralRepoAddr := fmt.Sprintf("http://%s:%d", CentralRepoIP(), CentralRepoPort())
	req, err := http.NewRequest(http.MethodDelete, centralRepoAddr+"/group/"+groupName+"/"+userId, nil)
	if err != nil {
		returns[0] = wasmComponent.writeError(HTTP_ERROR, err)
		return returns, wasmedge.Result_Success
	}
	req.Header.Set("X-ERMES-SECRET", ErmesSecret())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		returns[0] = wasmComponent.writeError(HTTP_ERROR, err)
		return returns, wasmedge.Result_Success
	}
	_ = res.Body.Close()

	if res.StatusCode == http.StatusOK {
		returns[0] = wasmComponent.writeResult("")
	} else {
		returns[0] = wasmComponent.writeError(CENTRAL_REPO_ERROR, errors.New(res.Status))
	}
	return returns, wasmedge.Result_Success
}

func hostAddUserToGroup(data any, callframe *wasmedge.CallingFrame, params []any) ([]any, wasmedge.Result) {
	userIdPtr := params[0].(int32)
	userIdLength := params[1].(int32)
	groupNamePtr := params[2].(int32)
	groupNameLength := params[3].(int32)

	wasmComponent := wasmComponents{
		memory:     callframe.GetMemoryByIndex(0),
		executor:   callframe.GetExecutor(),
		mallocFunc: callframe.GetModule().FindFunction("malloc"),
	}

	returns := make([]any, 1)
	userId, err := wasmComponent.getWasmString(userIdPtr, userIdLength)
	if err != nil {
		returns[0] = wasmComponent.writeError(WASM_RUNTIME_ERROR, err)
		return returns, wasmedge.Result_Success
	}
	groupName, err := wasmComponent.getWasmString(groupNamePtr, groupNameLength)

	centralRepoAddr := fmt.Sprintf("http://%s:%d", CentralRepoIP(), CentralRepoPort())
	req, err := http.NewRequest(http.MethodPost, centralRepoAddr+"/group/"+groupName+"/"+userId, nil)
	if err != nil {
		returns[0] = wasmComponent.writeError(HTTP_ERROR, err)
		return returns, wasmedge.Result_Success
	}
	req.Header.Set("X-ERMES-SECRET", ErmesSecret())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		returns[0] = wasmComponent.writeError(HTTP_ERROR, err)
		return returns, wasmedge.Result_Success
	}
	_ = res.Body.Close()

	if res.StatusCode == http.StatusOK {
		returns[0] = wasmComponent.writeResult("")
	} else {
		returns[0] = wasmComponent.writeError(CENTRAL_REPO_ERROR, errors.New(res.Status))
	}
	return returns, wasmedge.Result_Success
}

func hostCreateGroup(data any, callframe *wasmedge.CallingFrame, params []any) ([]any, wasmedge.Result) {
	groupNamePtr := params[0].(int32)
	groupNameLength := params[1].(int32)
	participantIDsJsonPtr := params[2].(int32)
	participantIDsJsonLength := params[3].(int32)

	wasmComponent := wasmComponents{
		memory:     callframe.GetMemoryByIndex(0),
		executor:   callframe.GetExecutor(),
		mallocFunc: callframe.GetModule().FindFunction("malloc"),
	}

	returns := make([]any, 1)
	participantIDsJsonStr, err := wasmComponent.getWasmString(participantIDsJsonPtr, participantIDsJsonLength)
	if err != nil {
		returns[0] = wasmComponent.writeError(WASM_RUNTIME_ERROR, err)
		return returns, wasmedge.Result_Success
	}
	groupName, err := wasmComponent.getWasmString(groupNamePtr, groupNameLength)

	centralRepoAddr := fmt.Sprintf("http://%s:%d", CentralRepoIP(), CentralRepoPort())
	req, err := http.NewRequest(http.MethodPost, centralRepoAddr+"/group/"+groupName, bytes.NewBufferString(participantIDsJsonStr))
	if err != nil {
		returns[0] = wasmComponent.writeError(HTTP_ERROR, err)
		return returns, wasmedge.Result_Success
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-ERMES-SECRET", ErmesSecret())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		returns[0] = wasmComponent.writeError(HTTP_ERROR, err)
		return returns, wasmedge.Result_Success
	}
	_ = res.Body.Close()

	if res.StatusCode == http.StatusCreated {
		returns[0] = wasmComponent.writeResult("")
	} else {
		returns[0] = wasmComponent.writeError(CENTRAL_REPO_ERROR, errors.New(res.Status))
	}
	return returns, wasmedge.Result_Success
}

func hostDeleteGroup(data any, callframe *wasmedge.CallingFrame, params []any) ([]any, wasmedge.Result) {
	groupNamePtr := params[0].(int32)
	groupNameLength := params[1].(int32)

	wasmComponent := wasmComponents{
		memory:     callframe.GetMemoryByIndex(0),
		executor:   callframe.GetExecutor(),
		mallocFunc: callframe.GetModule().FindFunction("malloc"),
	}

	returns := make([]any, 1)
	groupName, err := wasmComponent.getWasmString(groupNamePtr, groupNameLength)
	if err != nil {
		returns[0] = wasmComponent.writeError(WASM_RUNTIME_ERROR, err)
		return returns, wasmedge.Result_Success
	}

	centralRepoAddr := fmt.Sprintf("http://%s:%d", CentralRepoIP(), CentralRepoPort())
	req, err := http.NewRequest(http.MethodDelete, centralRepoAddr+"/group/"+groupName, nil)
	if err != nil {
		returns[0] = wasmComponent.writeError(HTTP_ERROR, err)
		return returns, wasmedge.Result_Success
	}
	req.Header.Set("X-ERMES-SECRET", ErmesSecret())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		returns[0] = wasmComponent.writeError(HTTP_ERROR, err)
		return returns, wasmedge.Result_Success
	}
	_ = res.Body.Close()

	if res.StatusCode == http.StatusOK {
		returns[0] = wasmComponent.writeResult("")
	} else {
		returns[0] = wasmComponent.writeError(CENTRAL_REPO_ERROR, errors.New(res.Status))
	}
	return returns, wasmedge.Result_Success
}
