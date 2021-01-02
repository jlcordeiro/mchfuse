package mch

import (
	"sync"
)

const (
	// considering each write to cache will have at most 128k it makes it
	// considerably easier to use this as a soft limit. meaning each cache
	// may actually contain up to CACHE_SIZE_SOFT_CAP + fuse.MAX_KERNEL_WRITE
	CACHE_SIZE_SOFT_CAP = 2 * 1024 * 1024
)

type CacheFile struct {
	mu      sync.Mutex
	content []byte
	// offset represents what offset (in the source file) the cached
	// content should be written into
	offset int64
}

func (cf *CacheFile) Reset() {
	cf.content = nil
	cf.offset = 0
}

func (cf *CacheFile) Add(data []byte, offset int64) {
	cf.mu.Lock()
	defer cf.mu.Unlock()

	// if this is the first time we are writing to cache, let's keep track
	// of this chunk's offset as we'll need it later to call the api
	if cf.content == nil {
		cf.offset = offset
	}

	cf.content = append(cf.content, data...)
}

func (cf *CacheFile) Length() int {
	cf.mu.Lock()
	defer cf.mu.Unlock()

	return len(cf.content)
}

func (cf *CacheFile) Full() bool {
	cf.mu.Lock()
	defer cf.mu.Unlock()
	return len(cf.content) >= CACHE_SIZE_SOFT_CAP
}

func (cf *CacheFile) Offset() int64 {
	cf.mu.Lock()
	defer cf.mu.Unlock()
	return cf.offset
}

func (cf *CacheFile) Content() []byte {
	cf.mu.Lock()
	defer cf.mu.Unlock()

	return cf.content
}
