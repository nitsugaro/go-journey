package steps

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
	jcache "github.com/nitsugaro/go-journey/cache"
	"github.com/nitsugaro/go-journey/types"
	jwtek "github.com/nitsugaro/go-jwte-manager/v2"
	"github.com/nitsugaro/go-nstore"
	goutils "github.com/nitsugaro/go-utils/v2"
	"github.com/nitsugaro/go-utils/v2/crypto"
	"github.com/nitsugaro/go-utils/v2/encoding"
)

const JWKCacheKey = "jwk_cache"

const (
	verifyJWTModePlainSecret     = "plain-secret"
	verifyJWTModeBase64URLSecret = "base64url-secret"
	verifyJWTModeJWK             = "jwk"
	verifyJWTModeIntrospection   = "introspection"
	verifyJWTModeUserinfo        = "userinfo"
)

type JWKCache interface {
	Get(uri string) ([]byte, bool)
	Set(uri string, value []byte) error
}

type jwteManagerJWKCache struct{}

type VerifyJWT struct {
	BasicStep

	_              struct{}       `description:"Verifies a JWT locally or through an OAuth endpoint and optionally stores its decoded representation."`
	Mode           string         `json:"mode" enum:"plain-secret,base64url-secret,jwk,introspection,userinfo" required:"true"`
	Token          string         `json:"token" required:"true" minLength:"1"`
	Secret         string         `json:"secret,omitempty"`
	Algorithm      string         `json:"algorithm,omitempty" enum:"HS256,HS384,HS512,RS256,RS384,RS512,PS256,PS384,PS512,ES256,ES256K,ES384,ES512,EdDSA"`
	JWK            any            `json:"jwk,omitempty"`
	JWKURI         string         `json:"jwk_uri,omitempty"`
	Endpoint       string         `json:"endpoint,omitempty"`
	ValidateIAT    bool           `json:"validate_iat" default:"true"`
	ValidateEXP    bool           `json:"validate_exp" default:"true"`
	RequiredClaims map[string]any `json:"required_claims,omitempty"`
	Output         string         `json:"output,omitempty"`
	Context        string         `json:"context" enum:"ctx,encCtx,closedCtx,tempCtx" default:"ctx"`
	Outcome        struct {
		Valid   string `json:"valid" required:"true" format:"uuid"`
		Invalid string `json:"invalid" required:"true" format:"uuid"`
		Error   string `json:"error" required:"true" format:"uuid"`
	} `json:"outcome" required:"true"`
}

func (*VerifyJWT) GetStepType() string { return types.VerifyJWTStep }

func (*VerifyJWT) VerifyConfig(stepName string, config goutils.TreeMapImpl) error {
	mode := config.Get("mode").AsStringOr("")
	if strings.Contains(mode, "${") {
		return nil
	}
	switch mode {
	case verifyJWTModePlainSecret, verifyJWTModeBase64URLSecret:
		if config.Get("secret").AsStringOr("") == "" || config.Get("algorithm").AsStringOr("") == "" {
			return types.StepInvalidConfig(stepName, mode+" mode requires secret and algorithm")
		}
		if _, err := symmetricJWTAlgorithm(config.Get("algorithm").AsStringOr("")); err != nil {
			return types.StepInvalidConfig(stepName, err.Error())
		}
	case verifyJWTModeJWK:
		if !config.IsDefined("jwk") == (config.Get("jwk_uri").AsStringOr("") == "") {
			return types.StepInvalidConfig(stepName, "jwk mode requires exactly one of jwk or jwk_uri")
		}
		if algorithm := config.Get("algorithm").AsStringOr(""); algorithm != "" && !strings.Contains(algorithm, "${") {
			if _, err := jwtSignatureAlgorithm(algorithm); err != nil {
				return types.StepInvalidConfig(stepName, err.Error())
			}
		}
	case verifyJWTModeIntrospection, verifyJWTModeUserinfo:
		if config.Get("endpoint").AsStringOr("") == "" {
			return types.StepInvalidConfig(stepName, mode+" mode requires endpoint")
		}
	default:
		return types.StepInvalidConfig(stepName, "unsupported JWT verification mode")
	}
	return nil
}

