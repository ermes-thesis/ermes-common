package wasm

import (
	"fmt"
	"github.com/redis/go-redis/v9"
	"github.com/second-state/WasmEdge-go/wasmedge"
	"log"
	"sync"
)

type WasmExecutor struct {
	vm    *wasmedge.VM
	mu    sync.Mutex
	inUse bool
}

type ExecutorPool struct {
	instances []*WasmExecutor
	mu        sync.Mutex
}

func NewExecutorPool(poolSize int) *ExecutorPool {
	pool := &ExecutorPool{}
	conf := wasmedge.NewConfigure(wasmedge.WASI)
	for i := 0; i < poolSize; i++ {
		wasmedge.SetLogErrorLevel()

		vm := wasmedge.NewVMWithConfig(conf)
		wasi := vm.GetImportModule(wasmedge.WASI)
		wasi.InitWasi(
			nil,
			nil,
			[]string{".:./sandbox"},
		)

		pool.instances = append(pool.instances, &WasmExecutor{vm: vm, inUse: false})
	}
	conf.Release()
	return pool
}

func (p *ExecutorPool) RegisterHostFns(redisClient *redis.Client) {
	for _, executor := range p.instances {
		// Create the module instance with the module name "extern".
		externMod := wasmedge.NewModule("extern")

		fnTwoParamOneReturnType := wasmedge.NewFunctionType(
			[]*wasmedge.ValType{wasmedge.NewValTypeI32(), wasmedge.NewValTypeI32(), wasmedge.NewValTypeI32(), wasmedge.NewValTypeI32()},
			[]*wasmedge.ValType{wasmedge.NewValTypeI32()},
		)
		fnThreeParamOneReturnType := wasmedge.NewFunctionType(
			[]*wasmedge.ValType{wasmedge.NewValTypeI32(), wasmedge.NewValTypeI32(), wasmedge.NewValTypeI32(), wasmedge.NewValTypeI32(), wasmedge.NewValTypeI32(), wasmedge.NewValTypeI32()},
			[]*wasmedge.ValType{wasmedge.NewValTypeI32()},
		)
		fnOneParamOneReturnType := wasmedge.NewFunctionType(
			[]*wasmedge.ValType{wasmedge.NewValTypeI32(), wasmedge.NewValTypeI32()},
			[]*wasmedge.ValType{wasmedge.NewValTypeI32()},
		)
		// Create a function context with the function type and host function body.
		// The third parameter is the pointer to the additional data.
		// Developers should guarantee the life cycle of the data, and it can be
		// `nil` if the external data is not needed.
		// The last parameter can be 0 if developers do not need the cost measuring.
		hostGetFun := wasmedge.NewFunction(fnTwoParamOneReturnType, hostGet, redisClient, 0)
		hostPutFun := wasmedge.NewFunction(fnThreeParamOneReturnType, hostPut, redisClient, 0)
		hostCreateCollectionFun := wasmedge.NewFunction(fnOneParamOneReturnType, hostCreateCollection, redisClient, 0)
		hostDeleteCollectionFun := wasmedge.NewFunction(fnOneParamOneReturnType, hostDeleteCollection, redisClient, 0)
		hostRemoveUserFromGroupFun := wasmedge.NewFunction(fnTwoParamOneReturnType, hostRemoveUserFromGroup, nil, 0)

		// If the function object is not added into a module instance object, it should be released.
		fnTwoParamOneReturnType.Release()
		fnThreeParamOneReturnType.Release()
		fnOneParamOneReturnType.Release()
		externMod.AddFunction("hostGet", hostGetFun)
		externMod.AddFunction("hostPut", hostPutFun)
		externMod.AddFunction("hostCreateCollection", hostCreateCollectionFun)
		externMod.AddFunction("hostDeleteCollection", hostDeleteCollectionFun)
		externMod.AddFunction("hostRemoveUserFromGroup", hostRemoveUserFromGroupFun)

		err := executor.vm.RegisterModule(externMod)
		if err != nil {
			fmt.Println(err)
			return
		}
	}
}

func (p *ExecutorPool) Acquire() *WasmExecutor {
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

func (p *ExecutorPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, executor := range p.instances {
		executor.mu.Lock()
		executor.vm.Release()
		executor.mu.Unlock()
	}
}
