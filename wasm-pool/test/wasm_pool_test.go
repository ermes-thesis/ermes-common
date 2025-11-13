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
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func createNewLogger(t testing.TB) logger.Logger {
	log, err := logger.NewLogger(logger.Config{
		Level:     zerolog.InfoLevel,
		Format:    "text",
		AddSource: false,
		Muted:     true,
	})
	require.NoError(t, err)
	return log
}

func TestWasmedgePool(t *testing.T) {
	pool := wasm.NewWasmedgeExecutorPool(10, createNewLogger(t))
	defer pool.Close()

	wasmBuffer, err := os.ReadFile("auth_function.wasm")
	if err != nil {
		t.Fatalf("failed to read wasm file: %v", err)
	}

	executor := pool.Acquire()
	defer executor.Release()

	resp := executor.Invoke(wasmBuffer, "authEntry", []byte("token"))
	fmt.Println("Auth function:", resp.Response, resp.HttpCode)

	if resp.HttpCode != 200 {
		t.Fatalf("invoke function failed: %s", resp.Response)
	}
}

func TestWasmtimePool(t *testing.T) {
	pool := wasm.NewWasmtimeExecutorPool(10, false, false, createNewLogger(t))
	defer pool.Close()

	wasmBuffer, err := os.ReadFile("auth_function.wasm")
	if err != nil {
		t.Fatalf("failed to read wasm file: %v", err)
	}

	executor := pool.Acquire()
	defer executor.Release()

	resp := executor.Invoke(wasmBuffer, "authEntry", []byte("token"))
	fmt.Println("Auth function:", resp.Response, resp.HttpCode)

	if resp.HttpCode != 200 {
		t.Fatalf("invoke function failed: %s", resp.Response)
	}
}

func TestWazeroPool(t *testing.T) {
	pool := wasm.NewWazeroExecutorPool(10, false, false, context.Background(), createNewLogger(t))
	defer pool.Close()

	wasmBuffer, err := os.ReadFile("auth_function.wasm")
	if err != nil {
		t.Fatalf("failed to read wasm file: %v", err)
	}

	executor := pool.Acquire()
	defer executor.Release()

	resp := executor.Invoke(wasmBuffer, "authEntry", []byte("token"))
	fmt.Println("Auth function:", resp.Response, resp.HttpCode)

	if resp.HttpCode != 200 {
		t.Fatalf("invoke function failed: %s", resp.Response)
	}
}

func TestWasmedgePoolConcurrent(t *testing.T) {
	pool := wasm.NewWasmedgeExecutorPool(20, createNewLogger(t))
	defer pool.Close()

	runConcurrentTest(t, pool)
}

func TestWasmtimePoolConcurrent(t *testing.T) {
	pool := wasm.NewWasmtimeExecutorPool(20, false, false, createNewLogger(t))
	defer pool.Close()

	runConcurrentTest(t, pool)
}

func TestWasmtimeWithCachePoolConcurrent(t *testing.T) {
	pool := wasm.NewWasmtimeExecutorPool(20, true, false, createNewLogger(t))
	defer pool.Close()

	runConcurrentTest(t, pool)
}

func TestWazeroPoolConcurrent(t *testing.T) {
	pool := wasm.NewWazeroExecutorPool(20, false, false, context.Background(), createNewLogger(t))
	defer pool.Close()

	runConcurrentTest(t, pool)
}

func TestWazeroWithCachePoolConcurrent(t *testing.T) {
	pool := wasm.NewWazeroExecutorPool(20, false, true, context.Background(), createNewLogger(t))
	defer pool.Close()

	runConcurrentTest(t, pool)
}

func runConcurrentTest(t *testing.T, pool wasm.ExecutorPool) {
	wasmBuffer, err := os.ReadFile("auth_function.wasm")
	if err != nil {
		t.Fatalf("failed to read wasm file: %v", err)
	}

	wg := sync.WaitGroup{}
	wg.Add(500)
	for i := 0; i < 500; i++ {
		go func() {
			defer wg.Done()
			executor := pool.Acquire()
			for executor == nil {
				time.Sleep(time.Millisecond * 10)
				executor = pool.Acquire()
			}
			defer executor.Release()

			resp := executor.Invoke(wasmBuffer, "authEntry", []byte("token"))
			fmt.Println("Auth function:", resp.Response, resp.HttpCode)

			if resp.HttpCode != 200 {
				t.Fatalf("invoke function failed: %s", resp.Response)
			}
		}()
	}
	wg.Wait()

}
func BenchmarkWasmedgePool(b *testing.B) {
	pool := wasm.NewWasmedgeExecutorPool(10, createNewLogger(b))
	defer pool.Close()

	wasmBuffer, err := os.ReadFile("auth_function.wasm")
	if err != nil {
		b.Fatalf("failed to read wasm file: %v", err)
	}

	for b.Loop() {
		executor := pool.Acquire()

		resp := executor.Invoke(wasmBuffer, "authEntry", []byte("token"))
		fmt.Println("Auth function:", resp.Response, resp.HttpCode)

		if resp.HttpCode != 200 {
			b.Fatalf("invoke function failed: %s", resp.Response)
		}
		executor.Release()
	}
}

func BenchmarkWasmtimePool(b *testing.B) {
	pool := wasm.NewWasmtimeExecutorPool(10, false, false, createNewLogger(b))
	defer pool.Close()

	wasmBuffer, err := os.ReadFile("auth_function.wasm")
	if err != nil {
		b.Fatalf("failed to read wasm file: %v", err)
	}

	for b.Loop() {
		executor := pool.Acquire()

		resp := executor.Invoke(wasmBuffer, "authEntry", []byte("token"))
		fmt.Println("Auth function:", resp.Response, resp.HttpCode)

		if resp.HttpCode != 200 {
			b.Fatalf("invoke function failed: %s", resp.Response)
		}
		executor.Release()
	}
}

func BenchmarkWazeroPool(b *testing.B) {
	pool := wasm.NewWazeroExecutorPool(10, false, false, context.Background(), createNewLogger(b))
	defer pool.Close()

	wasmBuffer, err := os.ReadFile("auth_function.wasm")
	if err != nil {
		b.Fatalf("failed to read wasm file: %v", err)
	}

	for b.Loop() {
		executor := pool.Acquire()

		resp := executor.Invoke(wasmBuffer, "authEntry", []byte("token"))
		fmt.Println("Auth function:", resp.Response, resp.HttpCode)

		if resp.HttpCode != 200 {
			b.Fatalf("invoke function failed: %s", resp.Response)
		}
		executor.Release()
	}
}
