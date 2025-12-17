package caches

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ermes-thesis/ermes-common/logger"
	"github.com/ermes-thesis/ermes-common/model"
	"golang.org/x/sync/singleflight"
)

// MetadataBinaryCache stores compiled binaries on disk and keeps track of them
// using an underlying BaseCache. Evicted binaries are automatically removed
// from disk.
type MetadataBinaryCache struct {
	cache  *BaseCache[model.FunctionMetadata]
	binary *BaseCache[[]byte]
	group  singleflight.Group
	binDir string
	log    logger.Logger
}
type FunctionBundle struct {
	Binary    []byte
	Metadata  model.FunctionMetadata
	Templates map[string]model.QueryTemplate
}

func (f FunctionBundle) String() string {
	displayBytes := f.Binary
	truncated := false
	if len(displayBytes) > 30 {
		displayBytes = displayBytes[:30]
		truncated = true
	}

	binStr := fmt.Sprintf("%v", displayBytes)
	if truncated {
		binStr = fmt.Sprintf("%s...(truncated, total %d bytes)", binStr, len(f.Binary))
	}

	return fmt.Sprintf("{Binary:%s Metadata:%+v Templates:%+v}", binStr, f.Metadata, f.Templates)
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

	binaryCache, err := NewBaseCache[[]byte](config.Size, config.Ttl)
	if err != nil {
		return nil, err
	}

	return &MetadataBinaryCache{
		cache:  binCache,
		binary: binaryCache,
		binDir: config.BinDir,
		log:    log,
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
	c.binary.Set(functionNameString, binary)
	return nil
}

func (c *MetadataBinaryCache) GetWithBinary(functionName string, functionVersion string) ([]byte, *model.FunctionMetadata, bool, error) {
	functionNameString := functionName + "@" + functionVersion
	binPath := filepath.Join(c.binDir, functionNameString+".wasm")
	if fnMetadata, ok := c.cache.Get(functionNameString); ok {

		if binBuf, ok := c.binary.Get(functionNameString); ok {
			return binBuf, &fnMetadata, true, nil
		}

		binBuf, err, _ := c.group.Do(functionNameString, func() (interface{}, error) {
			buff, err := os.ReadFile(binPath)
			if err != nil {
				return nil, err
			}

			c.binary.Set(functionNameString, buff)
			return buff, nil
		})
		if err != nil {
			return nil, nil, false, err
		}

		return binBuf.([]byte), &fnMetadata, true, nil
	}
	return nil, nil, false, nil
}

func (c *MetadataBinaryCache) Get(functionName string, functionVersion string) (model.FunctionMetadata, bool) {
	functionNameString := functionName + "@" + functionVersion
	return c.cache.Get(functionNameString)
}
