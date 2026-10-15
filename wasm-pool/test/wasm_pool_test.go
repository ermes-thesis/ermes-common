package test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ermes-thesis/ermes-common/logger"
	"github.com/ermes-thesis/ermes-common/wasm-pool"
	"github.com/stretchr/testify/require"
)

func createNewLogger(t testing.TB) logger.Logger {
	log, err := logger.NewLogger(logger.Config{
		Level:     logger.EvalLevel,
		Format:    "text",
		AddSource: false,
		Muted:     true,
	})
	require.NoError(t, err)
	return log
}

func createNewTestConfig(t *testing.T) wasm.WASPConfig {

	return wasm.WASPConfig{
		PoolSize:  10,
		WithCache: wasm.CacheNone,
		RunMode:   wasm.Dynamic,
		CacheSize: 2,
		Timeout:   5 * time.Second,
		Runtime:   0,
	}
}

func createNewBenchmarkConfig(b *testing.B) wasm.WASPConfig {

	return wasm.WASPConfig{
		PoolSize:  10,
		WithCache: wasm.CacheNone,
		RunMode:   wasm.Dynamic,
		CacheSize: 2,
		Timeout:   5 * time.Second,
		Runtime:   0,
	}
}

func TestWasmedgePool(t *testing.T) {
	pool := wasm.NewWasmedgeExecutorPool(createNewTestConfig(t), createNewLogger(t))
	defer pool.Close()

	wasmBuffer, err := os.ReadFile("auth_function.wasm")
	if err != nil {
		t.Fatalf("failed to read wasm file: %v", err)
	}

	resp := pool.InvokeFunction("fun", wasmBuffer, "authEntry", []byte("token"))
	fmt.Println("Auth function:", resp.Response, resp.HttpCode)

	if resp.HttpCode != 200 {
		t.Fatalf("invoke function failed: %s", resp.Response)
	}
}

func TestWasmtimePool(t *testing.T) {
	pool := wasm.NewWasmtimeExecutorPool(createNewTestConfig(t), createNewLogger(t))
	defer pool.Close()

	wasmBuffer, err := os.ReadFile("auth_function.wasm")
	if err != nil {
		t.Fatalf("failed to read wasm file: %v", err)
	}

	resp := pool.InvokeFunction("fun", wasmBuffer, "authEntry", []byte("token"))
	fmt.Println("Auth function:", resp.Response, resp.HttpCode)

	if resp.HttpCode != 200 {
		t.Fatalf("invoke function failed: %s", resp.Response)
	}
}

func TestWazeroPool(t *testing.T) {
	pool := wasm.NewWazeroExecutorPool(createNewTestConfig(t), context.Background(), createNewLogger(t))
	defer pool.Close()

	wasmBuffer, err := os.ReadFile("auth_function.wasm")
	if err != nil {
		t.Fatalf("failed to read wasm file: %v", err)
	}

	resp := pool.InvokeFunction("fun", wasmBuffer, "authEntry", []byte("token"))
	fmt.Println("Auth function:", resp.Response, resp.HttpCode)

	if resp.HttpCode != 200 {
		t.Fatalf("invoke function failed: %s", resp.Response)
	}
}

func TestWasmedgePoolConcurrent(t *testing.T) {
	pool := wasm.NewWasmedgeExecutorPool(wasm.WASPConfig{
		PoolSize:  20,
		WithCache: wasm.CacheNone,
		RunMode:   wasm.Dynamic,
		CacheSize: 2,
	}, createNewLogger(t))
	defer pool.Close()

	runConcurrentTest(t, pool)
}

func TestWasmedgeWithStandardCachePoolConcurrent(t *testing.T) {
	pool := wasm.NewWasmedgeExecutorPool(wasm.WASPConfig{
		PoolSize:  20,
		WithCache: wasm.CacheStandard,
		RunMode:   wasm.Dynamic,
		CacheSize: 2,
	}, createNewLogger(t))
	defer pool.Close()

	runConcurrentTest(t, pool)
}

func TestWasmedgeWithHybridCachePoolConcurrent(t *testing.T) {
	pool := wasm.NewWasmedgeExecutorPool(wasm.WASPConfig{
		PoolSize:  20,
		WithCache: wasm.CacheHybrid,
		RunMode:   wasm.Dynamic,
		CacheSize: 2,
	}, createNewLogger(t))
	defer pool.Close()

	runConcurrentTest(t, pool)
}

func TestWasmtimePoolConcurrent(t *testing.T) {
	pool := wasm.NewWasmtimeExecutorPool(wasm.WASPConfig{
		PoolSize:  20,
		WithCache: wasm.CacheNone,
		RunMode:   wasm.Dynamic,
		CacheSize: 2,
	}, createNewLogger(t))
	defer pool.Close()

	runConcurrentTest(t, pool)
}

func TestWasmtimeWithStandardCachePoolConcurrent(t *testing.T) {
	pool := wasm.NewWasmtimeExecutorPool(wasm.WASPConfig{
		PoolSize:  20,
		WithCache: wasm.CacheStandard,
		RunMode:   wasm.Dynamic,
		CacheSize: 2,
	}, createNewLogger(t))
	defer pool.Close()

	runConcurrentTest(t, pool)
}

