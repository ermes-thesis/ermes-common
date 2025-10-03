package test

import (
	"fmt"
	"testing"

	"github.com/ermes-thesis/ermes-wasm-pool/wasm-pool"
)

func TestWasmPool(t *testing.T) {
	pool := wasm.NewWasmedgeExecutorPool(10)
	defer pool.Close()

	executor := pool.Acquire()
	defer executor.Release()

	resp := executor.Invoke("auth_function", "authEntry", []byte("token"))
	fmt.Println("Auth function:", resp.Response, resp.HttpCode)

	if resp.HttpCode != 200 {
		t.Fatalf("invoke function failed: %s", resp.Response)
	}
}

func BenchmarkWasmPool(b *testing.B) {
	pool := wasm.NewWasmedgeExecutorPool(10)
	defer pool.Close()

	for b.Loop() {
		executor := pool.Acquire()

		resp := executor.Invoke("auth_function", "authEntry", []byte("token"))
		fmt.Println("Auth function:", resp.Response, resp.HttpCode)

		if resp.HttpCode != 200 {
			b.Fatalf("invoke function failed: %s", resp.Response)
		}
		executor.Release()
	}
}
