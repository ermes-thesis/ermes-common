package wasm

import (
	"log"
	"os"

	"github.com/bytecodealliance/wasmtime-go/v37"
	"github.com/ermes-thesis/ermes-common/wasm-pool/executor"
)

type WasmtimeExecutorPool struct {
	WasmExecutorPool
	linker *wasmtime.Linker
}

func NewWasmtimeExecutorPool(poolSize int) ExecutorPool {
	engine := wasmtime.NewEngine()

	// Create a linker with WASI functions defined within it
	linker := wasmtime.NewLinker(engine)
	err := linker.DefineWasi()
	if err != nil {
		log.Fatal(err)
	}

	pool := &WasmtimeExecutorPool{
		linker: linker,
	}

	// optional: create sandbox dir
	if err := os.MkdirAll("./sandbox", os.ModePerm); err != nil {
		log.Fatalf("Failed to create sandbox directory: %v", err)
	}

	for i := 0; i < poolSize; i++ {
		pool.instances = append(pool.instances, executor.NewWasmtimeExecutor(linker))
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

func (p *WasmtimeExecutorPool) Acquire() executor.Executor {
	return p.WasmExecutorPool.Acquire()
}

func (p *WasmtimeExecutorPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.linker.Close()
}
