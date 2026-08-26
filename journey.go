package gojourney

import (
	"time"

	jcache "github.com/nitsugaro/go-journey/cache"
	"github.com/nitsugaro/go-journey/steps"
	"github.com/nitsugaro/go-journey/types"
	"github.com/nitsugaro/go-nstore"
	"github.com/nitsugaro/go-utils/v2/crypto"
)

type JourneyManagerConfig struct {
	FolderPath string
	// JourneyStorage replaces filesystem-backed journey configuration loading.
	// FolderPath is optional when JourneyStorage is provided.
	JourneyStorage JourneyConfigurations
	// Tokens replaces the built-in JWTE-backed signer and validator.
	Tokens JourneyTokens
	// EncryptKey must be stable across manager instances that share states.
	// Supported AES key sizes are 16, 24, and 32 bytes.
	EncryptKey []byte
	Steps      *types.Steps
	// CacheManager owns reusable singleton resources and persistent instance configurations.
	CacheManager *jcache.Manager
	// PlaceholderResolvers extends placeholders with custom prefixes. Context
	// prefixes remain built in and take precedence over custom registrations.
	// The env prefix is compiled once per path for the lifetime of this manager;
	// every other custom prefix remains dynamic and resolves at step execution.
	PlaceholderResolvers map[string]types.PlaceholderResolver
	// CustomStorageStates allows distributed or durable single-use state storage.
	CustomStorageStates    JourneyStates
	DefaultTTL             time.Duration
	DefaultIntervalCleanup time.Duration
	// RESTAPI optionally registers Gin CRUD and invocation routes.
	RESTAPI *RESTAPIConfig
	// SchemaStorage replaces filesystem-backed developer JSON Schema storage.
	// If omitted, FolderPath + "/schemas" is used.
	SchemaStorage types.DeveloperSchemaProvider
	// ScheduleStorage replaces filesystem-backed scheduler configuration storage.
	// If omitted, FolderPath + "/schedules" is used.
	ScheduleStorage RESTScheduleStorage
	// Observer receives structured engine, REST and script events. If nil, the
	// engine is silent.
	Observer types.Observer
}

type journeyManager struct {
	storage              JourneyConfigurations
	states               JourneyStates
	encryptKey           []byte
	steps                *types.Steps
	tokens               JourneyTokens
	placeholderResolvers map[string]types.PlaceholderResolver
	runtimeJourneys      runtimeJourneyCache
	staticEnv            staticEnvironmentCache
	cacheManager         *jcache.Manager
	schemas              types.DeveloperSchemaProvider
	scheduleStorage      RESTScheduleStorage
	scheduler            *Scheduler
	observer             types.Observer
	listeners            *journeyListenerRegistry
}

