package wasm

import (
	"context"
	"log"
	"os"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"

	"github.com/ermes-thesis/ermes-common/wasm-pool/executor"
)

type WazeroExecutorPool struct {
	*wasmExecutorPool
	runtime wazero.Runtime
	ctx     context.Context
}

func NewWazeroExecutorPool(poolSize int, interpreter bool, ctx context.Context) *WazeroExecutorPool {
	var config wazero.RuntimeConfig
	if interpreter {
		config = wazero.NewRuntimeConfigInterpreter()
	} else {
		config = wazero.NewRuntimeConfig().WithCompilationCache(wazero.NewCompilationCache())
	}

	runtime := wazero.NewRuntimeWithConfig(ctx, config)

	modConf := wazero.NewModuleConfig().
		WithFSConfig(wazero.NewFSConfig().WithDirMount("./sandbox", ".")).
		WithStdout(os.Stdout).
		WithStderr(os.Stderr).
		WithSysNanosleep()

	wasi_snapshot_preview1.MustInstantiate(ctx, runtime)

	pool := &WazeroExecutorPool{
		ctx:              ctx,
		runtime:          runtime,
		wasmExecutorPool: newWasmExecutorPool(poolSize),
	}

	for i := 0; i < poolSize; i++ {
		pool.instances[i] = executor.NewWazeroExecutor(runtime, modConf)
	}
	return pool
}

func (p *WazeroExecutorPool) RegisterHostFns(fns map[string]any) error {
	builder := p.runtime.NewHostModuleBuilder("extern")

	for fnName, fn := range fns {
		builder.NewFunctionBuilder().WithFunc(fn).Export(fnName)
	}

	_, err := builder.Instantiate(p.ctx)
	return err
}

func (p *WazeroExecutorPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	err := p.runtime.Close(p.ctx)
	if err != nil {
		log.Fatalf("Failed to close wazero runtime: %v", err)
	}
}
