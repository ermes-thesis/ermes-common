package test

import (
	"fmt"
	"os"
	"testing"

	"github.com/ermes-thesis/ermes-common/wasm-pool"
)

func TestWasmPool(t *testing.T) {
	pool := wasm.NewWasmedgeExecutorPool(10)
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

func BenchmarkWasmPool(b *testing.B) {
	pool := wasm.NewWasmedgeExecutorPool(10)
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
