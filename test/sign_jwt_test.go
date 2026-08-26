package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
	journeysteps "github.com/nitsugaro/go-journey/steps"
	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

func signJWTStep(t *testing.T) types.IStep {
	t.Helper()
	step := journeysteps.GetDefaultStepRegistry().GetStep(types.SignJWTStep)
	if step == nil {
		t.Fatal("SignJWT is not registered")
	}
	return step
}

func TestSignJWTSecretClaimsHeadersTimesAndOutput(t *testing.T) {
	secret := "literal-secret!"
	transaction := newJWTTransaction()
	config := goutils.NewTreeMap(map[string]any{
		"algorithm":          "HS256",
		"key":                map[string]any{"type": "plain-secret", "value": secret, "kid": "secret-key"},
		"claims":             map[string]any{"role": "admin", "nested": map[string]any{"ok": true}},
		"headers":            map[string]any{"alg": "HS512", "cty": "jwt"},
		"issuer":             "go-journey",
		"subject":            "ada",
		"set_iat":            true,
		"set_jti":            true,
		"expires_in_seconds": 60,
		"context":            "ctx",
		"output":             "signed",
	})

	outcome, err := signJWTStep(t).Execute(transaction, config)
	if err != nil || outcome != "true" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
	tokenText := transaction.State.GetCtx().Get("signed").AsStringOr("")
	parsed, err := jwt.ParseString(tokenText, jwt.WithKey(jwa.HS256, []byte(secret)), jwt.WithValidate(false))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Subject() != "ada" || parsed.Issuer() != "go-journey" || parsed.JwtID() == "" {
		t.Fatalf("registered claims not set: sub=%q iss=%q jti=%q", parsed.Subject(), parsed.Issuer(), parsed.JwtID())
	}
	if _, err := uuid.Parse(parsed.JwtID()); err != nil {
		t.Fatalf("jti is not a UUID: %q", parsed.JwtID())
	}
	if role, _ := parsed.Get("role"); role != "admin" {
		t.Fatalf("role claim=%v", role)
	}
	if parsed.IssuedAt().IsZero() || time.Until(parsed.Expiration()) <= 0 {
		t.Fatal("iat or exp was not set")
	}
	header, _, _, err := journeystepsDecodeJWT(tokenText)
	if err != nil {
		t.Fatal(err)
	}
	if header["alg"] != "HS256" || header["kid"] != "secret-key" || header["cty"] != "jwt" {
		t.Fatalf("unexpected header: %#v", header)
	}
}

func TestSignJWTBase64URLSecret(t *testing.T) {
	secret := []byte("literal-secret!")
	transaction := newJWTTransaction()
	config := goutils.NewTreeMap(map[string]any{
		"algorithm": "HS256",
		"key":       map[string]any{"type": "base64url-secret", "value": base64.RawURLEncoding.EncodeToString(secret)},
		"claims":    map[string]any{"sub": "ada"},
		"context":   "ctx",
		"output":    "signed",
	})

	outcome, err := signJWTStep(t).Execute(transaction, config)
	if err != nil || outcome != "true" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
	tokenText := transaction.State.GetCtx().Get("signed").AsStringOr("")
	parsed, err := jwt.ParseString(tokenText, jwt.WithKey(jwa.HS256, secret), jwt.WithValidate(false))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Subject() != "ada" {
		t.Fatalf("subject=%q", parsed.Subject())
	}
}

func TestSignJWTPemAndPrivateJWK(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pemKey := mustPrivateKeyPEM(t, privateKey)
	for name, keyConfig := range map[string]map[string]any{
		"pem": {"type": "pem", "value": string(pemKey), "kid": "rsa-pem"},
		"jwk": {"type": "jwk", "value": mustPrivateJWK(t, privateKey), "kid": "rsa-jwk"},
	} {
		t.Run(name, func(t *testing.T) {
			transaction := newJWTTransaction()
			config := goutils.NewTreeMap(map[string]any{
				"algorithm": "RS256",
				"key":       keyConfig,
				"claims":    map[string]any{"sub": name},
				"set_iat":   false,
				"context":   "closedCtx",
				"output":    "token",
			})
			outcome, err := signJWTStep(t).Execute(transaction, config)
			if err != nil || outcome != "true" {
				t.Fatalf("outcome=%q err=%v", outcome, err)
			}
			tokenText := transaction.State.GetClosedCtx().Get("token").AsStringOr("")
			parsed, err := jwt.ParseString(tokenText, jwt.WithKey(jwa.RS256, &privateKey.PublicKey), jwt.WithValidate(false))
			if err != nil {
				t.Fatal(err)
			}
			if parsed.Subject() != name {
				t.Fatalf("subject=%q", parsed.Subject())
			}
			if !parsed.IssuedAt().IsZero() {
				t.Fatal("iat should not be set when set_iat is false")
			}
		})
	}
}

func TestSignJWTConfigValidation(t *testing.T) {
	step := signJWTStep(t)
	valid := goutils.NewTreeMap(map[string]any{
		"algorithm": "HS256",
		"key":       map[string]any{"type": "plain-secret", "value": "secret"},
		"claims":    map[string]any{"sub": "ada"},
		"output":    "token",
	})
	if err := step.VerifyConfig("sign", valid); err != nil {
		t.Fatal(err)
	}
	invalid := goutils.NewTreeMap(map[string]any{
		"algorithm": "RS256",
		"key":       map[string]any{"type": "plain-secret", "value": "secret"},
		"claims":    map[string]any{"sub": "ada"},
		"output":    "token",
	})
	if err := step.VerifyConfig("sign", invalid); err == nil {
		t.Fatal("expected secret + RS256 to be invalid")
	}
	if types.SignJWTStep != step.GetStepType() {
		t.Fatalf("step type=%q", step.GetStepType())
	}
}

func mustPrivateKeyPEM(t *testing.T, privateKey *rsa.PrivateKey) []byte {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}

func mustPrivateJWK(t *testing.T, privateKey *rsa.PrivateKey) string {
	t.Helper()
	key, err := jwk.FromRaw(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := key.Set(jwk.AlgorithmKey, jwa.RS256); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(key)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func journeystepsDecodeJWT(value string) (map[string]any, map[string]any, string, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return nil, nil, "", fmt.Errorf("invalid compact JWT")
	}
	var header map[string]any
	headerData, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, nil, "", err
	}
	if err := json.Unmarshal(headerData, &header); err != nil {
		return nil, nil, "", err
	}
	return header, nil, parts[2], nil
}
