package wasm

import (
	"log"
	"os"
	"sync"

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
}

func newWasmExecutorPool(poolSize int) *wasmExecutorPool {
	// optional: create sandbox dir
	if err := os.MkdirAll("./sandbox", os.ModePerm); err != nil {
		log.Fatalf("Failed to create sandbox directory: %v", err)
	}

	return &wasmExecutorPool{
		instances: make([]executor.Executor, poolSize),
	}
}

func (p *wasmExecutorPool) Acquire() executor.Executor {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, exec := range p.instances {
		if !exec.IsInUse() {
			//log.Default().Println("Acquired executor N. ", index)
			exec.SetInUse()
			return exec
		}
	}

	return nil
}
