package jcache

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"

	"github.com/nitsugaro/go-nstore"
)

var (
	ErrCacheNotFound      = errors.New("cache not found")
	ErrInstanceNotFound   = errors.New("cache instance not found")
	ErrFactoryNotFound    = errors.New("cache factory not found")
	ErrInstanceNotPointer = errors.New("cache instance must be a non-nil pointer")
	ErrMaxInstances       = errors.New("cache maximum instances exceeded")
	ErrMaxSize            = errors.New("cache maximum size exceeded")
)

const DefaultInstanceID = "default"

type Factory func(config json.RawMessage) (any, error)

// ConfigResolver receives the persisted constructor JSON and returns the JSON
// that must be used to build the live instance. The persisted config must not
// be modified by the resolver; it can contain placeholders/secrets while the
// returned config contains the resolved runtime values.
type ConfigResolver func(cacheKey, instanceID string, config json.RawMessage) (json.RawMessage, error)

type CacheConfig struct {
	Factory          Factory
	MaxInstances     int
	MaxSizeBytes     int64
	Description      string
	Schema           json.RawMessage
	UserConfigurable bool
}

type CacheInfo struct {
	Key              string          `json:"key"`
	MaxInstances     int             `json:"max_instances"`
	MaxSizeBytes     int64           `json:"max_size_bytes"`
	Instances        int             `json:"instances"`
	SizeBytes        int64           `json:"size_bytes"`
	Persistable      bool            `json:"persistable"`
	UserConfigurable bool            `json:"user_configurable"`
	Description      string          `json:"description,omitempty"`
	Schema           json.RawMessage `json:"schema,omitempty"`
}

type CacheInstanceInfo struct {
	CacheKey   string          `json:"cache_key"`
	InstanceID string          `json:"instance_id"`
	Config     json.RawMessage `json:"config,omitempty"`
	Persisted  bool            `json:"persisted"`
	Runtime    bool            `json:"runtime"`
	SizeBytes  int64           `json:"size_bytes"`
}

type ManagerConfig struct {
	FolderPath     string
	Caches         map[string]CacheConfig
	ConfigResolver ConfigResolver
}

type persistedInstance struct {
	*nstore.Metadata
	CacheKey   string          `json:"cache_key"`
	InstanceID string          `json:"instance_id"`
	Config     json.RawMessage `json:"config"`
}

type instance struct {
	value    any
	size     int64
	recordID string
	config   json.RawMessage
}

// Manager permits concurrent reads while updates/removals are exclusive.
// Returned instances are shared singleton pointers and must themselves be
// concurrency-safe.
type Manager struct {
	mu        sync.RWMutex
	caches    map[string]map[string]instance
	configs   map[string]CacheConfig
	storage   *nstore.NStorage[*persistedInstance]
	instances int
	sizeBytes int64
	resolver  ConfigResolver
}

func NewManager(config *ManagerConfig) (*Manager, error) {
	if config == nil {
		config = &ManagerConfig{}
	}
	manager := &Manager{
		caches: map[string]map[string]instance{}, configs: cloneConfigs(config.Caches), resolver: config.ConfigResolver,
	}
	for key, cacheConfig := range manager.configs {
		if cacheConfig.MaxInstances < 0 || cacheConfig.MaxSizeBytes < 0 {
			return nil, fmt.Errorf("cache %q limits cannot be negative", key)
		}
	}
	if config.FolderPath == "" {
		return manager, nil
	}
	storage, err := nstore.New[*persistedInstance](config.FolderPath)
	if err != nil {
		return nil, err
	}
	if err := storage.LoadFromDisk(); err != nil {
		return nil, err
	}
	manager.storage = storage
	for _, record := range storage.ListOfCache() {
		factory := manager.configs[record.CacheKey].Factory
		if factory == nil {
			return nil, fmt.Errorf("%w: %s", ErrFactoryNotFound, record.CacheKey)
		}
		resolvedConfig, err := manager.resolveConfig(record.CacheKey, record.InstanceID, record.Config)
		if err != nil {
			return nil, fmt.Errorf("resolve %s/%s: %w", record.CacheKey, record.InstanceID, err)
		}
		value, err := factory(resolvedConfig)
		if err != nil {
			return nil, fmt.Errorf("construct %s/%s: %w", record.CacheKey, record.InstanceID, err)
		}
		if err := validatePointer(value); err != nil {
			return nil, fmt.Errorf("construct %s/%s: %w", record.CacheKey, record.InstanceID, err)
		}
		if err := manager.addLoaded(record, value); err != nil {
			return nil, err
		}
	}
	return manager, nil
}

