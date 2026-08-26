package steps_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	jcache "github.com/nitsugaro/go-journey/cache"
	journeysteps "github.com/nitsugaro/go-journey/steps"
	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type proxyRecordingHTTPClient struct {
	method   string
	uri      string
	headers  map[string]string
	body     []byte
	response *goutils.Response
	err      error
}

func (client *proxyRecordingHTTPClient) Request(method, uri string, headers map[string]string, body []byte) (*goutils.Response, error) {
	client.method = method
	client.uri = uri
	client.headers = headers
	client.body = append([]byte(nil), body...)
	if client.err != nil {
		return nil, client.err
	}
	if client.response != nil {
		return client.response, nil
	}
	return &goutils.Response{Status: http.StatusOK, Headers: http.Header{}, Body: []byte("ok")}, nil
}

func (client *proxyRecordingHTTPClient) RequestWithContext(_ context.Context, method, uri string, headers map[string]string, body []byte) (*goutils.Response, error) {
	return client.Request(method, uri, headers, body)
}

func TestHTTPRouteTableMatchesMostSpecificRouteAndRewrites(t *testing.T) {
	table, err := journeysteps.NewHTTPRouteTable(&journeysteps.HTTPRouteTableConfig{
		Upstreams: map[string]journeysteps.HTTPRouteUpstream{
			"api": {URL: "https://backend.local/base", HTTPInstance: "proxy"},
		},
		Routes: []journeysteps.HTTPRoute{
			{Name: "user", URIs: []string{"https://api.example.com/users/{id}"}, Methods: []string{http.MethodGet}, Upstream: "api", Rewrite: "/v1/accounts/{id}"},
			{Name: "fallback", URIs: []string{"https://api.example.com/users/*"}, Methods: []string{http.MethodGet}, Upstream: "api"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	match, found := table.MatchURI("https://api.example.com/users/42", http.MethodGet)
	if !found {
		t.Fatal("expected route match")
	}
	if match.Route.Name != "user" || match.Params["id"] != "42" || match.HTTPInstance != "proxy" {
		t.Fatalf("unexpected match: %#v", match)
	}
	target, err := table.TargetURI(match, "x=1")
	if err != nil {
		t.Fatal(err)
	}
	if target != "https://backend.local/base/v1/accounts/42?x=1" {
		t.Fatalf("target uri = %q", target)
	}
}

func TestHTTPRouteTableUsesArrayOrderAsGlobalPriority(t *testing.T) {
	requestURI := "https://api.example.com/users/42"
	first := journeysteps.HTTPRoute{
		Name: "first broad rule", URIs: []string{"*://*/users/**"}, Methods: []string{"*"},
		Policies: []journeysteps.HTTPRoutePolicy{{Name: "policy-1"}},
	}
	second := journeysteps.HTTPRoute{
		Name: "second exact rule", URIs: []string{"https://api.example.com/users/{id}"}, Methods: []string{http.MethodGet},
		Policies: []journeysteps.HTTPRoutePolicy{{Name: "policy-2"}},
	}

	table, err := journeysteps.NewHTTPRouteTable(&journeysteps.HTTPRouteTableConfig{Routes: []journeysteps.HTTPRoute{first, second}})
	if err != nil {
		t.Fatal(err)
	}
	match, found := table.MatchURI(requestURI, http.MethodGet)
	if !found || match.Route.Name != first.Name {
		t.Fatalf("first configured rule must win: found=%v match=%#v", found, match)
	}

	table, err = journeysteps.NewHTTPRouteTable(&journeysteps.HTTPRouteTableConfig{Routes: []journeysteps.HTTPRoute{second, first}})
	if err != nil {
		t.Fatal(err)
	}
	match, found = table.MatchURI(requestURI, http.MethodGet)
	if !found || match.Route.Name != second.Name || match.Params["id"] != "42" {
		t.Fatalf("reordered first rule must win: found=%v match=%#v", found, match)
	}
}

func TestHTTPRouteTreeKeepsTerminalGroupMethodAndParameterIdentity(t *testing.T) {
	table, err := journeysteps.NewHTTPRouteTable(&journeysteps.HTTPRouteTableConfig{
		Routes: []journeysteps.HTTPRoute{
			{
				Name:     "post by account",
				URIs:     []string{"https://api.example.com/resources/{account}"},
				Methods:  []string{http.MethodPost},
				Policies: []journeysteps.HTTPRoutePolicy{{Name: "post-policy"}},
			},
			{
				Name:     "get by resource",
				URIs:     []string{"https://api.example.com/resources/{resource}"},
				Methods:  []string{http.MethodGet},
				Policies: []journeysteps.HTTPRoutePolicy{{Name: "get-policy"}},
			},
			{
				Name:     "fallback",
				URIs:     []string{"*://*/resources/*"},
				Methods:  []string{"*"},
				Policies: []journeysteps.HTTPRoutePolicy{{Name: "fallback-policy"}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	getMatch, found := table.MatchURI("https://api.example.com/resources/abc", http.MethodGet)
	if !found || getMatch.Route.Name != "get by resource" {
		t.Fatalf("GET must select its first compatible terminal: found=%v match=%#v", found, getMatch)
	}
	if getMatch.Params["resource"] != "abc" {
		t.Fatalf("GET parameter identity was not preserved: %#v", getMatch.Params)
	}
	if _, leaked := getMatch.Params["account"]; leaked {
		t.Fatalf("parameter name from another shared tree branch leaked: %#v", getMatch.Params)
	}
	if got := getMatch.Route.Policies[0].Name; got != "get-policy" {
		t.Fatalf("GET policy = %q", got)
	}

	postMatch, found := table.MatchURI("https://api.example.com/resources/abc", http.MethodPost)
	if !found || postMatch.Route.Name != "post by account" {
		t.Fatalf("POST must select the earlier compatible terminal: found=%v match=%#v", found, postMatch)
	}
	if postMatch.Params["account"] != "abc" {
		t.Fatalf("POST parameter identity was not preserved: %#v", postMatch.Params)
	}
	if got := postMatch.Route.Policies[0].Name; got != "post-policy" {
		t.Fatalf("POST policy = %q", got)
	}
}

func TestHTTPRouteTreeUsesURIOrderWithinTheSameGroup(t *testing.T) {
	table, err := journeysteps.NewHTTPRouteTable(&journeysteps.HTTPRouteTableConfig{
		Routes: []journeysteps.HTTPRoute{
			{
				Name: "multi-uri",
				URIs: []string{
					"*://*/assets/{generic}",
					"https://cdn.example.com/assets/{exact}",
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	match, found := table.MatchURI("https://cdn.example.com/assets/logo.svg", http.MethodGet)
	if !found {
		t.Fatal("expected route match")
	}
	if len(match.Route.URIs) != 1 || match.Route.URIs[0] != "*://*/assets/{generic}" {
		t.Fatalf("first matching URI in the group must win: %#v", match.Route.URIs)
	}
	if match.Params["generic"] != "logo.svg" {
		t.Fatalf("parameters must belong to the selected URI: %#v", match.Params)
	}
	if _, leaked := match.Params["exact"]; leaked {
		t.Fatalf("later URI parameters leaked into the match: %#v", match.Params)
	}
}

func TestHTTPRouteTableMatchesCompleteURIWithoutMixingHostAndPath(t *testing.T) {
	table, err := journeysteps.NewHTTPRouteTable(&journeysteps.HTTPRouteTableConfig{
		Routes: []journeysteps.HTTPRoute{
			{
				Name: "protected resources",
				URIs: []string{
					"https://a.example.com/my/recurso",
					"*://*.dominio.com/otro/recurso",
				},
				Methods: []string{http.MethodGet},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, uri := range []string{
		"https://a.example.com/my/recurso",
		"http://api.dominio.com/otro/recurso",
	} {
		if match, found := table.MatchURI(uri, http.MethodGet); !found || match.Route.Name != "protected resources" {
			t.Fatalf("expected %q to match, got found=%v match=%#v", uri, found, match)
		}
	}
	for _, uri := range []string{
		"http://a.example.com/my/recurso",
		"https://a.example.com/otro/recurso",
		"https://api.dominio.com/my/recurso",
		"https://dominio.com/otro/recurso",
	} {
		if match, found := table.MatchURI(uri, http.MethodGet); found {
			t.Fatalf("expected %q not to match, got %#v", uri, match)
		}
	}
}

func TestHTTPRouteTableSupportsURIWildcardCombinations(t *testing.T) {
	tests := []struct {
		name      string
		pattern   string
		matches   []string
		rejects   []string
		paramName string
		param     string
	}{
		{
			name: "any protocol", pattern: "*://api.example.com/status",
			matches: []string{"http://api.example.com/status", "https://api.example.com/status"},
			rejects: []string{"https://other.example.com/status", "https://api.example.com/other"},
		},
		{
			name: "wildcard subdomain", pattern: "https://*.example.com/status",
			matches: []string{"https://api.example.com/status", "https://v1.api.example.com/status"},
			rejects: []string{"https://example.com/status", "http://api.example.com/status"},
		},
		{
			name: "any host", pattern: "https://*/status",
			matches: []string{"https://api.example.com/status", "https://localhost/status"},
			rejects: []string{"http://api.example.com/status", "https://api.example.com/other"},
		},
		{
			name: "single path segment", pattern: "https://api.example.com/files/*",
			matches: []string{"https://api.example.com/files/report.pdf"},
			rejects: []string{"https://api.example.com/files", "https://api.example.com/files/2026/report.pdf"},
		},
		{
			name: "remaining path", pattern: "https://api.example.com/assets/**",
			matches: []string{"https://api.example.com/assets", "https://api.example.com/assets/icons/edit.svg"},
			rejects: []string{"https://api.example.com/asset/icons/edit.svg"},
		},
		{
			name: "path parameter", pattern: "https://api.example.com/users/{id}",
			matches:   []string{"https://api.example.com/users/42"},
			rejects:   []string{"https://api.example.com/users", "https://api.example.com/users/42/profile"},
			paramName: "id", param: "42",
		},
		{
			name: "combined wildcards", pattern: "*://*.example.com/api/**",
			matches: []string{"http://v1.example.com/api", "https://edge.us.example.com/api/users/42"},
			rejects: []string{"https://example.com/api/users/42", "https://edge.example.net/api/users/42"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			table, err := journeysteps.NewHTTPRouteTable(&journeysteps.HTTPRouteTableConfig{
				Routes: []journeysteps.HTTPRoute{{Name: test.name, URIs: []string{test.pattern}}},
			})
			if err != nil {
				t.Fatal(err)
			}
			for _, uri := range test.matches {
				match, found := table.MatchURI(uri, http.MethodGet)
				if !found {
					t.Fatalf("expected %q to match %q", uri, test.pattern)
				}
				if test.paramName != "" && match.Params[test.paramName] != test.param {
					t.Fatalf("parameter %q = %q, want %q", test.paramName, match.Params[test.paramName], test.param)
				}
			}
			for _, uri := range test.rejects {
				if match, found := table.MatchURI(uri, http.MethodGet); found {
					t.Fatalf("expected %q not to match %q, got %#v", uri, test.pattern, match)
				}
			}
		})
	}
}

func TestHTTPProxyBaseURLAndPathRewriteSelectCompleteURI(t *testing.T) {
	transaction := newStepTransaction()
	transaction.Context = context.Background()
	transaction.Request = types.NewMemoryRequest(&types.JourneyRequest{
		Method:    http.MethodGet,
		RoutePath: "/servicios/my/recurso",
		RawQuery:  "debug=true",
		BaseURL:   "http://journey.local",
		Host:      "journey.local",
		Protocol:  "http",
	})

	cacheManager, err := jcache.NewManager(&jcache.ManagerConfig{})
	if err != nil {
		t.Fatal(err)
	}
	table, err := journeysteps.NewHTTPRouteTable(&journeysteps.HTTPRouteTableConfig{
		DefaultHTTPInstance: "proxy",
		Routes: []journeysteps.HTTPRoute{
			{Name: "resource policies", URIs: []string{"https://a.example.com/my/recurso"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	client := &proxyRecordingHTTPClient{}
	if err := cacheManager.UpdateRuntimeCacheInstance(journeysteps.HTTPRouteTableCacheKey, "routes", table, 0); err != nil {
		t.Fatal(err)
	}
	if err := cacheManager.UpdateRuntimeCacheInstance(journeysteps.HTTPClientCacheKey, "proxy", client, 0); err != nil {
		t.Fatal(err)
	}
	transaction.CacheManager = cacheManager

	config, err := types.ResolveStepConfig(map[string]any{
		"route_table": "routes",
		"base_url":    "https://a.example.com",
		"rewrite":     map[string]string{"/servicios": "/"},
	}, transaction.State)
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := (&journeysteps.HTTPProxy{}).Execute(transaction, config)
	if err != nil {
		t.Fatal(err)
	}
	if outcome != "true" {
		t.Fatalf("outcome = %q", outcome)
	}
	if client.uri != "https://a.example.com/my/recurso?debug=true" {
		t.Fatalf("proxied URI = %q", client.uri)
	}
}

func TestHTTPProxyWritesUpstreamResponse(t *testing.T) {
	transaction := newStepTransaction()
	transaction.Context = context.Background()
	transaction.Request = types.NewMemoryRequest(&types.JourneyRequest{
		Method:    http.MethodPost,
		Path:      "/journey/users/42",
		RoutePath: "/users/42",
		RawQuery:  "debug=true",
		Host:      "api.example.com",
		Headers: map[string][]string{
			"Host":         {"api.example.com"},
			"Connection":   {"close"},
			"Content-Type": {"text/plain"},
			"X-Keep":       {"yes"},
			"X-Remove":     {"drop"},
		},
		Body: types.JourneyRequestBody{ContentType: "text/plain", Data: []byte("hello")},
	})

	cacheManager, err := jcache.NewManager(&jcache.ManagerConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if err := journeysteps.EnsureDefaultCacheConfigurations(cacheManager); err != nil {
		t.Fatal(err)
	}
	table, err := journeysteps.NewHTTPRouteTable(&journeysteps.HTTPRouteTableConfig{
		AddHeaders:   map[string]string{"X-Table": "table"},
		StripHeaders: []string{"X-Remove"},
		Upstreams: map[string]journeysteps.HTTPRouteUpstream{
			"api": {URL: "https://backend.local", HTTPInstance: "proxy", AddHeaders: map[string]string{"X-Upstream": "upstream"}},
		},
		Routes: []journeysteps.HTTPRoute{
			{Name: "user", URIs: []string{"http://api.example.com/users/{id}"}, Methods: []string{http.MethodPost}, Upstream: "api", Rewrite: "/v1/users/{id}", AddHeaders: map[string]string{"X-Route": "route"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	client := &proxyRecordingHTTPClient{response: &goutils.Response{
		Status: http.StatusCreated,
		Headers: http.Header{
			"Content-Type":        {"application/json"},
			"Content-Length":      {"999"},
			"Connection":          {"close"},
			"X-Upstream-Response": {"ok"},
		},
		Body: []byte(`{"ok":true}`),
	}}
	if err := cacheManager.UpdateRuntimeCacheInstance(journeysteps.HTTPRouteTableCacheKey, "routes", table, 0); err != nil {
		t.Fatal(err)
	}
	if err := cacheManager.UpdateRuntimeCacheInstance(journeysteps.HTTPClientCacheKey, "proxy", client, 0); err != nil {
		t.Fatal(err)
	}
	transaction.CacheManager = cacheManager

	config, err := types.ResolveStepConfig(map[string]any{
		"route_table":           "routes",
		"save_match_to":         "ctx.proxy_match",
		"copy_response_headers": true,
	}, transaction.State)
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := (&journeysteps.HTTPProxy{}).Execute(transaction, config)
	if err != nil {
		t.Fatal(err)
	}
	if outcome != "true" {
		t.Fatalf("outcome = %q", outcome)
	}

	response := transaction.Response.(*types.MemoryResponse)
	if response.StatusCode != http.StatusCreated || response.ContentType != "application/json" || string(response.BodyBytesValue) != `{"ok":true}` {
		t.Fatalf("unexpected response: %#v", response)
	}
	if response.Headers["X-Upstream-Response"][0] != "ok" {
		t.Fatalf("upstream response header was not copied: %#v", response.Headers)
	}
	if _, found := response.Headers["Content-Length"]; found {
		t.Fatalf("hop-by-hop response header was copied: %#v", response.Headers)
	}
	if client.method != http.MethodPost || client.uri != "https://backend.local/v1/users/42?debug=true" || string(client.body) != "hello" {
		t.Fatalf("unexpected proxied request: method=%q uri=%q body=%q", client.method, client.uri, client.body)
	}
	for key, expected := range map[string]string{"X-Keep": "yes", "X-Table": "table", "X-Upstream": "upstream", "X-Route": "route", "Content-Type": "text/plain"} {
		if client.headers[key] != expected {
			t.Fatalf("header %s = %q, want %q; all=%#v", key, client.headers[key], expected, client.headers)
		}
	}
	for _, key := range []string{"X-Remove", "Connection"} {
		if _, found := client.headers[key]; found {
			t.Fatalf("header %s should have been stripped: %#v", key, client.headers)
		}
	}
	if got := transaction.State.GetCtx().Get("proxy_match.route").AsStringOr(""); got != "user" {
		t.Fatalf("stored route = %q", got)
	}
	if got := transaction.State.GetCtx().Get("proxy_match.params.id").AsStringOr(""); got != "42" {
		t.Fatalf("stored route param = %q", got)
	}
}

func TestHTTPProxyReturnsConfiguredStatusWhenNoRouteMatches(t *testing.T) {
	transaction := newStepTransaction()
	transaction.Context = context.Background()
	transaction.Request = types.NewMemoryRequest(&types.JourneyRequest{
		Method:    http.MethodGet,
		RoutePath: "/users/42",
		Host:      "api.example.com",
		Headers:   map[string][]string{"Host": {"api.example.com"}},
	})
	cacheManager, err := jcache.NewManager(&jcache.ManagerConfig{})
	if err != nil {
		t.Fatal(err)
	}
	table, err := journeysteps.NewHTTPRouteTable(&journeysteps.HTTPRouteTableConfig{
		Routes: []journeysteps.HTTPRoute{{Name: "admin", URIs: []string{"http://api.example.com/admin"}, Upstream: "https://backend.local"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := cacheManager.UpdateRuntimeCacheInstance(journeysteps.HTTPRouteTableCacheKey, "routes", table, 0); err != nil {
		t.Fatal(err)
	}
	transaction.CacheManager = cacheManager

	config, err := types.ResolveStepConfig(map[string]any{"route_table": "routes", "no_match_status": 418}, transaction.State)
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := (&journeysteps.HTTPProxy{}).Execute(transaction, config)
	if err != nil {
		t.Fatal(err)
	}
	response := transaction.Response.(*types.MemoryResponse)
	if outcome != "false" || response.StatusCode != http.StatusTeapot || !strings.Contains(string(response.BodyBytesValue), "no HTTP proxy route matched") {
		t.Fatalf("outcome=%q response=%#v", outcome, response)
	}
}
