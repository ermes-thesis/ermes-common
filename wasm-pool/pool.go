package wasm

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ermes-thesis/ermes-common/model"
)

type ExecutorPool interface {
	RegisterHostFns(fns map[string]any) error
	Invoke(fnName string, fnBuffer []byte, entrypoint string, input ...[]byte) model.WasmResponse
	Close()
}

type RefCountedItem[T any] struct {
	item        T
	count       atomic.Int64
	releaseFunc func(T) error
}

func NewRefCountedItem[T any](item T, releaseFunc func(T) error) *RefCountedItem[T] {
	r := &RefCountedItem[T]{
		item:        item,
		releaseFunc: releaseFunc,
	}
	r.count.Store(1)
	return r
}

func (r *RefCountedItem[T]) Acquire() bool {
	for {
		current := r.count.Load()

		if current <= 0 {
			return false
		}

		if r.count.CompareAndSwap(current, current+1) {
			return true
		}
	}
}

func (r *RefCountedItem[T]) Release() error {
	newVal := r.count.Add(-1)

	if newVal == 0 {
		if r.releaseFunc != nil {
			return r.releaseFunc(r.item)
		}
	}

	if newVal < 0 {
		panic("This code should be unreachable, there are concurrency problems with the RefCountedItems")
	}
	return nil
}

func buildErrorResponse(err error) model.WasmResponse {
	return model.WasmResponse{
		HttpCode: 500,
		Response: fmt.Sprintf("wasm execution error: %v", err),
	}
}

func getTimeoutChannel(timeout time.Duration) <-chan time.Time {
	var timeoutChan <-chan time.Time

	if timeout > 0 {
		timeoutChan = time.After(timeout)
	}
	return timeoutChan
}
