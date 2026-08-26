package jcache

import (
	"errors"
	"regexp"
	"sync"
	"sync/atomic"

	"github.com/golang/groupcache/lru"
	goconf "github.com/nitsugaro/go-conf"
	jenv "github.com/nitsugaro/go-journey/env"
)

const RegexpCacheKey = "regexp"

var ErrInvalidRegexpCache = errors.New("regexp cache instance has an invalid type")
var defaultRegexpMaxEntries atomic.Int64

func init() {
	defaultRegexpMaxEntries.Store(1000)
	goconf.OnLoad(func() {
		defaultRegexpMaxEntries.Store(int64(jenv.GetOptionalJourneyField("cache.regexp.max", 1000)))
	})
}

func DefaultRegexpCacheMaxEntries() int {
	return int(defaultRegexpMaxEntries.Load())
}

// RegexpCache is a bounded, concurrency-safe cache of compiled expressions.
type RegexpCache struct {
	mu    sync.Mutex
	cache *lru.Cache
}

func NewRegexpCache(maxEntries int) *RegexpCache {
	if maxEntries <= 0 {
		maxEntries = 1000
	}
	return &RegexpCache{cache: lru.New(maxEntries)}
}

func (cache *RegexpCache) Get(pattern string) (*regexp.Regexp, error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if value, ok := cache.cache.Get(pattern); ok {
		return value.(*regexp.Regexp), nil
	}
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	cache.cache.Add(pattern, compiled)
	return compiled, nil
}

// EnsureRegexpCache installs the default runtime singleton unless the host
// already registered one.
func EnsureRegexpCache(manager *Manager, maxEntries int) error {
	if _, found := manager.GetCacheInstance(RegexpCacheKey, DefaultInstanceID); found {
		return nil
	}
	return manager.UpdateRuntimeCacheInstance(RegexpCacheKey, DefaultInstanceID, NewRegexpCache(maxEntries), 0)
}

func GetRegexp(manager *Manager, pattern string) (*regexp.Regexp, error) {
	if manager == nil {
		return regexp.Compile(pattern)
	}
	value, found := manager.GetCacheInstance(RegexpCacheKey, DefaultInstanceID)
	if !found {
		return regexp.Compile(pattern)
	}
	cache, ok := value.(*RegexpCache)
	if !ok {
		return nil, ErrInvalidRegexpCache
	}
	return cache.Get(pattern)
}
