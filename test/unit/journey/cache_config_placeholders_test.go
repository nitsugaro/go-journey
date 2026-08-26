package journey_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	gojourney "github.com/nitsugaro/go-journey"
	jcache "github.com/nitsugaro/go-journey/cache"
	"github.com/nitsugaro/go-journey/steps"
	"github.com/nitsugaro/go-journey/types"
)

func TestCacheConfigPlaceholderResolverUsesOnlyCustomResolvers(t *testing.T) {
	resolver := gojourney.NewCacheConfigPlaceholderResolver(map[string]types.PlaceholderResolver{
		"secret": func(path string) (any, error) {
			switch path {
			case "host":
				return "ldap.internal", nil
			case "port":
				return 636, nil
			case "enabled":
				return true, nil
			default:
				t.Fatalf("unexpected secret path %q", path)
				return nil, nil
			}
		},
	})
	resolved, err := resolver("ldap_client", "main", json.RawMessage(`{
		"url": "ldaps://${secret.host}",
		"port": "${secret.port}",
		"enabled": "${secret.enabled}"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(resolved, &config); err != nil {
		t.Fatal(err)
	}
	if config["url"] != "ldaps://ldap.internal" {
		t.Fatalf("url=%#v", config["url"])
	}
	if config["port"] != float64(636) {
		t.Fatalf("port=%#v", config["port"])
	}
	if config["enabled"] != true {
		t.Fatalf("enabled=%#v", config["enabled"])
	}
}

func TestCacheConfigPlaceholderResolverRejectsJourneyContextPrefixes(t *testing.T) {
	resolver := gojourney.NewCacheConfigPlaceholderResolver(map[string]types.PlaceholderResolver{
		"secret": func(path string) (any, error) { return path, nil },
	})
	_, err := resolver("http_client", "default", json.RawMessage(`{"password":"${ctx.password}"}`))
	if err == nil {
		t.Fatal("expected context placeholder to be rejected")
	}
	if !strings.Contains(err.Error(), "context placeholder") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCacheConfigPlaceholderResolverMaterializesGenericArraysIntoTypedConfig(t *testing.T) {
	resolvers := map[string]types.PlaceholderResolver{
		"env": func(path string) (any, error) {
			if path != "routes" {
				t.Fatalf("unexpected env path %q", path)
			}
			return []any{"https://httpbun.com/any", "https://google.com/404"}, nil
		},
	}
	config := &jcache.ManagerConfig{
		FolderPath:     t.TempDir(),
		ConfigResolver: gojourney.NewCacheConfigPlaceholderResolver(resolvers),
		Caches: map[string]jcache.CacheConfig{
			steps.HTTPRouteTableCacheKey: {Factory: steps.HTTPRouteTableFactory},
		},
	}
	manager, err := jcache.NewManager(config)
	if err != nil {
		t.Fatal(err)
	}
	raw := json.RawMessage(`{
		"routes": [{
			"name": "api-routes",
			"uris": "${env.routes}",
			"methods": ["GET", "POST"],
			"upstream": "https://httpbun.com"
		}]
	}`)
	if err := manager.UpdateCacheInstance(steps.HTTPRouteTableCacheKey, "api-routes", raw); err != nil {
		t.Fatal(err)
	}

	// Recreate the manager to prove persisted placeholder configuration is
	// resolved before the concrete []string field is unmarshaled.
	reloaded, err := jcache.NewManager(config)
	if err != nil {
		t.Fatal(err)
	}
	value, found := reloaded.GetCacheInstance(steps.HTTPRouteTableCacheKey, "api-routes")
	if !found {
		t.Fatal("reloaded route table was not found")
	}
	table, ok := value.(*steps.HTTPRouteTable)
	if !ok {
		t.Fatalf("instance type=%T", value)
	}
	for _, uri := range []string{"https://httpbun.com/any", "https://google.com/404"} {
		if _, matched := table.MatchURI(uri, http.MethodGet); !matched {
			t.Fatalf("generic resolver array did not materialize route %q", uri)
		}
	}
}