func (step *VerifyJWT) Execute(transaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	tokenText := config.Get("token").AsStringOr("")
	header, payload, signature, err := decodeJWT(tokenText)
	if err != nil {
		return "invalid", nil
	}

	mode := config.Get("mode").AsStringOr("")
	switch mode {
	case verifyJWTModePlainSecret, verifyJWTModeBase64URLSecret:
		err = verifyJWTSecret(tokenText, mode, config.Get("secret").AsStringOr(""), config.Get("algorithm").AsStringOr(""))
	case verifyJWTModeJWK:
		var set jwk.Set
		set, err = step.jwkSet(transaction, config)
		if err == nil {
			err = verifyJWTJWK(tokenText, header, set, config.Get("algorithm").AsStringOr(""))
		}
	case verifyJWTModeIntrospection, verifyJWTModeUserinfo:
		payload, err = step.verifyRemote(transaction, config, mode, tokenText)
	default:
		return "error", nil
	}
	if err != nil {
		if mode == verifyJWTModeIntrospection || mode == verifyJWTModeUserinfo {
			return "error", nil
		}
		return "invalid", nil
	}
	if !validateJWTTimes(payload, config.Get("validate_iat").AsBoolOr(true), config.Get("validate_exp").AsBoolOr(true), time.Now()) || !claimsEqual(payload, config.Get("required_claims")) {
		return "invalid", nil
	}
	if mode == verifyJWTModeIntrospection && payload["active"] != true {
		return "invalid", nil
	}
	if output := config.Get("output").AsStringOr(""); output != "" {
		ctx := transaction.State.Get(config.Get("context").AsStringOr(types.CtxKey))
		if ctx == nil {
			return "error", nil
		}
		ctx.Set(output, map[string]any{"header": header, "payload": payload, "signature": signature})
	}
	return "valid", nil
}

func verifyJWTSecret(tokenText, mode, secret, algorithm string) error {
	alg, err := symmetricJWTAlgorithm(algorithm)
	if err != nil {
		return err
	}

	secretBytes := []byte(secret)
	if mode == verifyJWTModeBase64URLSecret {
		secretBytes, err = encoding.DecodeBase64URL(secret)
		if err != nil {
			return err
		}
	} else if mode != verifyJWTModePlainSecret {
		return errors.New("unsupported secret verification mode")
	}

	_, err = jwt.ParseString(tokenText, jwt.WithKey(alg, secretBytes), jwt.WithValidate(false))
	return err
}

func jwtSignatureAlgorithm(algorithm string) (jwa.SignatureAlgorithm, error) {
	var alg jwa.SignatureAlgorithm
	if err := alg.Accept(algorithm); err != nil || alg == jwa.NoSignature {
		return "", errors.New("invalid JWT signature algorithm")
	}
	return alg, nil
}

func symmetricJWTAlgorithm(algorithm string) (jwa.SignatureAlgorithm, error) {
	alg, err := jwtSignatureAlgorithm(algorithm)
	if err != nil || !alg.IsSymmetric() {
		return "", errors.New("invalid symmetric signature algorithm")
	}
	return alg, nil
}

func (step *VerifyJWT) jwkSet(transaction *types.JourneyTransaction, config goutils.TreeMapImpl) (jwk.Set, error) {
	var data []byte
	if uri := config.Get("jwk_uri").AsStringOr(""); uri != "" {
		cache := transactionJWKCache(transaction)
		if cache == nil {
			return nil, errors.New("jwk_uri mode requires a JWK cache")
		}
		if cached, found := cache.Get(uri); found {
			return parseJWKSet(cached)
		}
		client := transactionHTTPClientInstance(transaction, "")
		res, err := requestHTTP(transaction, client, http.MethodGet, uri, map[string]string{"Accept": "application/json"}, nil)
		if err != nil || res.Status < 200 || res.Status >= 300 {
			return nil, fmt.Errorf("fetch jwk_uri")
		}
		data = res.Body
		if err := cache.Set(uri, append([]byte(nil), data...)); err != nil {
			return nil, err
		}
	} else {
		raw := config.Get("jwk").AsAnyOr(nil)
		if text, ok := raw.(string); ok {
			data = []byte(text)
		} else {
			data, _ = json.Marshal(raw)
		}
	}
	return parseJWKSet(data)
}

