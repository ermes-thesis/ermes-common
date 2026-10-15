package wasm

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ermes-thesis/ermes-common/logger"
	"github.com/ermes-thesis/ermes-common/model"
	"golang.org/x/sync/singleflight"
)

type WASP interface {
	InvokeFunction(fnName string, fnBinary []byte, entrypoint string, input ...[]byte) model.WasmResponse
	RegisterHostFunctions(fns map[string]any) error
	Close()
}

type WASPBase struct {
	cfg   WASPConfig
	log   logger.Logger
	group singleflight.Group
}

func NewWASPBase(cfg WASPConfig, log logger.Logger) (*WASPBase, error) {

	return &WASPBase{
		cfg:   cfg,
		log:   log,
		group: singleflight.Group{},
	}, nil
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
