package wasm

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	PoolSize  int           `yaml:"poolSize"`
	Timeout   time.Duration `yaml:"timeout"`
	WithCache bool          `yaml:"withCache"`
	CacheSize int           `yaml:"cacheSize"`
	IsAot     bool          `yaml:"isAot"`
}

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
