package steps_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/lestrrat-go/jwx/v2/jwk"
	jcache "github.com/nitsugaro/go-journey/cache"
	"github.com/nitsugaro/go-journey/env"
	journeysteps "github.com/nitsugaro/go-journey/steps"
	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

func TestOIDCAuthorizationCodeStartsFlowWithClientInput(t *testing.T) {
	transaction := newStepTransaction()
	config := oidcConfig("https://provider.example/token")

	outcome, err := (&journeysteps.OIDCAuthorizationCode{}).Execute(transaction, config)
	if err != nil || outcome != "" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
	inputs := transaction.ClientInputsBuilder.GetNewInputs()
	if len(inputs) != 1 {
		t.Fatalf("new client inputs=%d", len(inputs))
	}
	if inputs[0].Type != "msg_input" || inputs[0].StepType != types.OIDCAuthorizationCodeStep || inputs[0].SendBack {
		t.Fatalf("unexpected client input: %#v", inputs[0])
	}
	output, ok := inputs[0].Output.(map[string]any)
	if !ok {
		t.Fatalf("client input output has type %T", inputs[0].Output)
	}
	authorizeURL, err := url.Parse(output["authorize_url"].(string))
	if err != nil {
		t.Fatal(err)
	}
	query := authorizeURL.Query()
	if query.Get("response_type") != "code" || query.Get("client_id") != "client-id" || query.Get("scope") != "openid email" {
		t.Fatalf("bad authorize query: %s", authorizeURL.RawQuery)
	}
	if query.Get("state") == "" || query.Get("nonce") == "" || query.Get("code_challenge") == "" || query.Get("code_challenge_method") != "S256" {
		t.Fatalf("missing oidc security parameters: %s", authorizeURL.RawQuery)
	}
	baseKey := env.GetContextKey("oidc." + transaction.CurrentStepID)
	if transaction.State.GetClosedCtx().Get(baseKey+".state").AsStringOr("") != query.Get("state") {
		t.Fatal("state was not saved in closed context")
	}
}