func (manager *Manager) GetCache(key string) (map[string]any, bool) {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	cache, ok := manager.caches[key]
	if !ok {
		return nil, false
	}
	result := make(map[string]any, len(cache))
	for id, entry := range cache {
		result[id] = entry.value
	}
	return result, true
}

func (manager *Manager) GetCacheInstance(key, instanceID string) (any, bool) {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	cache, ok := manager.caches[key]
	if !ok {
		return nil, false
	}
	entry, ok := cache[instanceID]
	return entry.value, ok
}

func (manager *Manager) ConfigureCache(key string, config *CacheConfig) error {
	if config == nil {
		return errors.New("cache config is nil")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("cache key is required")
	}
	if config.MaxInstances < 0 || config.MaxSizeBytes < 0 {
		return fmt.Errorf("cache %q limits cannot be negative", key)
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.configs[key] = cloneConfig(*config)
	return nil
}

func (manager *Manager) ConfigureCacheIfMissing(key string, config *CacheConfig) error {
	if config == nil {
		return errors.New("cache config is nil")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("cache key is required")
	}
	if config.MaxInstances < 0 || config.MaxSizeBytes < 0 {
		return fmt.Errorf("cache %q limits cannot be negative", key)
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if existing, exists := manager.configs[key]; exists {
		if existing.Factory == nil {
			existing.Factory = config.Factory
		}
		if existing.Description == "" {
			existing.Description = config.Description
		}
		if len(existing.Schema) == 0 {
			existing.Schema = append(json.RawMessage(nil), config.Schema...)
		}
		if !existing.UserConfigurable {
			existing.UserConfigurable = config.UserConfigurable
		}
		manager.configs[key] = existing
		return nil
	}
	manager.configs[key] = cloneConfig(*config)
	return nil
}

func (manager *Manager) ConfigureConfigResolver(resolver ConfigResolver) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.resolver = resolver
}

// UpdateCacheInstance persists constructor configuration and atomically swaps
// the reconstructed live instance.
func (manager *Manager) UpdateCacheInstance(key, instanceID string, config any) error {
	manager.mu.RLock()
	cacheConfig := manager.configs[key]
	manager.mu.RUnlock()
	factory := cacheConfig.Factory
	if factory == nil {
		return fmt.Errorf("%w: %s", ErrFactoryNotFound, key)
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		return err
	}
	resolvedConfig, err := manager.resolveConfig(key, instanceID, encoded)
	if err != nil {
		return err
	}
	value, err := factory(resolvedConfig)
	if err != nil {
		return err
	}
	if err := validatePointer(value); err != nil {
		return err
	}

	manager.mu.Lock()
	defer manager.mu.Unlock()
	old, exists := manager.lookup(key, instanceID)
	if err := manager.checkLimits(key, exists, old.size, int64(len(encoded))); err != nil {
		return err
	}
	record := &persistedInstance{CacheKey: key, InstanceID: instanceID, Config: append(json.RawMessage(nil), encoded...)}
	if exists && old.recordID != "" {
		record.Metadata = &nstore.Metadata{ID: old.recordID}
	}
	if manager.storage != nil {
		if err := manager.storage.Save(record); err != nil {
			return err
		}
	}
	recordID := ""
	if record.Metadata != nil {
		recordID = record.ID
	}
	manager.store(key, instanceID, instance{
		value: value, size: int64(len(encoded)), recordID: recordID, config: append(json.RawMessage(nil), encoded...),
	}, exists, old.size)
	return nil
}

// UpdateRuntimeCacheInstance stores a process-local singleton. sizeBytes is the
// caller's estimate used by that cache category's MaxSizeBytes and may be zero
// when no size limit is configured.
func (manager *Manager) UpdateRuntimeCacheInstance(key, instanceID string, value any, sizeBytes int64) error {
	if err := validatePointer(value); err != nil {
		return err
	}
	if sizeBytes < 0 {
		return errors.New("cache instance size cannot be negative")
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	old, exists := manager.lookup(key, instanceID)
	if err := manager.checkLimits(key, exists, old.size, sizeBytes); err != nil {
		return err
	}
	if exists && old.recordID != "" && manager.storage != nil {
		if err := manager.deleteRecord(old.recordID); err != nil {
			return err
		}
	}
	manager.store(key, instanceID, instance{value: value, size: sizeBytes}, exists, old.size)
	return nil
}

func (manager *Manager) RemoveCacheInstance(key, instanceID string) error {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	cache, ok := manager.caches[key]
	if !ok {
		return ErrCacheNotFound
	}
	entry, ok := cache[instanceID]
	if !ok {
		return ErrInstanceNotFound
	}
	if entry.recordID != "" && manager.storage != nil {
		if err := manager.deleteRecord(entry.recordID); err != nil {
			return err
		}
	}
	delete(cache, instanceID)
	manager.instances--
	manager.sizeBytes -= entry.size
	if len(cache) == 0 {
		delete(manager.caches, key)
	}
	return nil
}

func (manager *Manager) RemoveCache(key string) error {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	cache, ok := manager.caches[key]
	if !ok {
		return ErrCacheNotFound
	}
	for _, entry := range cache {
		if entry.recordID != "" && manager.storage != nil {
			if err := manager.deleteRecord(entry.recordID); err != nil {
				return err
			}
		}
		manager.instances--
		manager.sizeBytes -= entry.size
	}
	delete(manager.caches, key)
	return nil
}

func (manager *Manager) Stats() (instances int, sizeBytes int64) {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return manager.instances, manager.sizeBytes
}

func (manager *Manager) CacheStats(key string) (instances int, sizeBytes int64) {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	for _, entry := range manager.caches[key] {
		instances++
		sizeBytes += entry.size
	}
	return instances, sizeBytes
}

func (manager *Manager) ListCaches() []CacheInfo {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	keys := map[string]struct{}{}
	for key := range manager.configs {
		keys[key] = struct{}{}
	}
	for key := range manager.caches {
		keys[key] = struct{}{}
	}
	result := make([]CacheInfo, 0, len(keys))
	for key := range keys {
		config := manager.configs[key]
		instances, sizeBytes := manager.cacheStatsLocked(key)
		result = append(result, CacheInfo{
			Key: key, MaxInstances: config.MaxInstances, MaxSizeBytes: config.MaxSizeBytes,
			Instances: instances, SizeBytes: sizeBytes, Persistable: config.Factory != nil,
			UserConfigurable: config.Factory != nil && config.UserConfigurable,
			Description:      config.Description, Schema: append(json.RawMessage(nil), config.Schema...),
		})
	}
	return result
}

func (manager *Manager) CacheInfo(key string) (CacheInfo, bool) {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	config, found := manager.configs[key]
	if !found {
		return CacheInfo{}, false
	}
	instances, sizeBytes := manager.cacheStatsLocked(key)
	return CacheInfo{
		Key: key, MaxInstances: config.MaxInstances, MaxSizeBytes: config.MaxSizeBytes,
		Instances: instances, SizeBytes: sizeBytes, Persistable: config.Factory != nil,
		UserConfigurable: config.Factory != nil && config.UserConfigurable,
		Description:      config.Description, Schema: append(json.RawMessage(nil), config.Schema...),
	}, true
}

func (manager *Manager) ListCacheInstances() []CacheInstanceInfo {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	result := []CacheInstanceInfo{}
	for key, cache := range manager.caches {
		for instanceID, entry := range cache {
			result = append(result, cacheInstanceInfo(key, instanceID, entry))
		}
	}
	return result
}

func (manager *Manager) CacheInstanceInfo(key, instanceID string) (CacheInstanceInfo, bool) {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	entry, ok := manager.lookup(key, instanceID)
	if !ok {
		return CacheInstanceInfo{}, false
	}
	return cacheInstanceInfo(key, instanceID, entry), true
}

func (manager *Manager) addLoaded(record *persistedInstance, value any) error {
	size := int64(len(record.Config))
	if err := manager.checkLimits(record.CacheKey, false, 0, size); err != nil {
		return err
	}
	manager.store(record.CacheKey, record.InstanceID, instance{
		value: value, size: size, recordID: record.ID, config: append(json.RawMessage(nil), record.Config...),
	}, false, 0)
	return nil
}

func (manager *Manager) resolveConfig(key, instanceID string, config json.RawMessage) (json.RawMessage, error) {
	cloned := append(json.RawMessage(nil), config...)
	manager.mu.RLock()
	resolver := manager.resolver
	manager.mu.RUnlock()
	if resolver == nil {
		return cloned, nil
	}
	resolved, err := resolver(key, instanceID, cloned)
	if err != nil {
		return nil, err
	}
	return append(json.RawMessage(nil), resolved...), nil
}

func (manager *Manager) checkLimits(key string, replacing bool, oldSize, newSize int64) error {
	config := manager.configs[key]
	instances, sizeBytes := manager.cacheStatsLocked(key)
	if !replacing && config.MaxInstances > 0 && instances+1 > config.MaxInstances {
		return ErrMaxInstances
	}
	if config.MaxSizeBytes > 0 && sizeBytes-oldSize+newSize > config.MaxSizeBytes {
		return ErrMaxSize
	}
	return nil
}

func (manager *Manager) cacheStatsLocked(key string) (instances int, sizeBytes int64) {
	for _, entry := range manager.caches[key] {
		instances++
		sizeBytes += entry.size
	}
	return instances, sizeBytes
}

func (manager *Manager) lookup(key, id string) (instance, bool) {
	cache := manager.caches[key]
	if cache == nil {
		return instance{}, false
	}
	entry, ok := cache[id]
	return entry, ok
}

func (manager *Manager) store(key, id string, entry instance, replacing bool, oldSize int64) {
	if manager.caches[key] == nil {
		manager.caches[key] = map[string]instance{}
	}
	manager.caches[key][id] = entry
	if !replacing {
		manager.instances++
	}
	manager.sizeBytes += entry.size - oldSize
}

func cacheInstanceInfo(key, instanceID string, entry instance) CacheInstanceInfo {
	return CacheInstanceInfo{
		CacheKey: key, InstanceID: instanceID, Config: append(json.RawMessage(nil), entry.config...),
		Persisted: entry.recordID != "", Runtime: entry.recordID == "", SizeBytes: entry.size,
	}
}

func (manager *Manager) deleteRecord(id string) error {
	err := manager.storage.Delete(id)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func validatePointer(value any) error {
	if value == nil {
		return ErrInstanceNotPointer
	}
	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return ErrInstanceNotPointer
	}
	return nil
}

func cloneConfigs(source map[string]CacheConfig) map[string]CacheConfig {
	result := make(map[string]CacheConfig, len(source))
	for key, config := range source {
		if key != "" {
			result[key] = cloneConfig(config)
		}
	}
	return result
}

func cloneConfig(config CacheConfig) CacheConfig {
	config.Schema = append(json.RawMessage(nil), config.Schema...)
	return config
}
