package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jws"
	jcache "github.com/nitsugaro/go-journey/cache"
	"github.com/nitsugaro/go-journey/inputs"
	journeysteps "github.com/nitsugaro/go-journey/steps"
	"github.com/nitsugaro/go-journey/types"
	"github.com/nitsugaro/go-nstore"
	goutils "github.com/nitsugaro/go-utils/v2"
)

func newJWTTransaction() *types.JourneyTransaction {
	state := types.NewJourneyState()
	cacheManager, _ := jcache.NewManager(&jcache.ManagerConfig{})
	return &types.JourneyTransaction{
		Journey:             &types.JourneyConfiguration{Metadata: &nstore.Metadata{ID: "journey"}},
		CurrentStepID:       "verify-jwt",
		State:               state,
		ClientInputsBuilder: inputs.NewClientInputBuilder(nil, state.GetCtx()),
		CacheManager:        cacheManager,
	}
}

func verifyJWTStep(t *testing.T) types.IStep {
	t.Helper()
	step := journeysteps.GetDefaultStepRegistry().GetStep(types.VerifyJWTStep)
	if step == nil {
		t.Fatal("VerifyJWT is not registered")
	}
	return step
}

type testJWKCache struct {
	values map[string][]byte
	sets   int
}

func (cache *testJWKCache) Get(uri string) ([]byte, bool) {
	value, ok := cache.values[uri]
	return append([]byte(nil), value...), ok
}

func (cache *testJWKCache) Set(uri string, value []byte) error {
	cache.sets++
	cache.values[uri] = append([]byte(nil), value...)
	return nil
}

type jwtHTTPClient struct {
	response *goutils.Response
	calls    int
}

func (client *jwtHTTPClient) Request(string, string, map[string]string, []byte) (*goutils.Response, error) {
	client.calls++
	return client.response, nil
}

func (client *jwtHTTPClient) RequestWithContext(context.Context, string, string, map[string]string, []byte) (*goutils.Response, error) {
	client.calls++
	return client.response, nil
}

func signedTestJWT(t *testing.T, secret []byte, claims map[string]any, kid string) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	options := []jws.SignOption{jws.WithKey(jwa.HS256, secret)}
	if kid != "" {
		headers := jws.NewHeaders()
		if err := headers.Set(jws.KeyIDKey, kid); err != nil {
			t.Fatal(err)
		}
		options = []jws.SignOption{jws.WithKey(jwa.HS256, secret, jws.WithProtectedHeaders(headers))}
	}
	signed, err := jws.Sign(payload, options...)
	if err != nil {
		t.Fatal(err)
	}
	return string(signed)
}

