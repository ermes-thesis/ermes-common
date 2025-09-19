package wasm

import (
	"log"
	"sync"

	"github.com/second-state/WasmEdge-go/wasmedge"
)

type ExecutorPool interface {
	RegisterModule(module *wasmedge.Module) error
	Acquire() Executor
	Close()
}

type WasmExecutorPool struct {
	instances []*WasmExecutor
	mu        sync.Mutex
}

func NewExecutorPool(poolSize int) *WasmExecutorPool {
	pool := &WasmExecutorPool{}
	wasmedge.SetLogErrorLevel()
	conf := wasmedge.NewConfigure(wasmedge.WASI)
	for i := 0; i < poolSize; i++ {
		pool.instances = append(pool.instances, NewWasmExecutor(conf))
	}
	conf.Release()
	return pool
}

func (p *WasmExecutorPool) RegisterModule(module *wasmedge.Module) error {
	for _, executor := range p.instances {
		err := executor.vm.RegisterModule(module)
		if err != nil {
			log.Fatalf("Failed to register module: %v", err)
			return err
		}
	}
	return nil
}

func (p *WasmExecutorPool) Acquire() Executor {
	p.mu.Lock()
	defer p.mu.Unlock()

	for index, executor := range p.instances {
		if !executor.inUse {
			log.Default().Println("Acquired executor N. ", index)
			executor.inUse = true
			return executor
		}
	}

	return nil
}

func (p *WasmExecutorPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, executor := range p.instances {
		executor.mu.Lock()
		executor.vm.Release()
		executor.mu.Unlock()
	}
}
