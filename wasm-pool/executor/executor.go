package executor

import (
	"github.com/bytecodealliance/wasmtime-go/v37"
	"github.com/second-state/WasmEdge-go/wasmedge"
	"log"
	"os"
)

type Executor interface {
	Invoke(fnName string, entrypoint string, input ...[]byte) WasmResponse
	Release()
	IsInUse() bool
	SetInUse()
	Close()
	RegisterHostFns(fns map[string]any) error
}

type WasmResponse struct {
	HttpCode            int                          `json:"http_code"`
	Response            any                          `json:"response"`
	CollectionsToCommit map[string]map[string]string `json:"collections_to_commit"`
	DeletedRecords      map[string][]string          `json:"deleted_records"`
	DeletedCollections  []string                     `json:"deleted_collections"`
}

func NewWasmedgeExecutor(conf *wasmedge.Configure) *WasmedgeExecutor {
	vm := wasmedge.NewVMWithConfig(conf)
	wasi := vm.GetImportModule(wasmedge.WASI)

	err := os.MkdirAll("./sandbox", os.ModePerm)
	if err != nil {
		log.Fatalf("Failed to create sandbox directory: %v", err)
	}

	wasi.InitWasi(
		nil,
		nil,
		[]string{".:./sandbox"},
	)
	exec := &WasmedgeExecutor{vm: vm, inUse: false}

	return exec
}

func NewWasmtimeExecutor(conf *wasmtime.WasiConfig) *WasmtimeExecutor {
	engine := wasmtime.NewEngine()

	// Create a linker with WASI functions defined within it
	linker := wasmtime.NewLinker(engine)
	err := linker.DefineWasi()
	if err != nil {
		log.Fatal(err)
	}

	// optional: create sandbox dir
	if err := os.MkdirAll("./sandbox", os.ModePerm); err != nil {
		log.Fatalf("Failed to create sandbox directory: %v", err)
	}

	err = conf.PreopenDir("./sandbox", ".", wasmtime.DIR_WRITE|wasmtime.DIR_READ, wasmtime.FILE_WRITE|wasmtime.FILE_READ)
	if err != nil {
		log.Fatal(err)
	}

	store := wasmtime.NewStore(engine)
	store.SetWasi(conf)

	return &WasmtimeExecutor{
		linker: linker,
		store:  store,
		inUse:  false,
	}
}
