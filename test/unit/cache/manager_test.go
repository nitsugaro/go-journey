package cache_test

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"

	jcache "github.com/nitsugaro/go-journey/cache"
)

type serviceConfig struct {
	Name    string `json:"name"`
	Retries int    `json:"retries,omitempty"`
	Enabled bool   `json:"enabled,omitempty"`
}

type service struct {
	name    string
	retries int
	enabled bool
}

func serviceFactory(raw json.RawMessage) (any, error) {
	var config serviceConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		return nil, err
	}
	return &service{name: config.Name, retries: config.Retries, enabled: config.Enabled}, nil
}

func TestCacheManagerPersistsConfigurationAndReconstructsPointers(t *testing.T) {
	folder := t.TempDir()
	config := jcache.ManagerConfig{FolderPath: folder, Caches: map[string]jcache.CacheConfig{
		"service": {Factory: serviceFactory},
	}}
	manager, err := jcache.NewManager(&config)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.UpdateCacheInstance("service", "primary", serviceConfig{Name: "first"}); err != nil {
		t.Fatal(err)
	}
	value, ok := manager.GetCacheInstance("service", "primary")
	if !ok || value.(*service).name != "first" {
		t.Fatalf("instance=%#v found=%v", value, ok)
	}

	restarted, err := jcache.NewManager(&config)
	if err != nil {
		t.Fatal(err)
	}
	restored, ok := restarted.GetCacheInstance("service", "primary")
	if !ok || restored.(*service).name != "first" {
		t.Fatalf("restored=%#v found=%v", restored, ok)
	}
	if err := restarted.RemoveCacheInstance("service", "primary"); err != nil {
		t.Fatal(err)
	}
	again, err := jcache.NewManager(&config)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := again.GetCacheInstance("service", "primary"); found {
		t.Fatal("removed persisted instance was reconstructed")
	}
}

func TestCacheManagerResolvesPersistedConfigurationWithoutMutatingStoredConfig(t *testing.T) {
	folder := t.TempDir()
	resolver := func(_ string, _ string, config json.RawMessage) (json.RawMessage, error) {
		var raw map[string]any
		if err := json.Unmarshal(config, &raw); err != nil {
			return nil, err
		}
		if raw["name"] == "${secret.service_name}" {
			raw["name"] = "resolved-service"
		}
		if raw["retries"] == "${secret.retries}" {
			raw["retries"] = 3
		}
		if raw["enabled"] == "${secret.enabled}" {
			raw["enabled"] = true
		}
		return json.Marshal(raw)
	}
	config := jcache.ManagerConfig{
		FolderPath:     folder,
		ConfigResolver: resolver,
		Caches: map[string]jcache.CacheConfig{
			"service": {Factory: serviceFactory},
		},
	}
	manager, err := jcache.NewManager(&config)
	if err != nil {
		t.Fatal(err)
	}
	rawConfig := map[string]any{
		"name":    "${secret.service_name}",
		"retries": "${secret.retries}",
		"enabled": "${secret.enabled}",
	}
	if err := manager.UpdateCacheInstance("service", "primary", rawConfig); err != nil {
		t.Fatal(err)
	}
	value, ok := manager.GetCacheInstance("service", "primary")
	if !ok {
		t.Fatal("resolved instance not found")
	}
	got := value.(*service)
	if got.name != "resolved-service" || got.retries != 3 || !got.enabled {
		t.Fatalf("resolved instance=%#v", got)
	}
	info, ok := manager.CacheInstanceInfo("service", "primary")
	if !ok {
		t.Fatal("instance info not found")
	}
	if !json.Valid(info.Config) {
		t.Fatalf("stored config is not JSON: %s", info.Config)
	}
	var stored map[string]any
	if err := json.Unmarshal(info.Config, &stored); err != nil {
		t.Fatal(err)
	}
	if stored["name"] != "${secret.service_name}" || stored["retries"] != "${secret.retries}" || stored["enabled"] != "${secret.enabled}" {
		t.Fatalf("stored config was mutated: %#v", stored)
	}

	restarted, err := jcache.NewManager(&config)
	if err != nil {
		t.Fatal(err)
	}
	restored, ok := restarted.GetCacheInstance("service", "primary")
	if !ok {
		t.Fatal("resolved persisted instance not restored")
	}
	restoredService := restored.(*service)
	if restoredService.name != "resolved-service" || restoredService.retries != 3 || !restoredService.enabled {
		t.Fatalf("restored instance=%#v", restoredService)
	}
}

func TestCacheManagerLimitsPointersAndConcurrentAccess(t *testing.T) {
	manager, err := jcache.NewManager(&jcache.ManagerConfig{
		Caches: map[string]jcache.CacheConfig{
			"service": {Factory: serviceFactory, MaxInstances: 1, MaxSizeBytes: 64},
			"runtime": {MaxInstances: 1, MaxSizeBytes: 8},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.UpdateRuntimeCacheInstance("runtime", "default", serviceConfig{Name: "not-pointer"}, 1); !errors.Is(err, jcache.ErrInstanceNotPointer) {
		t.Fatalf("non-pointer error=%v", err)
	}
	if err := manager.UpdateCacheInstance("service", "primary", serviceConfig{Name: "one"}); err != nil {
		t.Fatal(err)
	}
	if err := manager.UpdateRuntimeCacheInstance("runtime", "default", &service{name: "runtime"}, 8); err != nil {
		t.Fatal(err)
	}
	if err := manager.UpdateRuntimeCacheInstance("runtime", "extra", &service{}, 1); !errors.Is(err, jcache.ErrMaxInstances) {
		t.Fatalf("instance limit error=%v", err)
	}
	if err := manager.UpdateRuntimeCacheInstance("other", "unlimited", &service{}, 128); err != nil {
		t.Fatalf("one cache category affected another: %v", err)
	}

	var group sync.WaitGroup
	for index := 0; index < 24; index++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			if index%4 == 0 {
				_ = manager.UpdateCacheInstance("service", "primary", serviceConfig{Name: "updated"})
				return
			}
			for read := 0; read < 100; read++ {
				value, found := manager.GetCacheInstance("service", "primary")
				if !found || value == nil {
					t.Errorf("instance unavailable during concurrent read")
					return
				}
			}
		}(index)
	}
	group.Wait()
	instances, size := manager.Stats()
	if instances != 3 || size <= 0 {
		t.Fatalf("stats instances=%d size=%d", instances, size)
	}
	runtimeInstances, runtimeSize := manager.CacheStats("runtime")
	if runtimeInstances != 1 || runtimeSize != 8 {
		t.Fatalf("runtime stats instances=%d size=%d", runtimeInstances, runtimeSize)
	}
}

func TestRegexpCacheIsManagedRuntimeSingleton(t *testing.T) {
	manager, err := jcache.NewManager(&jcache.ManagerConfig{
		Caches: map[string]jcache.CacheConfig{
			jcache.RegexpCacheKey: {MaxInstances: 1},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := jcache.EnsureRegexpCache(manager, 2); err != nil {
		t.Fatal(err)
	}
	first, err := jcache.GetRegexp(manager, "^[a-z]+$")
	if err != nil {
		t.Fatal(err)
	}
	second, err := jcache.GetRegexp(manager, "^[a-z]+$")
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("compiled regexp was not reused")
	}
	instances, _ := manager.CacheStats(jcache.RegexpCacheKey)
	if instances != 1 {
		t.Fatalf("regexp cache instances=%d", instances)
	}
}
