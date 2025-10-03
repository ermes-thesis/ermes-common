package utils

import (
	"github.com/bytecodealliance/wasmtime-go/v37"
	"github.com/ermes-thesis/ermes-wasm-pool/wasm-pool"
	"github.com/second-state/WasmEdge-go/wasmedge"
)

type WasmRuntimeAdapter interface {
	GetWasmString(ptr int32, length int32) (string, error)
	GetWasmStringUnsafe(ptr int32, length int32) (string, error)
	GetWasmObject(ptr int32, length int32, result any) error
	GetWasmObjectUnsafe(ptr int32, length int32, result any) error
	WriteError(errCode wasm.ResponseCode, err error) int32
	WriteResult(value string) int32
	WriteArrayResult(values []any) int32
}

func NewWasmedgeAdapter(Memory *wasmedge.Memory, Executor *wasmedge.Executor, MallocFunc *wasmedge.Function) *WasmedgeAdapter {
	return &WasmedgeAdapter{
		Memory:     Memory,
		Executor:   Executor,
		MallocFunc: MallocFunc,
	}
}

func NewWasmtimeAdapter(store *wasmtime.Store, memory *wasmtime.Memory, mallocFunc *wasmtime.Func) *WasmtimeAdapter {
	return &WasmtimeAdapter{
		Store:      store,
		Memory:     memory,
		MallocFunc: mallocFunc,
	}
}
