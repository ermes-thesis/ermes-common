package wasm

import (
	"log"

	"github.com/bytecodealliance/wasmtime-go/v37"
	"github.com/ermes-thesis/ermes-common/wasm-pool/executor"
)

type WasmtimeExecutorPool struct {
	*wasmExecutorPool
	linker *wasmtime.Linker
}

func NewWasmtimeExecutorPool(poolSize int) *WasmtimeExecutorPool {
	cfg := wasmtime.NewConfig()
	err := cfg.CacheConfigLoadDefault()
	if err != nil {
		log.Fatal(err)
	}

	engine := wasmtime.NewEngineWithConfig(cfg)

	// Create a linker with WASI functions defined within it
	linker := wasmtime.NewLinker(engine)
	err = linker.DefineWasi()
	if err != nil {
		log.Fatal(err)
	}

	pool := &WasmtimeExecutorPool{
		linker:           linker,
		wasmExecutorPool: newWasmExecutorPool(poolSize),
	}

	for i := 0; i < poolSize; i++ {
		pool.instances[i] = executor.NewWasmtimeExecutor(linker)
	}

	return pool
}

func (p *WasmtimeExecutorPool) RegisterHostFns(fns map[string]any) error {
	for fnName, fn := range fns {
		err := p.linker.FuncWrap("extern", fnName, fn)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *WasmtimeExecutorPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.linker.Close()
}
