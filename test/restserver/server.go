package restserver

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	gojourney "github.com/nitsugaro/go-journey"
	jcache "github.com/nitsugaro/go-journey/cache"
	"github.com/nitsugaro/go-journey/steps"
	"github.com/nitsugaro/go-journey/types"
	jsrun "github.com/nitsugaro/go-jsruntime/v2"
)

type Config struct {
	JourneyFolder  string
	ScriptFolder   string
	SchemaFolder   string
	ScheduleFolder string
	CacheFolder    string
	EncryptKey     []byte
}

func New(config *Config) (*gin.Engine, error) {
	if config == nil {
		return nil, errors.New("rest server config is nil")
	}
	if config.JourneyFolder == "" || config.ScriptFolder == "" {
		return nil, errors.New("journey and script folders are required")
	}
	if len(config.EncryptKey) != 16 && len(config.EncryptKey) != 24 && len(config.EncryptKey) != 32 {
		return nil, errors.New("encrypt key must contain 16, 24, or 32 bytes")
	}
	registry := steps.GetDefaultStepRegistry()
	journeys, err := gojourney.NewJourneyStorage(config.JourneyFolder, registry)
	if err != nil {
		return nil, err
	}
	if err := journeys.LoadFromDisk(); err != nil {
		return nil, err
	}
	cacheFolder := strings.TrimSpace(config.CacheFolder)
	if cacheFolder == "" {
		cacheFolder = filepath.Join(filepath.Dir(config.JourneyFolder), "cache")
	}
	placeholderResolvers := map[string]types.PlaceholderResolver{
		"env": func(path string) (any, error) {
			if path == "available.domain" {
				return "httpbun.com", nil
			}

			if path == "routes" {
				return []any{"https://httpbun.com/any", "https://google.com/404"}, nil
			}

			return nil, errors.New("xd")
		},
	}
	cacheManager, err := jcache.NewManager(&jcache.ManagerConfig{
		FolderPath:     cacheFolder,
		ConfigResolver: gojourney.NewCacheConfigPlaceholderResolver(placeholderResolvers),
		Caches: map[string]jcache.CacheConfig{
			steps.HTTPClientCacheKey:     {Factory: steps.HTTPClientFactory, MaxInstances: 10},
			steps.LDAPClientCacheKey:     {Factory: steps.LDAPClientFactory, MaxInstances: 10},
			steps.HTTPRouteTableCacheKey: {Factory: steps.HTTPRouteTableFactory},
		},
	})
	if err != nil {
		return nil, err
	}
	scriptManager, scriptStorage := jsrun.NewDefaultStorage(config.ScriptFolder)
	if err := scriptStorage.LoadFromDisk(); err != nil {
		return nil, err
	}
	if err := steps.ConfigureScriptRuntime(cacheManager, scriptManager, scriptStorage); err != nil {
		return nil, err
	}
	schemaFolder := strings.TrimSpace(config.SchemaFolder)
	if schemaFolder == "" {
		schemaFolder = filepath.Join(filepath.Dir(config.JourneyFolder), "schemas")
	}
	schemaStorage, err := gojourney.NewDeveloperSchemaStorage(schemaFolder)
	if err != nil {
		return nil, err
	}
	scheduleFolder := strings.TrimSpace(config.ScheduleFolder)
	if scheduleFolder == "" {
		scheduleFolder = filepath.Join(filepath.Dir(config.JourneyFolder), "schedules")
	}
	scheduleStorage, err := gojourney.NewScheduleStorage(scheduleFolder)
	if err != nil {
		return nil, err
	}
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/health", func(context *gin.Context) {
		context.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	gojourney.NewManager(&gojourney.JourneyManagerConfig{
		JourneyStorage:       journeys,
		CacheManager:         cacheManager,
		SchemaStorage:        schemaStorage,
		ScheduleStorage:      scheduleStorage,
		EncryptKey:           config.EncryptKey,
		PlaceholderResolvers: placeholderResolvers,
		Observer:             gojourney.NewJSONEventObserver(os.Stdout),
		RESTAPI: &gojourney.RESTAPIConfig{
			Enabled: true, Router: router, BasePath: "/journey",
			PrepareExecution: func(context *gin.Context, execution *types.JourneyExecute) error {
				realm, _ := context.Get("realm")
				if realmName := strings.TrimSpace(fmt.Sprint(realm)); realmName != "" && realm != nil {
					execution.Payload.SetRealm(&types.Realm{Name: realmName})
				} else if realmName := strings.TrimSpace(context.Param("realm")); realmName != "" {
					execution.Payload.SetRealm(&types.Realm{Name: realmName})
				}
				return nil
			},
		},
	}).OnJourneySuccess(func(jee *types.JourneyExecutionEvent) {
		fmt.Println("TERMINÓ: " + jee.Journey.Name)
	})

	return router, nil
}