func TestOIDCAuthorizationCodeExchangesCallbackCode(t *testing.T) {
	var receivedBody string
	tokenServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		auth := "Basic " + base64.StdEncoding.EncodeToString([]byte("client-id:client-secret"))
		if request.Header.Get("Authorization") != auth {
			t.Fatalf("authorization header=%q", request.Header.Get("Authorization"))
		}
		data, _ := io.ReadAll(request.Body)
		receivedBody = string(data)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"access_token":"access","id_token":"id","token_type":"Bearer","expires_in":3600}`))
	}))
	defer tokenServer.Close()

	transaction := newStepTransaction()
	baseKey := env.GetContextKey("oidc." + transaction.CurrentStepID)
	transaction.State.GetClosedCtx().Set(baseKey+".state", "expected-state")
	transaction.State.GetClosedCtx().Set(baseKey+".code_verifier", "verifier")
	transaction.Request = types.NewHTTPRequestAccessor(httptest.NewRequest(http.MethodGet, "/callback?code=abc&state=expected-state", nil), 0)

	outcome, err := (&journeysteps.OIDCAuthorizationCode{}).Execute(transaction, oidcConfig(tokenServer.URL))
	if err != nil || outcome != "true" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
	values, err := url.ParseQuery(receivedBody)
	if err != nil {
		t.Fatal(err)
	}
	if values.Get("grant_type") != "authorization_code" || values.Get("code") != "abc" || values.Get("code_verifier") != "verifier" {
		t.Fatalf("bad token request body: %s", receivedBody)
	}
	if got := transaction.State.GetEncryptedCtx().Get("oidc.tokens.access_token").AsStringOr(""); got != "access" {
		t.Fatalf("access token not stored: %q", got)
	}
	if transaction.State.GetClosedCtx().IsDefined(baseKey) {
		t.Fatal("internal oidc state was not cleaned up")
	}
}

func TestOIDCAuthorizationCodeStoresProviderErrorInTempContext(t *testing.T) {
	transaction := newStepTransaction()
	baseKey := env.GetContextKey("oidc." + transaction.CurrentStepID)
	transaction.State.GetClosedCtx().Set(baseKey+".state", "expected-state")
	transaction.Request = types.NewHTTPRequestAccessor(httptest.NewRequest(http.MethodGet, "/callback?error=access_denied&error_description=nope&state=expected-state", nil), 0)

	outcome, err := (&journeysteps.OIDCAuthorizationCode{}).Execute(transaction, oidcConfig("https://provider.example/token"))
	if err != nil || outcome != "false" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
	errorKey := env.GetContextKey("oidc." + transaction.CurrentStepID + ".error")
	if got := transaction.State.GetTempCtx().Get(errorKey + ".code").AsStringOr(""); got != "access_denied" {
		t.Fatalf("temp error code=%q", got)
	}
	if transaction.State.GetClosedCtx().IsDefined(baseKey) {
		t.Fatal("internal oidc state was not cleaned up after provider error")
	}
}

func TestOIDCAuthorizationCodeBuildsSignedAuthorizationRequestObject(t *testing.T) {
	transaction := newStepTransaction()
	config := oidcConfig("https://provider.example/token")
	config.Set("use_jar", true)
	config.Set("private_key_jwk", oidcPrivateJWK(t))
	config.Set("client_auth_method", "none")

	outcome, err := (&journeysteps.OIDCAuthorizationCode{}).Execute(transaction, config)
	if err != nil || outcome != "" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
	inputs := transaction.ClientInputsBuilder.GetNewInputs()
	if len(inputs) != 1 {
		t.Fatalf("new client inputs=%d", len(inputs))
	}
	output := inputs[0].Output.(map[string]any)
	authorizeURL, err := url.Parse(output["authorize_url"].(string))
	if err != nil {
		t.Fatal(err)
	}
	query := authorizeURL.Query()
	requestObject := query.Get("request")
	if requestObject == "" {
		t.Fatalf("authorize URL did not include request object: %s", authorizeURL.RawQuery)
	}
	if query.Get("response_type") != "" || query.Get("state") != "" {
		t.Fatalf("classic authorization params leaked outside JAR: %s", authorizeURL.RawQuery)
	}
	_, payload := decodeCompactJWTPart(t, requestObject)
	if payload["client_id"] != "client-id" || payload["response_type"] != "code" || payload["redirect_uri"] != "https://app.example/callback" {
		t.Fatalf("bad JAR payload: %#v", payload)
	}
	if payload["state"] == "" || payload["nonce"] == "" || payload["code_challenge"] == "" {
		t.Fatalf("missing JAR security params: %#v", payload)
	}
}

func TestOIDCAuthorizationCodePushesPARWithBasicClientAuthentication(t *testing.T) {
	var receivedAuth, receivedBody string
	parServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedAuth = request.Header.Get("Authorization")
		data, _ := io.ReadAll(request.Body)
		receivedBody = string(data)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"request_uri":"urn:example:par:123","expires_in":90}`))
	}))
	defer parServer.Close()

	transaction := newStepTransaction()
	config := oidcConfig("https://provider.example/token")
	config.Set("par_url", parServer.URL)

	outcome, err := (&journeysteps.OIDCAuthorizationCode{}).Execute(transaction, config)
	if err != nil || outcome != "" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
	expectedAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("client-id:client-secret"))
	if receivedAuth != expectedAuth {
		t.Fatalf("PAR authorization header=%q", receivedAuth)
	}
	if values, err := url.ParseQuery(receivedBody); err != nil || values.Get("response_type") != "code" || values.Get("state") == "" {
		t.Fatalf("bad PAR body=%q err=%v", receivedBody, err)
	}
	inputs := transaction.ClientInputsBuilder.GetNewInputs()
	output := inputs[0].Output.(map[string]any)
	authorizeURL, _ := url.Parse(output["authorize_url"].(string))
	if got := authorizeURL.Query().Get("request_uri"); got != "urn:example:par:123" {
		t.Fatalf("authorize request_uri=%q", got)
	}
	if authorizeURL.Query().Get("state") != "" || authorizeURL.Query().Get("request") != "" {
		t.Fatalf("authorize URL should only carry request_uri/client_id after PAR: %s", authorizeURL.RawQuery)
	}
}

