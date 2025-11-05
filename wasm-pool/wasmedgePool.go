package wasm

import (
	"fmt"
	"github.com/ermes-thesis/ermes-common/logger"
	"github.com/ermes-thesis/ermes-common/wasm-pool/executor"

	"github.com/second-state/WasmEdge-go/wasmedge"
)

type WasmedgeExecutorPool struct {
	*wasmExecutorPool
}

func NewWasmedgeExecutorPool(poolSize int, log logger.Logger) *WasmedgeExecutorPool {
	pool := &WasmedgeExecutorPool{
		wasmExecutorPool: newWasmExecutorPool(poolSize, log),
	}
	wasmedge.SetLogErrorLevel()
	conf := wasmedge.NewConfigure(wasmedge.WASI)
	for i := 0; i < poolSize; i++ {
		pool.instances[i] = executor.NewWasmedgeExecutor(conf, log)
	}
	conf.Release()
	return pool
}

func (p *WasmedgeExecutorPool) RegisterHostFns(fns map[string]any) error {
	for _, exec := range p.instances {
		err := exec.RegisterHostFns(fns)
		if err != nil {
			p.log.Error("RegisterHostFns failed", "fns", fmt.Sprintf("%+v", fns), "error", err)
			return fmt.Errorf("failed to register host functions: %v", err)
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
