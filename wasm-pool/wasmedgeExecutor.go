package wasm

import (
	"github.com/ermes-thesis/ermes-common/logger"

	"github.com/second-state/WasmEdge-go/wasmedge"
)

type wasmedgeExecutor struct {
	store     *wasmedge.Store
	loader    *wasmedge.Loader
	validator *wasmedge.Validator
	exec      *wasmedge.Executor
	compiler  *wasmedge.Compiler
}

func newWasmedgeExecutor(wasi *wasmedge.Module, log logger.Logger) *wasmedgeExecutor {
	e := &wasmedgeExecutor{
		store:     wasmedge.NewStore(),
		loader:    wasmedge.NewLoader(),
		validator: wasmedge.NewValidator(),
		exec:      wasmedge.NewExecutor(),
		compiler:  wasmedge.NewCompiler(),
	}

	err := e.exec.RegisterImport(e.store, wasi)
	if err != nil {
		log.Errorf("Failed to register WASI module: %v", err)
	}

	return e
}

func (e *wasmedgeExecutor) Close() {
	e.store.Release()
	e.loader.Release()
	e.validator.Release()
	e.exec.Release()
	e.compiler.Release()
}

func (e *wasmedgeExecutor) RegisterHostFunctions(extern *wasmedge.Module) error {
	return e.exec.RegisterImport(e.store, extern)
}
