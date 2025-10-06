package caches

import "time"

type MetadataBinaryCacheConfig struct {
	Size   int           `yaml:"size"`
	Ttl    time.Duration `yaml:"ttl"`
	BinDir string        `yaml:"binDir"`
}