func parseJWKSet(data []byte) (jwk.Set, error) {
	var document map[string]json.RawMessage
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	if _, ok := document["keys"]; !ok {
		return nil, errors.New("jwk mode requires JWKS format with keys")
	}
	set, err := jwk.Parse(data)
	if err != nil {
		return nil, err
	}
	if set.Len() == 0 {
		return nil, errors.New("jwk mode requires JWKS with at least one key")
	}
	return set, nil
}

func verifyJWTJWK(tokenText string, header map[string]any, set jwk.Set, configuredAlgorithm string) error {
	key, err := selectJWTJWK(header, set)
	if err != nil {
		return err
	}
	alg, err := jwkVerifierAlgorithm(key, configuredAlgorithm)
	if err != nil {
		return err
	}
	var raw any
	if err := key.Raw(&raw); err != nil {
		return err
	}
	_, err = jwt.ParseString(tokenText, jwt.WithKey(alg, raw), jwt.WithValidate(false))
	return err
}

func selectJWTJWK(header map[string]any, set jwk.Set) (jwk.Key, error) {
	if set == nil || set.Len() == 0 {
		return nil, errors.New("jwk mode requires JWKS with at least one key")
	}
	tokenKid := fmt.Sprint(header["kid"])
	if tokenKid == "<nil>" {
		tokenKid = ""
	}
	if set.Len() == 1 {
		key, ok := set.Key(0)
		if !ok {
			return nil, errors.New("JWK not found")
		}
		if tokenKid != "" && jwkString(key, jwk.KeyIDKey) != tokenKid {
			return nil, errors.New("JWT kid does not match JWK")
		}
		return key, nil
	}
	for iterator := set.Keys(context.Background()); iterator.Next(context.Background()); {
		pair := iterator.Pair()
		key, ok := pair.Value.(jwk.Key)
		if !ok {
			continue
		}
		if jwkString(key, jwk.KeyIDKey) == "" {
			return nil, errors.New("multiple JWK keys require kid")
		}
	}
	if tokenKid == "" {
		return nil, errors.New("JWT kid is required when JWKS has multiple keys")
	}
	key, ok := set.LookupKeyID(tokenKid)
	if !ok {
		return nil, errors.New("JWK kid not found")
	}
	return key, nil
}

func jwkVerifierAlgorithm(key jwk.Key, configuredAlgorithm string) (jwa.SignatureAlgorithm, error) {
	keyAlgorithm := jwkString(key, jwk.AlgorithmKey)
	if configuredAlgorithm != "" {
		alg, err := jwtSignatureAlgorithm(configuredAlgorithm)
		if err != nil {
			return "", err
		}
		if keyAlgorithm != "" && keyAlgorithm != alg.String() {
			return "", errors.New("configured JWT algorithm does not match JWK algorithm")
		}
		return alg, nil
	}
	if keyAlgorithm == "" {
		return "", errors.New("JWK algorithm is required when verify_jwt algorithm is empty")
	}
	return jwtSignatureAlgorithm(keyAlgorithm)
}