func signedTestJWTWithoutHeaderAlgorithm(t *testing.T, secret []byte, claims map[string]any, kid string) string {
	t.Helper()
	header := map[string]any{"typ": "JWT"}
	if kid != "" {
		header["kid"] = kid
	}
	headerData, err := json.Marshal(header)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	signingInput := base64.RawURLEncoding.EncodeToString(headerData) + "." + base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func TestVerifyJWTSecretClaimsTimesAndOutput(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	token := signedTestJWT(t, secret, map[string]any{
		"sub": "ada", "role": "admin", "iat": time.Now().Add(-time.Minute).Unix(), "exp": time.Now().Add(time.Minute).Unix(),
	}, "")
	transaction := newJWTTransaction()
	config := goutils.NewTreeMap(map[string]any{
		"mode": "plain-secret", "token": token, "secret": string(secret), "algorithm": "HS256",
		"validate_iat": true, "validate_exp": true, "required_claims": map[string]any{"role": "admin"},
		"context": "ctx", "output": "verified",
	})
	outcome, err := verifyJWTStep(t).Execute(transaction, config)
	if err != nil || outcome != "valid" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
	if transaction.State.GetCtx().Get("verified.payload.sub").AsStringOr("") != "ada" || transaction.State.GetCtx().Get("verified.signature").AsStringOr("") == "" {
		t.Fatal("decoded token data was not stored")
	}
	config.Set("required_claims.role", "user")
	if outcome, _ := verifyJWTStep(t).Execute(transaction, config); outcome != "invalid" {
		t.Fatalf("claim mismatch outcome=%q", outcome)
	}
	expired := signedTestJWT(t, secret, map[string]any{"exp": time.Now().Add(-time.Minute).Unix()}, "")
	config.Set("token", expired).Set("required_claims", map[string]any{})
	if outcome, _ := verifyJWTStep(t).Execute(transaction, config); outcome != "invalid" {
		t.Fatalf("expired outcome=%q", outcome)
	}
}

func TestVerifyJWTBase64URLSecret(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	token := signedTestJWT(t, secret, map[string]any{"sub": "ada"}, "")
	transaction := newJWTTransaction()
	config := goutils.NewTreeMap(map[string]any{
		"mode":      "base64url-secret",
		"token":     token,
		"secret":    base64.RawURLEncoding.EncodeToString(secret),
		"algorithm": "HS256",
	})
	outcome, err := verifyJWTStep(t).Execute(transaction, config)
	if err != nil || outcome != "valid" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
}

func TestVerifyJWTJWKSingleKeyDoesNotRequireKidOrHeaderAlgorithm(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	key, err := jwk.FromRaw(secret)
	if err != nil {
		t.Fatal(err)
	}
	_ = key.Set(jwk.AlgorithmKey, jwa.HS256)
	set := jwk.NewSet()
	_ = set.AddKey(key)
	jwks, _ := json.Marshal(set)
	transaction := newJWTTransaction()
	config := goutils.NewTreeMap(map[string]any{
		"mode": "jwk", "token": signedTestJWTWithoutHeaderAlgorithm(t, secret, map[string]any{"sub": "ada"}, ""), "jwk": string(jwks), "algorithm": "HS256",
	})
	if outcome, err := verifyJWTStep(t).Execute(transaction, config); err != nil || outcome != "valid" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
	config.Set("algorithm", "")
	if outcome, err := verifyJWTStep(t).Execute(transaction, config); err != nil || outcome != "valid" {
		t.Fatalf("key algorithm outcome=%q err=%v", outcome, err)
	}
	config.Set("token", signedTestJWTWithoutHeaderAlgorithm(t, secret, map[string]any{"sub": "ada"}, "missing-kid"))
	if outcome, err := verifyJWTStep(t).Execute(transaction, config); err != nil || outcome != "invalid" {
		t.Fatalf("kid mismatch outcome=%q err=%v", outcome, err)
	}
	rawKey, _ := json.Marshal(key)
	config.Set("token", signedTestJWTWithoutHeaderAlgorithm(t, secret, map[string]any{"sub": "ada"}, "")).Set("jwk", string(rawKey))
	if outcome, err := verifyJWTStep(t).Execute(transaction, config); err != nil || outcome != "invalid" {
		t.Fatalf("raw JWK outcome=%q err=%v", outcome, err)
	}
}

func TestVerifyJWTJWKURIUsesOptionalCache(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	key, err := jwk.FromRaw(secret)
	if err != nil {
		t.Fatal(err)
	}
	_ = key.Set(jwk.KeyIDKey, "key-1")
	_ = key.Set(jwk.AlgorithmKey, jwa.HS256)
	set := jwk.NewSet()
	_ = set.AddKey(key)
	jwks, _ := json.Marshal(set)
	client := &jwtHTTPClient{response: &goutils.Response{Status: http.StatusOK, Headers: http.Header{}, Body: jwks}}
	cache := &testJWKCache{values: map[string][]byte{}}
	transaction := newJWTTransaction()
	if err := transaction.CacheManager.UpdateRuntimeCacheInstance(journeysteps.HTTPClientCacheKey, jcache.DefaultInstanceID, client, 0); err != nil {
		t.Fatal(err)
	}
	if err := transaction.CacheManager.UpdateRuntimeCacheInstance(journeysteps.JWKCacheKey, jcache.DefaultInstanceID, cache, 0); err != nil {
		t.Fatal(err)
	}
	config := goutils.NewTreeMap(map[string]any{
		"mode": "jwk", "token": signedTestJWT(t, secret, map[string]any{"sub": "ada"}, "key-1"), "jwk_uri": "https://issuer.test/jwks", "algorithm": "HS256",
	})
	for invocation := 0; invocation < 2; invocation++ {
		if outcome, err := verifyJWTStep(t).Execute(transaction, config); err != nil || outcome != "valid" {
			t.Fatalf("invocation %d outcome=%q err=%v", invocation, outcome, err)
		}
	}
	config.Set("algorithm", "HS384")
	if outcome, err := verifyJWTStep(t).Execute(transaction, config); err != nil || outcome != "invalid" {
		t.Fatalf("algorithm mismatch outcome=%q err=%v", outcome, err)
	}
	if client.calls != 1 || cache.sets != 1 {
		t.Fatalf("HTTP calls=%d cache sets=%d", client.calls, cache.sets)
	}
}

func TestVerifyJWTJWKURIRequiresCache(t *testing.T) {
	transaction := newJWTTransaction()
	transaction.CacheManager = nil
	config := goutils.NewTreeMap(map[string]any{
		"mode": "jwk", "token": signedTestJWT(t, []byte("01234567890123456789012345678901"), map[string]any{"sub": "ada"}, ""), "jwk_uri": "https://issuer.test/jwks", "algorithm": "HS256",
	})
	if outcome, err := verifyJWTStep(t).Execute(transaction, config); err != nil || outcome != "invalid" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
}

func TestVerifyJWTJWKMultipleKeysRequireTokenKid(t *testing.T) {
	firstSecret := []byte("01234567890123456789012345678901")
	secondSecret := []byte("abcdefghijklmnopqrstuvwxyz123456")
	firstKey, err := jwk.FromRaw(firstSecret)
	if err != nil {
		t.Fatal(err)
	}
	_ = firstKey.Set(jwk.KeyIDKey, "key-1")
	_ = firstKey.Set(jwk.AlgorithmKey, jwa.HS256)
	secondKey, err := jwk.FromRaw(secondSecret)
	if err != nil {
		t.Fatal(err)
	}
	_ = secondKey.Set(jwk.KeyIDKey, "key-2")
	_ = secondKey.Set(jwk.AlgorithmKey, jwa.HS256)
	set := jwk.NewSet()
	_ = set.AddKey(firstKey)
	_ = set.AddKey(secondKey)
	jwks, _ := json.Marshal(set)
	transaction := newJWTTransaction()
	config := goutils.NewTreeMap(map[string]any{
		"mode": "jwk", "token": signedTestJWT(t, secondSecret, map[string]any{"sub": "ada"}, "key-2"), "jwk": string(jwks), "algorithm": "HS256",
	})
	if outcome, err := verifyJWTStep(t).Execute(transaction, config); err != nil || outcome != "valid" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
	config.Set("token", signedTestJWT(t, secondSecret, map[string]any{"sub": "ada"}, ""))
	if outcome, err := verifyJWTStep(t).Execute(transaction, config); err != nil || outcome != "invalid" {
		t.Fatalf("missing kid outcome=%q err=%v", outcome, err)
	}
	_ = secondKey.Remove(jwk.KeyIDKey)
	jwks, _ = json.Marshal(set)
	config.Set("jwk", string(jwks)).Set("token", signedTestJWT(t, secondSecret, map[string]any{"sub": "ada"}, "key-2"))
	if outcome, err := verifyJWTStep(t).Execute(transaction, config); err != nil || outcome != "invalid" {
		t.Fatalf("missing key kid outcome=%q err=%v", outcome, err)
	}
}

func TestVerifyJWTIntrospection(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	token := signedTestJWT(t, secret, map[string]any{"sub": "ada"}, "")
	body, _ := json.Marshal(map[string]any{"active": true, "sub": "ada"})
	client := &jwtHTTPClient{response: &goutils.Response{Status: http.StatusOK, Headers: http.Header{}, Body: body}}
	transaction := newJWTTransaction()
	if err := transaction.CacheManager.UpdateRuntimeCacheInstance(journeysteps.HTTPClientCacheKey, jcache.DefaultInstanceID, client, 0); err != nil {
		t.Fatal(err)
	}
	config := goutils.NewTreeMap(map[string]any{
		"mode": "introspection", "token": token, "endpoint": "https://issuer.test/introspect", "required_claims": map[string]any{"sub": "ada"},
	})
	if outcome, err := verifyJWTStep(t).Execute(transaction, config); err != nil || outcome != "valid" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
}

func TestVerifyJWTConfigModes(t *testing.T) {
	tests := []map[string]any{
		{"mode": "plain-secret", "secret": "x", "algorithm": "HS256"},
		{"mode": "base64url-secret", "secret": "eA", "algorithm": "HS256"},
		{"mode": "jwk", "jwk_uri": "https://issuer.test/jwks", "algorithm": "RS256"},
		{"mode": "userinfo", "endpoint": "https://issuer.test/userinfo"},
	}
	for _, config := range tests {
		if err := verifyJWTStep(t).VerifyConfig("jwt", goutils.NewTreeMap(config)); err != nil {
			t.Fatal(err)
		}
	}
	if types.VerifyJWTStep != verifyJWTStep(t).GetStepType() {
		t.Fatal("unexpected step type")
	}
}
