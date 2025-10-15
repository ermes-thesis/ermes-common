package executor

import (
	"github.com/ermes-thesis/ermes-common/model"
)

type Executor interface {
	Invoke(fnBuffer []byte, entrypoint string, input ...[]byte) model.WasmResponse
	Release()
	IsInUse() bool
	SetInUse()
	Close()
	RegisterHostFns(fns map[string]any) error
}