func jwkString(key jwk.Key, field string) string {
	value, ok := key.Get(field)
	if !ok || value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func (step *VerifyJWT) verifyRemote(transaction *types.JourneyTransaction, config goutils.TreeMapImpl, mode, tokenText string) (map[string]any, error) {
	endpoint := config.Get("endpoint").AsStringOr("")
	method, headers, body := http.MethodGet, map[string]string{"Authorization": "Bearer " + tokenText, "Accept": "application/json"}, []byte(nil)
	if mode == "introspection" {
		method = http.MethodPost
		headers["Content-Type"] = "application/x-www-form-urlencoded"
		body = []byte(url.Values{"token": []string{tokenText}}.Encode())
	}
	res, err := requestHTTP(transaction, transactionHTTPClientInstance(transaction, ""), method, endpoint, headers, body)
	if err != nil || res.Status < 200 || res.Status >= 300 {
		return nil, fmt.Errorf("remote JWT verification failed")
	}
	var payload map[string]any
	if err := json.Unmarshal(res.Body, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func transactionHTTPClientInstance(transaction *types.JourneyTransaction, instanceID string) HTTPClient {
	if strings.TrimSpace(instanceID) == "" {
		instanceID = jcache.DefaultInstanceID
	}
	if transaction.CacheManager != nil {
		if instance, ok := transaction.CacheManager.GetCacheInstance(HTTPClientCacheKey, instanceID); ok {
			if client, valid := instance.(HTTPClient); valid {
				return client
			}
		}
	}
	return Client
}

func transactionJWKCache(transaction *types.JourneyTransaction) JWKCache {
	if transaction.CacheManager != nil {
		instance, ok := transaction.CacheManager.GetCacheInstance(JWKCacheKey, jcache.DefaultInstanceID)
		if ok {
			cache, _ := instance.(JWKCache)
			if cache != nil {
				return cache
			}
		}
	}
	if jwtek.GetExternalJwkStorage() == nil {
		return nil
	}
	return jwteManagerJWKCache{}
}

func (jwteManagerJWKCache) Get(uri string) ([]byte, bool) {
	storage := jwtek.GetExternalJwkStorage()
	if storage == nil {
		return nil, false
	}
	jwks, err := storage.Load(jwteJWKURIID(uri))
	if err != nil || time.Now().Unix()-jwks.Iat >= storage.CacheSeconds {
		return nil, false
	}
	data, err := json.Marshal(map[string]any{"keys": jwks.Jwks})
	if err != nil {
		return nil, false
	}
	return data, true
}

func (jwteManagerJWKCache) Set(uri string, value []byte) error {
	storage := jwtek.GetExternalJwkStorage()
	if storage == nil {
		return nil
	}
	var response jwtek.ResponseJwks
	if err := json.Unmarshal(value, &response); err != nil {
		return err
	}
	return storage.Save(&jwtek.Jwks{
		Metadata: &nstore.Metadata{ID: jwteJWKURIID(uri)},
		Jwks:     response.Keys,
		Iat:      time.Now().Unix(),
	})
}

func jwteJWKURIID(uri string) string {
	return encoding.EncodeBase64URL(crypto.HashSHA1(uri))
}

func requestHTTP(transaction *types.JourneyTransaction, client HTTPClient, method, uri string, headers map[string]string, body []byte) (*goutils.Response, error) {
	if client == nil {
		return nil, errors.New("HTTP client is not configured")
	}
	if transaction.Context != nil {
		return client.RequestWithContext(transaction.Context, method, uri, headers, body)
	}
	return client.Request(method, uri, headers, body)
}

func decodeJWT(value string) (map[string]any, map[string]any, string, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 3 || parts[2] == "" {
		return nil, nil, "", errors.New("invalid compact JWT")
	}
	decode := func(encoded string) (map[string]any, error) {
		data, err := base64.RawURLEncoding.DecodeString(encoded)
		if err != nil {
			return nil, err
		}
		var object map[string]any
		err = json.Unmarshal(data, &object)
		return object, err
	}
	header, err := decode(parts[0])
	if err != nil {
		return nil, nil, "", err
	}
	payload, err := decode(parts[1])
	return header, payload, parts[2], err
}

func validateJWTTimes(payload map[string]any, validateIAT, validateEXP bool, now time.Time) bool {
	if validateIAT {
		if value, exists := payload["iat"]; exists {
			iat, ok := value.(float64)
			if !ok || iat > float64(now.Unix()) {
				return false
			}
		}
	}
	if validateEXP {
		if value, exists := payload["exp"]; exists {
			exp, ok := value.(float64)
			if !ok || exp <= float64(now.Unix()) {
				return false
			}
		}
	}
	return true
}

func claimsEqual(payload map[string]any, configured goutils.TreeMapImpl) bool {
	required, err := configured.AsMap()
	if err != nil {
		return true
	}
	tree := goutils.NewTreeMap(payload)
	for claim, expected := range required {
		if !reflect.DeepEqual(tree.Get(claim).AsAnyOr(nil), expected) {
			return false
		}
	}
	return true
}

func init() {
	defaultStepRegistry.AddStep(&VerifyJWT{}, map[string]map[string]any{
		".":     {"x-category": types.AuthCategory, "x-order": []string{"mode", "token", "secret", "algorithm", "jwk", "jwk_uri", "endpoint", "validate_iat", "validate_exp", "required_claims", "context", "output", "outcome"}},
		"token": {"x-type": "scriptable"}, "secret": {"x-type": "scriptable"}, "jwk_uri": {"x-type": "scriptable"}, "endpoint": {"x-type": "scriptable"},
	})
}