func TestWasmtimeWithHybridCachePoolConcurrent(t *testing.T) {
	pool := wasm.NewWasmtimeExecutorPool(wasm.WASPConfig{
		PoolSize:  20,
		WithCache: wasm.CacheHybrid,
		RunMode:   wasm.Dynamic,
		CacheSize: 2,
	}, createNewLogger(t))
	defer pool.Close()

	runConcurrentTest(t, pool)
}

func TestWazeroPoolConcurrent(t *testing.T) {
	pool := wasm.NewWazeroExecutorPool(wasm.WASPConfig{
		PoolSize:  20,
		WithCache: wasm.CacheNone,
		RunMode:   wasm.AOT,
		CacheSize: 2,
	}, context.Background(), createNewLogger(t))
	defer pool.Close()

	runConcurrentTest(t, pool)
}

func TestWazeroWithStandardCachePoolConcurrent(t *testing.T) {
	pool := wasm.NewWazeroExecutorPool(wasm.WASPConfig{
		PoolSize:  20,
		WithCache: wasm.CacheStandard,
		RunMode:   wasm.AOT,
		CacheSize: 2,
	}, context.Background(), createNewLogger(t))
	defer pool.Close()

	runConcurrentTest(t, pool)
}

func TestWazeroWithHybridCachePoolConcurrent(t *testing.T) {
	pool := wasm.NewWazeroExecutorPool(wasm.WASPConfig{
		PoolSize:  20,
		WithCache: wasm.CacheHybrid,
		RunMode:   wasm.AOT,
		CacheSize: 2,
	}, context.Background(), createNewLogger(t))
	defer pool.Close()

	runConcurrentTest(t, pool)
}

func runConcurrentTest(t *testing.T, pool wasm.WASP) {
	wasmBuffer, err := os.ReadFile("auth_function.wasm")
	if err != nil {
		t.Fatalf("failed to read wasm file: %v", err)
	}

	wg := sync.WaitGroup{}
	wg.Add(600)
	for i := 0; i < 200; i++ {
		go func() {
			defer wg.Done()

			resp := pool.InvokeFunction("fun", wasmBuffer, "authEntry", []byte("token"))
			fmt.Println("Auth function:", resp.Response, resp.HttpCode)

			if resp.HttpCode != 200 {
				t.Errorf("invoke function failed: %s", resp.Response)
				return
			}
		}()
		go func() {
			defer wg.Done()

			resp := pool.InvokeFunction("fun2", wasmBuffer, "authEntry", []byte("token"))
			fmt.Println("Auth function:", resp.Response, resp.HttpCode)

			if resp.HttpCode != 200 {
				t.Errorf("invoke function failed: %s", resp.Response)
				return
			}
		}()
		go func() {
			defer wg.Done()

			resp := pool.InvokeFunction("fun3", wasmBuffer, "authEntry", []byte("token"))
			fmt.Println("Auth function:", resp.Response, resp.HttpCode)

			if resp.HttpCode != 200 {
				t.Errorf("invoke function failed: %s", resp.Response)
				return
			}
		}()
	}
	wg.Wait()

}

func BenchmarkWasmedgePool(b *testing.B) {
	pool := wasm.NewWasmedgeExecutorPool(createNewBenchmarkConfig(b), createNewLogger(b))
	defer pool.Close()

	wasmBuffer, err := os.ReadFile("auth_function.wasm")
	if err != nil {
		b.Fatalf("failed to read wasm file: %v", err)
	}

	for b.Loop() {

		resp := pool.InvokeFunction("fun", wasmBuffer, "authEntry", []byte("token"))
		fmt.Println("Auth function:", resp.Response, resp.HttpCode)

		if resp.HttpCode != 200 {
			b.Fatalf("invoke function failed: %s", resp.Response)
		}
	}
}

func BenchmarkWasmtimePool(b *testing.B) {
	pool := wasm.NewWasmtimeExecutorPool(createNewBenchmarkConfig(b), createNewLogger(b))
	defer pool.Close()

	wasmBuffer, err := os.ReadFile("auth_function.wasm")
	if err != nil {
		b.Fatalf("failed to read wasm file: %v", err)
	}

	for b.Loop() {

		resp := pool.InvokeFunction("fun", wasmBuffer, "authEntry", []byte("token"))
		fmt.Println("Auth function:", resp.Response, resp.HttpCode)

		if resp.HttpCode != 200 {
			b.Fatalf("invoke function failed: %s", resp.Response)
		}
	}
}

func BenchmarkWazeroPool(b *testing.B) {
	pool := wasm.NewWazeroExecutorPool(createNewBenchmarkConfig(b), context.Background(), createNewLogger(b))
	defer pool.Close()

	wasmBuffer, err := os.ReadFile("auth_function.wasm")
	if err != nil {
		b.Fatalf("failed to read wasm file: %v", err)
	}

	for b.Loop() {

		resp := pool.InvokeFunction("fun", wasmBuffer, "authEntry", []byte("token"))
		fmt.Println("Auth function:", resp.Response, resp.HttpCode)

		if resp.HttpCode != 200 {
			b.Fatalf("invoke function failed: %s", resp.Response)
		}
	}
}
