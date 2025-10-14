package wasm

import (
	"log"
	"sync"

	"github.com/tetratelabs/wazero"

	"github.com/ermes-thesis/ermes-common/wasm-pool/executor"

	"github.com/second-state/WasmEdge-go/wasmedge"
)

type ExecutorPool interface {
	RegisterHostFns(fns map[string]any) error
	Acquire() executor.Executor
	Close()
}

type WasmExecutorPool struct {
	instances []executor.Executor
	mu        sync.Mutex
}

func NewWasmedgeExecutorPool(poolSize int) *WasmExecutorPool {
	pool := &WasmExecutorPool{}
	wasmedge.SetLogErrorLevel()
	conf := wasmedge.NewConfigure(wasmedge.WASI)
	for i := 0; i < poolSize; i++ {
		pool.instances = append(pool.instances, executor.NewWasmedgeExecutor(conf))
	}
	conf.Release()
	return pool
}

func NewWazeroExecutorPool(poolSize int, interpreter bool) *WasmExecutorPool {
	pool := &WasmExecutorPool{}

	var config wazero.RuntimeConfig
	if interpreter {
		config = wazero.NewRuntimeConfigInterpreter()
	} else {
		config = wazero.NewRuntimeConfig().WithCompilationCache(wazero.NewCompilationCache())
	}
	for i := 0; i < poolSize; i++ {
		pool.instances = append(pool.instances, executor.NewWazeroExecutor(config))
	}
	return pool
}

func (p *WasmExecutorPool) RegisterHostFns(fns map[string]any) error {
	for _, exec := range p.instances {
		err := exec.RegisterHostFns(fns)
		if err != nil {
			log.Fatalf("Failed to register module: %v", err)
			return err
		}
	}
	return nil
}

func (p *WasmExecutorPool) Acquire() executor.Executor {
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

func (p *WasmExecutorPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, exec := range p.instances {
		exec.Close()
	}
}