func NewManager(journeyManagerConfig *JourneyManagerConfig) *journeyManager {
	if journeyManagerConfig == nil {
		panic("gojourney: manager config is required")
	}
	if journeyManagerConfig.JourneyStorage == nil && len(journeyManagerConfig.FolderPath) == 0 {
		panic("gojourney: folder_path is a required field.")
	}

	var storage JourneyConfigurations = journeyManagerConfig.JourneyStorage
	var err error
	if storage == nil {
		fileStorage, storageErr := nstore.New[*types.JourneyConfiguration](journeyManagerConfig.FolderPath)
		if storageErr != nil {
			panic(storageErr)
		}
		if storageErr = fileStorage.LoadFromDisk(); storageErr != nil {
			panic(storageErr)
		}
		storage = &JourneyStorage{NStorage: fileStorage}
	}

	instance := &journeyManager{
		storage:              storage,
		steps:                journeyManagerConfig.Steps,
		states:               journeyManagerConfig.CustomStorageStates,
		encryptKey:           journeyManagerConfig.EncryptKey,
		tokens:               journeyManagerConfig.Tokens,
		placeholderResolvers: clonePlaceholderResolvers(journeyManagerConfig.PlaceholderResolvers),
		cacheManager:         journeyManagerConfig.CacheManager,
		schemas:              journeyManagerConfig.SchemaStorage,
		scheduleStorage:      journeyManagerConfig.ScheduleStorage,
		observer:             journeyManagerConfig.Observer,
		listeners:            newJourneyListenerRegistry(),
	}
	if instance.cacheManager == nil {
		instance.cacheManager, err = jcache.NewManager(&jcache.ManagerConfig{
			ConfigResolver: NewCacheConfigPlaceholderResolver(instance.placeholderResolvers),
		})
		if err != nil {
			panic("gojourney: cannot create cache manager: " + err.Error())
		}
	} else {
		instance.cacheManager.ConfigureConfigResolver(NewCacheConfigPlaceholderResolver(instance.placeholderResolvers))
	}
	if err := steps.EnsureDefaultCacheConfigurations(instance.cacheManager); err != nil {
		panic("gojourney: cannot configure default cache instances: " + err.Error())
	}
	if err := jcache.EnsureRegexpCache(instance.cacheManager, jcache.DefaultRegexpCacheMaxEntries()); err != nil {
		panic("gojourney: cannot configure regexp cache: " + err.Error())
	}
	if err := steps.EnsureDefaultScriptRuntime(instance.cacheManager); err != nil {
		panic("gojourney: cannot configure script runtime: " + err.Error())
	}
	if instance.schemas == nil && journeyManagerConfig.FolderPath != "" {
		schemaStorage, schemaErr := NewDeveloperSchemaStorage(journeyManagerConfig.FolderPath + "/schemas")
		if schemaErr != nil {
			panic("gojourney: cannot configure schema storage: " + schemaErr.Error())
		}
		instance.schemas = schemaStorage
	}
	if instance.schemas != nil {
		if err := instance.cacheManager.UpdateRuntimeCacheInstance(steps.SchemaStorageCacheKey, jcache.DefaultInstanceID, instance.schemas, 0); err != nil {
			panic("gojourney: cannot register schema storage: " + err.Error())
		}
	}
	if instance.scheduleStorage == nil && journeyManagerConfig.RESTAPI != nil {
		instance.scheduleStorage = journeyManagerConfig.RESTAPI.ScheduleStorage
	}
	if instance.scheduleStorage == nil {
		instance.scheduleStorage = ensureScheduleStorage(journeyManagerConfig)
	}

	if instance.states == nil {
		defaultTTL := journeyManagerConfig.DefaultTTL
		if journeyManagerConfig.DefaultTTL == 0 {
			defaultTTL = 10 * time.Minute
		}

		defaultIntervalCleanup := journeyManagerConfig.DefaultIntervalCleanup
		if journeyManagerConfig.DefaultIntervalCleanup == 0 {
			defaultIntervalCleanup = 1 * time.Minute
		}

		instance.states = newDefaultJourneyStates(defaultTTL, defaultIntervalCleanup)
	}
	if instance.tokens == nil {
		instance.tokens = defaultJourneyTokens{}
	}

	if len(instance.encryptKey) == 0 {
		instance.encryptKey, err = crypto.GetRandBytes(32)
		if err != nil {
			panic("gojourney: cannot generate encryption key: " + err.Error())
		}
	} else if len(instance.encryptKey) != 16 && len(instance.encryptKey) != 24 && len(instance.encryptKey) != 32 {
		panic("gojourney: encrypt key must contain 16, 24, or 32 bytes")
	}
	instance.encryptKey = append([]byte(nil), instance.encryptKey...)

	if instance.steps == nil {
		instance.steps = steps.GetDefaultStepRegistry()
	} else {
		// A supplied registry extends the built-ins. Existing custom entries win,
		// which lets a developer intentionally replace a default implementation.
		if err := instance.steps.AddMissingFrom(steps.GetDefaultStepRegistry()); err != nil {
			panic("gojourney: cannot merge default step registry: " + err.Error())
		}
	}
	if fileStorage, ok := instance.storage.(*JourneyStorage); ok {
		fileStorage.Steps = instance.steps
	}
	ensureSchedulerRuntime(instance, instance.scheduleStorage)
	if err := instance.registerRESTAPI(journeyManagerConfig.RESTAPI); err != nil {
		panic("gojourney: cannot configure REST API: " + err.Error())
	}

	return instance
}

func clonePlaceholderResolvers(source map[string]types.PlaceholderResolver) map[string]types.PlaceholderResolver {
	cloned := make(map[string]types.PlaceholderResolver, len(source))
	for prefix, resolver := range source {
		if prefix != "" && resolver != nil && !types.IsCtx(prefix) && prefix != "encCtx" && prefix != "closedCtx" && prefix != "tempCtx" {
			cloned[prefix] = resolver
		}
	}
	return cloned
}

// GetCacheManager returns the manager-owned singleton registry so applications
// can update instances dynamically without rebuilding JourneyExecute values.
func (jm *journeyManager) GetCacheManager() *jcache.Manager {
	return jm.cacheManager
}

// GetScheduler returns the optional runtime scheduler when schedule storage was
// configured for this manager.
func (jm *journeyManager) GetScheduler() *Scheduler {
	return jm.scheduler
}
