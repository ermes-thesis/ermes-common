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

var cfg = wasm.Config{
	PoolSize:  10,
	WithCache: false,
	IsAot:     false,
	Timeout:   5 * time.Second,
}

func TestWasmedgePool(t *testing.T) {
	pool := wasm.NewWasmedgeExecutorPool(cfg, createNewLogger(t))
	defer pool.Close()

	wasmBuffer, err := os.ReadFile("auth_function.wasm")
	if err != nil {
		t.Fatalf("failed to read wasm file: %v", err)
	}

	resp := pool.Invoke(wasmBuffer, "authEntry", []byte("token"))
	fmt.Println("Auth function:", resp.Response, resp.HttpCode)

	if resp.HttpCode != 200 {
		t.Fatalf("invoke function failed: %s", resp.Response)
	}
}

func TestWasmtimePool(t *testing.T) {
	pool := wasm.NewWasmtimeExecutorPool(cfg, createNewLogger(t))
	defer pool.Close()

	wasmBuffer, err := os.ReadFile("auth_function.wasm")
	if err != nil {
		t.Fatalf("failed to read wasm file: %v", err)
	}

	resp := pool.Invoke(wasmBuffer, "authEntry", []byte("token"))
	fmt.Println("Auth function:", resp.Response, resp.HttpCode)

	if resp.HttpCode != 200 {
		t.Fatalf("invoke function failed: %s", resp.Response)
	}
}

func TestWazeroPool(t *testing.T) {
	pool := wasm.NewWazeroExecutorPool(cfg, context.Background(), createNewLogger(t))
	defer pool.Close()

	wasmBuffer, err := os.ReadFile("auth_function.wasm")
	if err != nil {
		t.Fatalf("failed to read wasm file: %v", err)
	}

	resp := pool.Invoke(wasmBuffer, "authEntry", []byte("token"))
	fmt.Println("Auth function:", resp.Response, resp.HttpCode)

	if resp.HttpCode != 200 {
		t.Fatalf("invoke function failed: %s", resp.Response)
	}
}

func TestWasmedgePoolConcurrent(t *testing.T) {
	pool := wasm.NewWasmedgeExecutorPool(wasm.Config{
		PoolSize: 20,
		Timeout:  2 * time.Minute,
	}, createNewLogger(t))
	defer pool.Close()

	runConcurrentTest(t, pool)
}

func TestWasmtimePoolConcurrent(t *testing.T) {
	pool := wasm.NewWasmtimeExecutorPool(wasm.Config{
		PoolSize:  20,
		WithCache: false,
		IsAot:     false,
		Timeout:   2 * time.Minute,
	}, createNewLogger(t))
	defer pool.Close()

	runConcurrentTest(t, pool)
}

func TestWasmtimeWithCachePoolConcurrent(t *testing.T) {
	pool := wasm.NewWasmtimeExecutorPool(wasm.Config{
		PoolSize:  20,
		WithCache: true,
		IsAot:     false,
		Timeout:   2 * time.Minute,
	}, createNewLogger(t))
	defer pool.Close()

	runConcurrentTest(t, pool)
}

func TestWazeroPoolConcurrent(t *testing.T) {
	pool := wasm.NewWazeroExecutorPool(wasm.Config{
		PoolSize:  20,
		WithCache: false,
		IsAot:     true,
		Timeout:   2 * time.Minute,
	}, context.Background(), createNewLogger(t))
	defer pool.Close()

	runConcurrentTest(t, pool)
}

func TestWazeroWithCachePoolConcurrent(t *testing.T) {
	pool := wasm.NewWazeroExecutorPool(wasm.Config{
		PoolSize:  20,
		WithCache: true,
		IsAot:     true,
		Timeout:   2 * time.Minute,
	}, context.Background(), createNewLogger(t))
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

			resp := pool.Invoke(wasmBuffer, "authEntry", []byte("token"))
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
	pool := wasm.NewWasmedgeExecutorPool(cfg, createNewLogger(b))
	defer pool.Close()

	wasmBuffer, err := os.ReadFile("auth_function.wasm")
	if err != nil {
		b.Fatalf("failed to read wasm file: %v", err)
	}

	for b.Loop() {

		resp := pool.Invoke(wasmBuffer, "authEntry", []byte("token"))
		fmt.Println("Auth function:", resp.Response, resp.HttpCode)

		if resp.HttpCode != 200 {
			b.Fatalf("invoke function failed: %s", resp.Response)
		}
	}
}

func BenchmarkWasmtimePool(b *testing.B) {
	pool := wasm.NewWasmtimeExecutorPool(cfg, createNewLogger(b))
	defer pool.Close()

	wasmBuffer, err := os.ReadFile("auth_function.wasm")
	if err != nil {
		b.Fatalf("failed to read wasm file: %v", err)
	}

	for b.Loop() {

		resp := pool.Invoke(wasmBuffer, "authEntry", []byte("token"))
		fmt.Println("Auth function:", resp.Response, resp.HttpCode)

		if resp.HttpCode != 200 {
			b.Fatalf("invoke function failed: %s", resp.Response)
		}
	}
}

func BenchmarkWazeroPool(b *testing.B) {
	pool := wasm.NewWazeroExecutorPool(cfg, context.Background(), createNewLogger(b))
	defer pool.Close()

	wasmBuffer, err := os.ReadFile("auth_function.wasm")
	if err != nil {
		b.Fatalf("failed to read wasm file: %v", err)
	}

	for b.Loop() {

		resp := pool.Invoke(wasmBuffer, "authEntry", []byte("token"))
		fmt.Println("Auth function:", resp.Response, resp.HttpCode)

		if resp.HttpCode != 200 {
			b.Fatalf("invoke function failed: %s", resp.Response)
		}
	}
}
