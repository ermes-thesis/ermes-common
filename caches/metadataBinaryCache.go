package caches

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/ermes-thesis/ermes-common/logger"
	"github.com/ermes-thesis/ermes-common/model"
)

// MetadataBinaryCache stores compiled binaries on disk and keeps track of them
// using an underlying BaseCache. Evicted binaries are automatically removed
// from disk.
type MetadataBinaryCache struct {
	cache  *BaseCache[model.FunctionMetadata]
	binDir string
	log    logger.Logger
	pool   sync.Pool
}
type FunctionBundle struct {
	Binary    *ReusableBuffer
	Metadata  model.FunctionMetadata
	Templates map[string]model.QueryTemplate
}

func (f FunctionBundle) String() string {
	displayBytes := f.Binary.Buf
	truncated := false
	if len(displayBytes) > 30 {
		displayBytes = displayBytes[:30]
		truncated = true
	}

	binStr := fmt.Sprintf("%v", displayBytes)
	if truncated {
		binStr = fmt.Sprintf("%s...(truncated, total %d bytes)", binStr, len(f.Binary.Buf))
	}

	return fmt.Sprintf("{Binary:%s Metadata:%+v Templates:%+v}", binStr, f.Metadata, f.Templates)
}

type ReusableBuffer struct {
	Buf         []byte
	ReleaseFunc func() `json:"-"`
}

// NewMetadataBinaryCache creates a new MetadataBinaryCache based on the given configuration.
// Evicted binaries are automatically deleted from disk.
//
// Parameters:
//   - config: config.MetadataBinaryCacheConfig containing the cache size, TTL, and binary directory
//
// Returns:
//   - *MetadataBinaryCache: the newly created binary cache
//   - error: non-nil if the underlying BaseCache could not be created
func NewMetadataBinaryCache(config MetadataBinaryCacheConfig, log logger.Logger) (*MetadataBinaryCache, error) {
	err := os.MkdirAll(config.BinDir, os.ModePerm)
	if err != nil {
		return nil, err
	}

	binCache, err := NewBaseCacheWithEviction(config.Size, config.Ttl, func(key string, item CacheItem[model.FunctionMetadata]) {
		_ = os.Remove(filepath.Join(config.BinDir, item.Value.Name+"@"+item.Value.Version))
	})
	if err != nil {
		return nil, err
	}

	return &MetadataBinaryCache{
		cache:  binCache,
		binDir: config.BinDir,
		log:    log,
		pool: sync.Pool{
			New: func() any {
				log.Eval("Creating new buffer in pool")
				return make([]byte, 0, 512*1024) // 512KB initial buffer size
			},
		},
	}, nil
}

// Set stores a binary on disk and records its path in the cache.
//
// Parameters:
//   - functionName: the name of the function associated with the binary
//   - binary: the binary content to store
//
// Returns:
//   - error: non-nil if writing the file to disk fails
func (c *MetadataBinaryCache) Set(functionMetadata model.FunctionMetadata, binary []byte) error {
	functionNameString := functionMetadata.Name + "@" + functionMetadata.Version
	binPath := filepath.Join(c.binDir, functionNameString+".wasm")

	err := os.WriteFile(binPath, binary, 0644)
	if err != nil {
		return err
	}

	c.cache.Set(functionNameString, functionMetadata)
	return nil
}

func (c *MetadataBinaryCache) GetWithBinary(functionName string, functionVersion string) (*ReusableBuffer, *model.FunctionMetadata, bool, error) {
	functionNameString := functionName + "@" + functionVersion
	binPath := filepath.Join(c.binDir, functionNameString+".wasm")
	if fnMetadata, ok := c.cache.Get(functionNameString); ok {
		file, err := os.Open(binPath)
		if err != nil {
			return nil, nil, false, err
		}

		stats, err := file.Stat()
		if err != nil {
			return nil, nil, false, err
		}

		buf := c.pool.Get().([]byte)

		if cap(buf) < int(stats.Size()) {
			buf = make([]byte, stats.Size())
		} else {
			buf = buf[:stats.Size()]
		}

		_, err = io.ReadAtLeast(file, buf, int(stats.Size()))
		if err != nil {
			return nil, nil, false, err
		}
		return &ReusableBuffer{
			Buf: buf,
			ReleaseFunc: func() {
				c.log.Eval("Releasing buffer back to pool")
				c.pool.Put(buf)
			},
		}, &fnMetadata, true, nil
	}
	return nil, nil, false, nil
}

func (c *MetadataBinaryCache) Get(functionName string, functionVersion string) (model.FunctionMetadata, bool) {
	functionNameString := functionName + "@" + functionVersion
	return c.cache.Get(functionNameString)
}
