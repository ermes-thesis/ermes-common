package wasm

import (
	"fmt"
	"time"

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

func getTimeoutChannel(timeout time.Duration) <-chan time.Time {
	var timeoutChan <-chan time.Time

	if timeout > 0 {
		timeoutChan = time.After(timeout)
	}
	return timeoutChan
}
