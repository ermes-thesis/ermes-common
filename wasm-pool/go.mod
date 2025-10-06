module github.com/ermes-thesis/ermes-common/wasm-pool

go 1.24.0

toolchain go1.24.4

require (
	github.com/bytecodealliance/wasmtime-go/v37 v37.0.0
	github.com/ermes-thesis/ermes-common/model v0.0.0-00010101000000-000000000000
	github.com/second-state/WasmEdge-go v0.14.0
)

require github.com/ermes-thesis/ermes-common/utils v0.0.0-00010101000000-000000000000 // indirect

replace github.com/ermes-thesis/ermes-common/model => ../model

replace github.com/ermes-thesis/ermes-common/utils => ../utils
