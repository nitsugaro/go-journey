package steps_test

import (
	"context"
	"net/http"
	"testing"

	jcache "github.com/nitsugaro/go-journey/cache"
	journeysteps "github.com/nitsugaro/go-journey/steps"
	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type recordingHTTPClient struct {
	headers map[string]string
}

func (client *recordingHTTPClient) Request(_ string, _ string, headers map[string]string, _ []byte) (*goutils.Response, error) {
	client.headers = headers
	return &goutils.Response{Status: http.StatusOK, Headers: http.Header{}, Body: []byte("ok")}, nil
}

func (client *recordingHTTPClient) RequestWithContext(_ context.Context, method, uri string, headers map[string]string, body []byte) (*goutils.Response, error) {
	return client.Request(method, uri, headers, body)
}

func TestHttpRequestSendsResolvedObjectHeaderPlaceholders(t *testing.T) {
	transaction := newStepTransaction()
	transaction.State.GetCtx().Set("audit_response.rawBody", "audit-ok")
	transaction.State.GetCtx().Set("analytics_response.rawBody", "analytics-ok")
	client := &recordingHTTPClient{}
	cacheManager, _ := jcache.NewManager(&jcache.ManagerConfig{})
	if err := cacheManager.UpdateRuntimeCacheInstance(journeysteps.HTTPClientCacheKey, jcache.DefaultInstanceID, client, 0); err != nil {
		t.Fatal(err)
	}
	transaction.CacheManager = cacheManager

	raw := map[string]any{
		"uri": "https://example.test", "method": "POST", "headers": map[string]any{
			"X-Audit-Response":     "${ctx.audit_response.rawBody}",
			"X-Analytics-Response": "${ctx.analytics_response.rawBody}",
		},
		"body": "forward", "content_type": "TEXT", "response_output": "ctx.forward_response",
		"vars": map[string]any{
			"headers.X-Audit-Response": map[string]any{"type": "string", "placeholders": []any{map[string]any{
				"template": "ctx.audit_response.rawBody", "starts_at": 0, "ends_at": 29,
			}}},
			"headers.X-Analytics-Response": map[string]any{"type": "string", "placeholders": []any{map[string]any{
				"template": "ctx.analytics_response.rawBody", "starts_at": 0, "ends_at": 33,
			}}},
		},
	}
	config, err := types.ResolveStepConfig(raw, transaction.State)
	if err != nil {
		t.Fatal(err)
	}
	if outcome, err := (&journeysteps.HttpRequest{}).Execute(transaction, config); err != nil || outcome != "true" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
	if client.headers["X-Audit-Response"] != "audit-ok" || client.headers["X-Analytics-Response"] != "analytics-ok" {
		t.Fatalf("sent headers = %#v", client.headers)
	}
}
