package wasm

import (
	"os"
	"sync"

	"github.com/ermes-thesis/ermes-common/logger"
	"github.com/ermes-thesis/ermes-common/wasm-pool/executor"
)

type ExecutorPool interface {
	RegisterHostFns(fns map[string]any) error
	Acquire() executor.Executor
	Close()
}

type wasmExecutorPool struct {
	ExecutorPool
	instances []executor.Executor
	mu        sync.Mutex
	log       *logger.Logger
}

func newWasmExecutorPool(poolSize int, log *logger.Logger) *wasmExecutorPool {
	// optional: create sandbox dir
	if err := os.MkdirAll("./sandbox", os.ModePerm); err != nil {
		log.Errorf("Failed to create sandbox directory: %v", err)
	}

	return &wasmExecutorPool{
		instances: make([]executor.Executor, poolSize),
		log:       log,
	}
}

func (p *wasmExecutorPool) Acquire() executor.Executor {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, exec := range p.instances {
		if !exec.IsInUse() {
			p.log.Debugf("Acquired executor N. %d", i)
			exec.SetInUse()
			return exec
		}
	}

	p.log.Info("No more executors available. Retry later")
	return nil
}
