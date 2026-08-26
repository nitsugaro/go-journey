package steps

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jws"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/nitsugaro/go-journey/types"
	"github.com/nitsugaro/go-utils/encoding"
	goutils "github.com/nitsugaro/go-utils/v2"
)

const (
	signJWTKeyTypePlainSecret     = "plain-secret"
	signJWTKeyTypeBase64URLSecret = "base64url-secret"
	signJWTKeyTypePEM             = "pem"
	signJWTKeyTypeJWK             = "jwk"
)

type SignJWT struct {
	BasicStep

	_         struct{}       `description:"Signs a JWT using a plain secret, base64url secret, private PEM key or private JWK, and saves the compact token into context."`
	Algorithm string         `json:"algorithm" enum:"HS256,HS384,HS512,RS256,RS384,RS512,PS256,PS384,PS512,ES256,ES256K,ES384,ES512,EdDSA" required:"true"`
	Key       SignJWTKey     `json:"key" required:"true"`
	Claims    map[string]any `json:"claims" required:"true"`
	Headers   map[string]any `json:"headers,omitempty"`

	Issuer   string   `json:"issuer,omitempty"`
	Subject  string   `json:"subject,omitempty"`
	Audience []string `json:"audience,omitempty"`

	SetIAT           bool   `json:"set_iat" default:"true"`
	SetJTI           bool   `json:"set_jti" default:"false"`
	ExpiresInSeconds int    `json:"expires_in_seconds,omitempty" default:"0"`
	Output           string `json:"output" required:"true" minLength:"1"`
	Context          string `json:"context" enum:"ctx,encCtx,closedCtx,tempCtx" default:"ctx"`
	Outcome          struct {
		True  string `json:"true" required:"true" format:"uuid"`
		Error string `json:"error" required:"true" format:"uuid"`
	} `json:"outcome" required:"true"`
}

type SignJWTKey struct {
	Type  string `json:"type" enum:"plain-secret,base64url-secret,pem,jwk" required:"true"`
	Value any    `json:"value" required:"true"`
	KID   string `json:"kid,omitempty"`
}

func (*SignJWT) GetStepType() string { return types.SignJWTStep }

func (*SignJWT) VerifyConfig(stepName string, config goutils.TreeMapImpl) error {
	algorithm := config.Get("algorithm").AsStringOr("")
	if algorithm == "" {
		return types.StepInvalidConfig(stepName, "algorithm is required")
	}
	if strings.Contains(algorithm, "${") {
		return nil
	}
	alg, err := jwtSignatureAlgorithm(algorithm)
	if err != nil {
		return types.StepInvalidConfig(stepName, err.Error())
	}

	keyType := config.Get("key.type").AsStringOr("")
	if strings.Contains(keyType, "${") {
		return nil
	}
	switch keyType {
	case signJWTKeyTypePlainSecret, signJWTKeyTypeBase64URLSecret:
		if !alg.IsSymmetric() {
			return types.StepInvalidConfig(stepName, "secret key type requires HS* algorithm")
		}
	case signJWTKeyTypePEM, signJWTKeyTypeJWK:
		if alg.IsSymmetric() {
			return types.StepInvalidConfig(stepName, keyType+" key type cannot use HS* algorithm")
		}
	default:
		return types.StepInvalidConfig(stepName, "unsupported JWT signing key type")
	}
	if config.Get("key.value").IsEmpty() {
		return types.StepInvalidConfig(stepName, "key.value is required")
	}
	if config.Get("claims").IsEmpty() {
		return types.StepInvalidConfig(stepName, "claims are required")
	}
	if config.Get("output").AsStringOr("") == "" {
		return types.StepInvalidConfig(stepName, "output is required")
	}
	return nil
}

func (step *SignJWT) Execute(transaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	signed, err := signJWTCompactFromConfig(config, time.Now())
	if err != nil {
		return "error", nil
	}
	ctx := transaction.State.Get(config.Get("context").AsStringOr(types.CtxKey))
	if ctx == nil {
		return "error", nil
	}
	ctx.Set(config.Get("output").AsStringOr(""), signed)
	return "true", nil
}

type JWTSignOptions struct {
	Algorithm string
	Key       SignJWTKey
	Claims    map[string]any
	Headers   map[string]any
	Issuer    string
	Subject   string
	Audience  []string
	SetIAT    bool
	SetJTI    bool
	ExpiresIn int
	Now       time.Time
}

func SignCompactJWT(options JWTSignOptions) (string, error) {
	now := options.Now
	if now.IsZero() {
		now = time.Now()
	}

	claims := map[string]any{
		"algorithm":          options.Algorithm,
		"key":                map[string]any{"type": options.Key.Type, "value": options.Key.Value, "kid": options.Key.KID},
		"claims":             options.Claims,
		"headers":            options.Headers,
		"issuer":             options.Issuer,
		"subject":            options.Subject,
		"set_iat":            options.SetIAT,
		"set_jti":            options.SetJTI,
		"expires_in_seconds": options.ExpiresIn,
	}

	if len(options.Audience) == 1 {
		claims["audience"] = options.Audience[0]
	} else {
		claims["audience"] = options.Audience
	}

	config := goutils.NewTreeMap(claims)

	return signJWTCompactFromConfig(config, now)
}

func signJWTCompactFromConfig(config goutils.TreeMapImpl, now time.Time) (string, error) {
	token, err := buildSignJWTToken(config, now)
	if err != nil {
		return "", err
	}
	alg, key, err := signJWTKey(config)
	if err != nil {
		return "", err
	}
	headers, err := signJWTHeaders(config)
	if err != nil {
		return "", err
	}
	signed, err := jwt.Sign(token, jwt.WithKey(alg, key, jws.WithProtectedHeaders(headers)))
	if err != nil {
		return "", err
	}
	return string(signed), nil
}

