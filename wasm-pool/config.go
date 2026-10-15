package wasm

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type FunctionCacheConfig struct {
	Size        int           `yaml:"size"`
	Ttl         time.Duration `yaml:"ttl"`
	RegistryDir string        `yaml:"registryDir"`
}

type WASPConfig struct {
	PoolSize  int               `yaml:"poolSize"`
	Timeout   time.Duration     `yaml:"timeout"`
	WithCache CacheType         `yaml:"withCache"`
	CacheSize int               `yaml:"cacheSize"`
	RunMode   RunMode           `yaml:"runMode"`
	Runtime   ChosenWasmRuntime `yaml:"runtime"`
}

type RunMode int

const (
	AOT RunMode = iota
	Dynamic
)

func (m *RunMode) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	switch strings.ToLower(s) {
	case "aot":
		*m = AOT
	case "dynamic":
		*m = Dynamic
	default:
		return fmt.Errorf("unknown run mode: %s", s)
	}

	return nil
}

type CacheType int

const (
	CacheNone     CacheType = iota
	CacheStandard           // Cache del runtime
	CacheHybrid             // Cache del runtime + our custom cache
)

type ChosenWasmRuntime int

const (
	Wasmedge ChosenWasmRuntime = iota
	Wasmtime
	Wazero
)

func (m *ChosenWasmRuntime) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	switch strings.ToLower(s) {
	case "wasmedge":
		*m = Wasmedge
	case "wasmtime":
		*m = Wasmtime
	case "wazero":
		*m = Wazero
	default:
		return fmt.Errorf("unknown chosen wasm runtime: %s", s)
	}

	return nil
}

func (m ChosenWasmRuntime) String() string {
	switch m {
	case Wasmedge:
		return "wasmedge"
	case Wasmtime:
		return "wasmtime"
	case Wazero:
		return "wazero"
	default:
		return "unknown"
	}
}
