package wasm

import (
	"fmt"
	"github.com/bytecodealliance/wasmtime-go/v37"
	"github.com/ermes-thesis/ermes-common/logger"
	"github.com/ermes-thesis/ermes-common/wasm-pool/executor"
)

type WasmtimeExecutorPool struct {
	*wasmExecutorPool
	linker *wasmtime.Linker
}

func NewWasmtimeExecutorPool(poolSize int, log *logger.Logger) *WasmtimeExecutorPool {
	cfg := wasmtime.NewConfig()
	err := cfg.CacheConfigLoadDefault()
	if err != nil {
		log.Errorf("Failed to load Wasmtime cache config: %v", err)
	}

	engine := wasmtime.NewEngineWithConfig(cfg)

	// Create a linker with WASI functions defined within it
	linker := wasmtime.NewLinker(engine)
	err = linker.DefineWasi()
	if err != nil {
		log.Errorf("Failed to define WASI in linker: %v", err)
	}

	pool := &WasmtimeExecutorPool{
		linker:           linker,
		wasmExecutorPool: newWasmExecutorPool(poolSize, log),
	}

	for i := 0; i < poolSize; i++ {
		pool.instances[i] = executor.NewWasmtimeExecutor(linker, log)
	}

	return pool
}

func (p *WasmtimeExecutorPool) RegisterHostFns(fns map[string]any) error {
	for fnName, fn := range fns {
		err := p.linker.FuncWrap("extern", fnName, fn)
		if err != nil {
			p.log.Error("RegisterHostFns failed", "fns", fmt.Sprintf("%+v", fns), "error", err)
			return fmt.Errorf("failed to register host functions: %v", err)
		}
	}
	return nil
}

func (p *WasmtimeExecutorPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.linker.Close()
}