func TestOIDCAuthorizationCodeUsesPrivateKeyJWTForTokenExchange(t *testing.T) {
	var receivedBody string
	tokenServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		data, _ := io.ReadAll(request.Body)
		receivedBody = string(data)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"access_token":"access","token_type":"Bearer"}`))
	}))
	defer tokenServer.Close()

	transaction := newStepTransaction()
	baseKey := env.GetContextKey("oidc." + transaction.CurrentStepID)
	transaction.State.GetClosedCtx().Set(baseKey+".state", "expected-state")
	transaction.Request = types.NewHTTPRequestAccessor(httptest.NewRequest(http.MethodGet, "/callback?code=abc&state=expected-state", nil), 0)
	config := oidcConfig(tokenServer.URL)
	config.Set("client_secret", "")
	config.Set("client_auth_method", "private_key_jwt")
	config.Set("private_key_jwk", oidcPrivateJWK(t))

	outcome, err := (&journeysteps.OIDCAuthorizationCode{}).Execute(transaction, config)
	if err != nil || outcome != "true" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
	values, err := url.ParseQuery(receivedBody)
	if err != nil {
		t.Fatal(err)
	}
	if values.Get("client_assertion_type") != "urn:ietf:params:oauth:client-assertion-type:jwt-bearer" || values.Get("client_assertion") == "" {
		t.Fatalf("missing private_key_jwt assertion: %s", receivedBody)
	}
	_, assertion := decodeCompactJWTPart(t, values.Get("client_assertion"))
	if assertion["iss"] != "client-id" || assertion["sub"] != "client-id" {
		t.Fatalf("bad assertion subject: %#v", assertion)
	}
	if assertion["aud"] != tokenServer.URL {
		t.Fatalf("bad assertion audience: %#v", assertion["aud"])
	}
	if strings.Contains(receivedBody, "client_secret") {
		t.Fatalf("private_key_jwt request leaked client_secret: %s", receivedBody)
	}
}

func TestOIDCAuthorizationCodeUsesConfiguredHTTPClientInstance(t *testing.T) {
	client := &oidcRecordingHTTPClient{body: []byte(`{"request_uri":"urn:example:par:custom"}`)}
	cacheManager, _ := jcache.NewManager(&jcache.ManagerConfig{})
	if err := cacheManager.UpdateRuntimeCacheInstance(journeysteps.HTTPClientCacheKey, "oidc-client", client, 0); err != nil {
		t.Fatal(err)
	}
	transaction := newStepTransaction()
	transaction.CacheManager = cacheManager
	config := oidcConfig("https://provider.example/token")
	config.Set("par_url", "https://provider.example/par")
	config.Set("http_client", "oidc-client")

	outcome, err := (&journeysteps.OIDCAuthorizationCode{}).Execute(transaction, config)
	if err != nil || outcome != "" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
	if client.uri != "https://provider.example/par" || client.method != http.MethodPost {
		t.Fatalf("configured client was not used: method=%q uri=%q", client.method, client.uri)
	}
}

func TestOIDCAuthorizationCodeTLSClientAuthenticationUsesHTTPClientTransportOnly(t *testing.T) {
	client := &oidcRecordingHTTPClient{body: []byte(`{"access_token":"access","token_type":"Bearer"}`)}
	cacheManager, _ := jcache.NewManager(&jcache.ManagerConfig{})
	if err := cacheManager.UpdateRuntimeCacheInstance(journeysteps.HTTPClientCacheKey, "mtls-client", client, 0); err != nil {
		t.Fatal(err)
	}
	transaction := newStepTransaction()
	transaction.CacheManager = cacheManager
	baseKey := env.GetContextKey("oidc." + transaction.CurrentStepID)
	transaction.State.GetClosedCtx().Set(baseKey+".state", "expected-state")
	transaction.Request = types.NewHTTPRequestAccessor(httptest.NewRequest(http.MethodGet, "/callback?code=abc&state=expected-state", nil), 0)

	config := oidcConfig("https://provider.example/token")
	config.Set("client_auth_method", "self_signed_tls_client_auth")
	config.Set("http_client", "mtls-client")

	outcome, err := (&journeysteps.OIDCAuthorizationCode{}).Execute(transaction, config)
	if err != nil || outcome != "true" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
	if client.uri != "https://provider.example/token" {
		t.Fatalf("configured mTLS client was not used: %q", client.uri)
	}
	values, err := url.ParseQuery(string(client.requestBody))
	if err != nil {
		t.Fatal(err)
	}
	if values.Get("client_id") != "client-id" {
		t.Fatalf("tls auth must send client_id: %s", client.requestBody)
	}
	if values.Get("client_secret") != "" || values.Get("client_assertion") != "" || client.headers["Authorization"] != "" {
		t.Fatalf("tls auth leaked non-TLS credentials: headers=%v body=%s", client.headers, client.requestBody)
	}
}

func TestOIDCAuthorizationCodeTLSClientAuthenticationRequiresConfiguredHTTPClient(t *testing.T) {
	config := oidcConfig("https://provider.example/token")
	config.Set("client_auth_method", "tls_client_auth")

	err := (&journeysteps.OIDCAuthorizationCode{}).VerifyConfig("oidc", config)
	if err == nil || !strings.Contains(err.Error(), "http_client is required") {
		t.Fatalf("expected http_client validation error, got %v", err)
	}
}

func oidcConfig(tokenURL string) goutils.TreeMapImpl {
	return goutils.NewTreeMap(map[string]any{
		"authorize_url":      "https://provider.example/authorize",
		"token_url":          tokenURL,
		"client_id":          "client-id",
		"client_secret":      "client-secret",
		"redirect_uri":       "https://app.example/callback",
		"scope":              []any{"openid", "email"},
		"pkce":               true,
		"nonce":              true,
		"output":             "encCtx.oidc.tokens",
		"outcome":            map[string]any{"true": "ok", "false": "fail"},
		"extra_token_params": map[string]any{"audience": "api"},
	})
}

type oidcRecordingHTTPClient struct {
	method      string
	uri         string
	headers     map[string]string
	requestBody []byte
	body        []byte
}

func (client *oidcRecordingHTTPClient) Request(method, uri string, headers map[string]string, body []byte) (*goutils.Response, error) {
	client.method = method
	client.uri = uri
	client.headers = headers
	client.requestBody = append([]byte(nil), body...)
	return &goutils.Response{Status: http.StatusCreated, Headers: http.Header{}, Body: client.body}, nil
}

func (client *oidcRecordingHTTPClient) RequestWithContext(_ context.Context, method, uri string, headers map[string]string, body []byte) (*goutils.Response, error) {
	return client.Request(method, uri, headers, body)
}

func oidcPrivateJWK(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwkKey, err := jwk.FromRaw(key)
	if err != nil {
		t.Fatal(err)
	}
	if err := jwkKey.Set(jwk.KeyIDKey, "oidc-test-key"); err != nil {
		t.Fatal(err)
	}
	if err := jwkKey.Set(jwk.AlgorithmKey, "RS256"); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(jwkKey)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func decodeCompactJWTPart(t *testing.T, value string) (map[string]any, map[string]any) {
	t.Helper()
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		t.Fatalf("invalid compact jwt: %q", value)
	}
	decode := func(segment string) map[string]any {
		data, err := base64.RawURLEncoding.DecodeString(segment)
		if err != nil {
			t.Fatal(err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		return decoded
	}
	return decode(parts[0]), decode(parts[1])
}
