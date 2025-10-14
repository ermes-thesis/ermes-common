package executor

import (
	"context"
	"log"
	"os"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"

	"github.com/bytecodealliance/wasmtime-go/v37"
	"github.com/ermes-thesis/ermes-common/model"
	"github.com/second-state/WasmEdge-go/wasmedge"
)

type Executor interface {
	Invoke(fnBuffer []byte, entrypoint string, input ...[]byte) model.WasmResponse
	Release()
	IsInUse() bool
	SetInUse()
	Close()
	RegisterHostFns(fns map[string]any) error
}

func NewWasmedgeExecutor(conf *wasmedge.Configure) *WasmedgeExecutor {
	vm := wasmedge.NewVMWithConfig(conf)
	wasi := vm.GetImportModule(wasmedge.WASI)

	err := os.MkdirAll("./sandbox", os.ModePerm)
	if err != nil {
		log.Fatalf("Failed to create sandbox directory: %v", err)
	}

	wasi.InitWasi(
		nil,
		nil,
		[]string{".:./sandbox"},
	)
	exec := &WasmedgeExecutor{vm: vm, inUse: false}

	return exec
}

func NewWasmtimeExecutor(linker *wasmtime.Linker) *WasmtimeExecutor {
	return &WasmtimeExecutor{
		linker: linker,
		inUse:  false,
	}
}

func NewWazeroExecutor(conf wazero.RuntimeConfig) *WazeroExecutor {
	ctx := context.Background()

	runtime := wazero.NewRuntimeWithConfig(ctx, conf)

	modConf := wazero.NewModuleConfig().
		WithFSConfig(wazero.NewFSConfig().WithDirMount("./sandbox", ".")).
		WithStdout(os.Stdout).
		WithStderr(os.Stderr)

	// optional: create sandbox dir
	if err := os.MkdirAll("./sandbox", os.ModePerm); err != nil {
		log.Fatalf("Failed to create sandbox directory: %v", err)
	}

	wasi_snapshot_preview1.MustInstantiate(ctx, runtime)

	return &WazeroExecutor{
		runtime:   runtime,
		modConfig: modConf,
		inUse:     false,
	}
}
