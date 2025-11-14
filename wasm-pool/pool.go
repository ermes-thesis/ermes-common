package wasm

import (
	"fmt"

	"github.com/ermes-thesis/ermes-common/model"
)

type ExecutorPool interface {
	RegisterHostFns(fns map[string]any) error
	Invoke(fnBuffer []byte, entrypoint string, input ...[]byte) model.WasmResponse
	Close()
}

func buildErrorResponse(err error) model.WasmResponse {
	return model.WasmResponse{
		HttpCode: 500,
		Response: fmt.Sprintf("wasm execution error: %v", err),
	}
}