func buildSignJWTToken(config goutils.TreeMapImpl, now time.Time) (jwt.Token, error) {
	claims, err := config.Get("claims").AsMap()
	if err != nil {
		return nil, err
	}
	token := jwt.New()
	for key, value := range claims {
		if err := token.Set(key, value); err != nil {
			return nil, err
		}
	}
	if issuer := config.Get("issuer").AsStringOr(""); issuer != "" {
		if err := token.Set(jwt.IssuerKey, issuer); err != nil {
			return nil, err
		}
	}
	if subject := config.Get("subject").AsStringOr(""); subject != "" {
		if err := token.Set(jwt.SubjectKey, subject); err != nil {
			return nil, err
		}
	}

	if audience := config.Get("audience").AsAnyOr(nil); audience != nil {
		token.Options().Enable(jwt.FlattenAudience)
		if err := token.Set(jwt.AudienceKey, audience); err != nil {
			return nil, err
		}
	}
	if config.Get("set_jti").AsBoolOr(false) {
		if err := token.Set(jwt.JwtIDKey, uuid.NewString()); err != nil {
			return nil, err
		}
	}
	if config.Get("set_iat").AsBoolOr(true) {
		if err := token.Set(jwt.IssuedAtKey, now); err != nil {
			return nil, err
		}
	}
	if seconds := config.Get("expires_in_seconds").AsIntOr(0); seconds > 0 {
		if err := token.Set(jwt.ExpirationKey, now.Add(time.Duration(seconds)*time.Second)); err != nil {
			return nil, err
		}
	}
	return token, nil
}

func signJWTHeaders(config goutils.TreeMapImpl) (jws.Headers, error) {
	headers := jws.NewHeaders()
	rawHeaders, err := config.Get("headers").AsMap()
	if err == nil {
		for key, value := range rawHeaders {
			if strings.EqualFold(key, "alg") {
				continue
			}
			if err := headers.Set(key, value); err != nil {
				return nil, err
			}
		}
	}
	if kid := config.Get("key.kid").AsStringOr(""); kid != "" {
		if err := headers.Set(jws.KeyIDKey, kid); err != nil {
			return nil, err
		}
	}
	if _, exists := headers.Get(jws.TypeKey); !exists {
		if err := headers.Set(jws.TypeKey, "JWT"); err != nil {
			return nil, err
		}
	}
	return headers, nil
}

func signJWTKey(config goutils.TreeMapImpl) (jwa.SignatureAlgorithm, any, error) {
	alg, err := jwtSignatureAlgorithm(config.Get("algorithm").AsStringOr(""))
	if err != nil {
		return "", nil, err
	}
	keyType := config.Get("key.type").AsStringOr("")
	raw := config.Get("key.value").AsAnyOr(nil)
	switch keyType {
	case signJWTKeyTypePlainSecret, signJWTKeyTypeBase64URLSecret:
		if !alg.IsSymmetric() {
			return "", nil, errors.New("secret key type requires HS* algorithm")
		}
		secretBytes, err := signJWTSecretBytes(keyType, fmt.Sprint(raw))
		if err != nil {
			return "", nil, err
		}
		return alg, secretBytes, nil
	case signJWTKeyTypePEM:
		if alg.IsSymmetric() {
			return "", nil, errors.New("pem key type cannot use HS* algorithm")
		}
		return rawSigningKey(alg, []byte(fmt.Sprint(raw)), jwk.WithPEM(true))
	case signJWTKeyTypeJWK:
		if alg.IsSymmetric() {
			return "", nil, errors.New("jwk key type cannot use HS* algorithm")
		}
		data, err := marshalSignJWK(raw)
		if err != nil {
			return "", nil, err
		}
		return rawSigningKey(alg, data)
	default:
		return "", nil, errors.New("unsupported JWT signing key type")
	}
}

func signJWTSecretBytes(keyType, secret string) ([]byte, error) {
	if keyType == signJWTKeyTypePlainSecret {
		return []byte(secret), nil
	}
	return encoding.DecodeBase64URL(secret)
}

func rawSigningKey(alg jwa.SignatureAlgorithm, data []byte, options ...jwk.ParseOption) (jwa.SignatureAlgorithm, any, error) {
	key, err := jwk.ParseKey(data, options...)
	if err != nil {
		return "", nil, err
	}
	if keyAlgorithm := jwkString(key, jwk.AlgorithmKey); keyAlgorithm != "" && keyAlgorithm != alg.String() {
		return "", nil, errors.New("configured JWT algorithm does not match signing key algorithm")
	}
	var raw any
	if err := key.Raw(&raw); err != nil {
		return "", nil, err
	}
	return alg, raw, nil
}

func marshalSignJWK(raw any) ([]byte, error) {
	if text, ok := raw.(string); ok {
		return []byte(text), nil
	}
	return json.Marshal(raw)
}

func init() {
	defaultStepRegistry.AddStep(&SignJWT{}, map[string]map[string]any{
		".": {
			"x-category": types.AuthCategory,
			"x-order":    []string{"algorithm", "key", "claims", "headers", "issuer", "subject", "audience", "set_iat", "set_jti", "expires_in_seconds", "context", "output", "outcome"},
		},
		"key":       {"x-order": []string{"type", "value", "kid"}},
		"key.value": {"x-type": "scriptable"},
		"claims":    {"x-type": "json-object"},
		"headers":   {"x-type": "json-object"},
	})
}
