package wasm

import (
	"log"

	"github.com/ermes-thesis/ermes-common/wasm-pool/executor"

	"github.com/second-state/WasmEdge-go/wasmedge"
)

type WasmedgeExecutorPool struct {
	*wasmExecutorPool
}

func NewWasmedgeExecutorPool(poolSize int) *WasmedgeExecutorPool {
	pool := &WasmedgeExecutorPool{
		wasmExecutorPool: newWasmExecutorPool(poolSize),
	}
	wasmedge.SetLogErrorLevel()
	conf := wasmedge.NewConfigure(wasmedge.WASI)
	for i := 0; i < poolSize; i++ {
		pool.instances[i] = executor.NewWasmedgeExecutor(conf)
	}
	conf.Release()
	return pool
}

func (p *WasmedgeExecutorPool) RegisterHostFns(fns map[string]any) error {
	for _, exec := range p.instances {
		err := exec.RegisterHostFns(fns)
		if err != nil {
			log.Fatalf("Failed to register module: %v", err)
			return err
		}
	}
	return nil
}

func (p *WasmedgeExecutorPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, exec := range p.instances {
		exec.Close()
	}
}
