package steps

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nitsugaro/go-journey/env"
	"github.com/nitsugaro/go-journey/inputs"
	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
	"github.com/nitsugaro/go-utils/v2/crypto"
	"github.com/nitsugaro/go-utils/v2/encoding"
)

const (
	oidcClientAuthNone          = "none"
	oidcClientAuthPost          = "client_secret_post"
	oidcClientAuthBasic         = "client_secret_basic"
	oidcClientAuthPrivateJWT    = "private_key_jwt"
	oidcClientAuthTLS           = "tls_client_auth"
	oidcClientAuthSelfSignedTLS = "self_signed_tls_client_auth"
	oidcDefaultScope            = "openid"
	oidcDefaultSigningAlgorithm = "RS256"
	oidcErrorContextName        = "oidc"
)

type OIDCAuthorizationCode struct {
	BasicStep

	_ struct{} `description:"Starts or resumes an OIDC authorization-code flow. The first execution returns an authorization client input; the resumed execution exchanges the callback code for tokens."`

	AuthorizeURL         string         `json:"authorize_url" required:"true" minLength:"1" description:"OIDC provider authorization endpoint."`
	TokenURL             string         `json:"token_url" required:"true" minLength:"1" description:"OIDC provider token endpoint."`
	AudURL               string         `json:"aud_url" required:"true" minLength:"1" description:"Aud endpoint."`
	ClientID             string         `json:"client_id" required:"true" minLength:"1" description:"OIDC client id."`
	ClientSecret         string         `json:"client_secret,omitempty" description:"OIDC client secret. Required for client_secret_basic or client_secret_post."`
	PrivateKeyJWK        any            `json:"private_key_jwk,omitempty" description:"Private JWK used for private_key_jwt client authentication and signed JAR request objects."`
	SigningAlgorithm     string         `json:"signing_algorithm,omitempty" enum:"RS256,RS384,RS512,PS256,PS384,PS512,ES256,ES256K,ES384,ES512,EdDSA" default:"RS256" description:"Algorithm used to sign private_key_jwt and JAR request objects."`
	ClientAuthMethod     string         `json:"client_auth_method,omitempty" enum:"none,client_secret_post,client_secret_basic,private_key_jwt,tls_client_auth,self_signed_tls_client_auth" description:"Client authentication method. If empty, basic is used when a secret exists, private_key_jwt when only a private JWK exists, otherwise none. TLS client authentication uses the configured http_client transport certificates."`
	HTTPClient           string         `json:"http_client,omitempty" description:"Optional http_client cache instance used for PAR and token endpoint requests. Empty uses default."`
	RedirectURI          string         `json:"redirect_uri" required:"true" minLength:"1" description:"Redirect URI registered in the provider."`
	UseJAR               bool           `json:"use_jar" default:"false" description:"Sign authorization request parameters as a JWT request object."`
	PARURL               string         `json:"par_url,omitempty" description:"Optional pushed authorization request endpoint. When set, the step pushes the request and sends request_uri to the authorize URL."`
	Scope                []string       `json:"scope,omitempty" description:"Requested scopes. Defaults to openid."`
	PKCE                 bool           `json:"pkce" default:"true" description:"Generate and verify the authorization request with S256 PKCE."`
	Nonce                bool           `json:"nonce" default:"true" description:"Generate a nonce parameter for OIDC id_token replay protection."`
	ExtraAuthorizeParams map[string]any `json:"extra_authorize_params,omitempty" description:"Additional authorization endpoint parameters."`
	ExtraTokenParams     map[string]any `json:"extra_token_params,omitempty" description:"Additional token endpoint parameters."`
	Output               string         `json:"output" required:"true" pattern:"^(encCtx|closedCtx)(\\.\\w+)+$" description:"Context path where token response is saved."`
	Outcome              struct {
		True  string `json:"true" required:"true" format:"uuid"`
		False string `json:"false" required:"true" format:"uuid"`
	} `json:"outcome" required:"true"`
}

func (*OIDCAuthorizationCode) GetStepType() string { return types.OIDCAuthorizationCodeStep }

func (*OIDCAuthorizationCode) VerifyConfig(stepName string, config goutils.TreeMapImpl) error {
	for _, key := range []string{"authorize_url", "token_url", "client_id", "redirect_uri", "output"} {
		if strings.TrimSpace(config.Get(key).AsStringOr("")) == "" {
			return types.StepInvalidConfig(stepName, key+" is required")
		}
	}
	authMethod := strings.TrimSpace(config.Get("client_auth_method").AsStringOr(""))
	if strings.Contains(authMethod, "${") {
		return nil
	}
	switch authMethod {
	case "", oidcClientAuthNone, oidcClientAuthPost, oidcClientAuthBasic, oidcClientAuthPrivateJWT, oidcClientAuthTLS, oidcClientAuthSelfSignedTLS:
	default:
		return types.StepInvalidConfig(stepName, "unsupported client_auth_method")
	}
	clientSecret := strings.TrimSpace(config.Get("client_secret").AsStringOr(""))
	httpClient := strings.TrimSpace(config.Get("http_client").AsStringOr(""))
	if (authMethod == oidcClientAuthPost || authMethod == oidcClientAuthBasic) && clientSecret == "" {
		return types.StepInvalidConfig(stepName, "client_secret is required for "+authMethod)
	}
	if (authMethod == oidcClientAuthPrivateJWT || config.Get("use_jar").AsBoolOr(false)) && config.Get("private_key_jwk").IsEmpty() {
		return types.StepInvalidConfig(stepName, "private_key_jwk is required for private_key_jwt or JAR")
	}
	if (authMethod == oidcClientAuthTLS || authMethod == oidcClientAuthSelfSignedTLS) && httpClient == "" {
		return types.StepInvalidConfig(stepName, "http_client is required for "+authMethod)
	}
	if alg := strings.TrimSpace(config.Get("signing_algorithm").AsStringOr("")); alg != "" && !strings.Contains(alg, "${") {
		if _, err := jwtSignatureAlgorithm(alg); err != nil {
			return types.StepInvalidConfig(stepName, err.Error())
		}
	}
	output := config.Get("output").AsStringOr("")
	if strings.Contains(output, "${") {
		return nil
	}
	if !strings.HasPrefix(output, types.EncCtxKey+".") && !strings.HasPrefix(output, types.ClosedCtxKey+".") {
		return types.StepInvalidConfig(stepName, "output must start with encCtx. or closedCtx.")
	}
	return nil
}

func (step *OIDCAuthorizationCode) Execute(trx *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	baseKey := env.GetContextKey(oidcErrorContextName + "." + trx.CurrentStepID)

	expectedState := trx.State.GetClosedCtx().Get(baseKey + ".state").AsStringOr("")
	if expectedState == "" {
		trx.EmitEvent(&types.Event{
			Type:    types.EventInfo,
			Message: "not found state in context, returning authorization url...",
			Subject: types.EventSubject{
				Type: "step", ID: trx.CurrentStepID, Name: trx.Journey.Steps[trx.CurrentStepID].Name,
			},
		})
		return step.startAuthorization(trx, config, baseKey)
	}

	return step.exchangeCallback(trx, config, baseKey, expectedState)
}

func (step *OIDCAuthorizationCode) startAuthorization(transaction *types.JourneyTransaction, config goutils.TreeMapImpl, baseKey string) (string, error) {
	state, err := secureToken()
	if err != nil {
		return "", err
	}
	authorizeURL, err := url.Parse(config.Get("authorize_url").AsStringOr(""))
	if err != nil {
		return "", err
	}
	values := authorizeURL.Query()
	values.Set("client_id", config.Get("client_id").AsStringOr(""))
	values.Set("redirect_uri", config.Get("redirect_uri").AsStringOr(""))
	values.Set("response_type", "code")
	values.Set("scope", strings.Join(oidcScopes(config), " "))
	values.Set("state", state)

	closedCtx := transaction.State.GetClosedCtx()
	closedCtx.Set(baseKey+".state", state)

	if config.Get("nonce").AsBoolOr(true) {
		nonce, err := secureToken()
		if err != nil {
			return "", err
		}
		values.Set("nonce", nonce)
		closedCtx.Set(baseKey+".nonce", nonce)
	}
	if config.Get("pkce").AsBoolOr(true) {
		verifier, err := secureToken()
		if err != nil {
			return "", err
		}
		challenge := sha256.Sum256([]byte(verifier))
		values.Set("code_challenge", base64.RawURLEncoding.EncodeToString(challenge[:]))
		values.Set("code_challenge_method", "S256")
		closedCtx.Set(baseKey+".code_verifier", verifier)
	}

	extraParams := anyMap(config.Get("extra_authorize_params"))
	if config.Get("use_jar").AsBoolOr(false) {
		requestObject, err := step.buildRequestObject(config, values, config.Get("aud_url").AsStringOr(""), extraParams)
		if err != nil {
			step.setOIDCError(transaction, "jar_failed", err.Error(), nil)
			step.cleanup(transaction, baseKey)
			return "false", nil
		}
		values = url.Values{}
		values.Set("client_id", config.Get("client_id").AsStringOr(""))
		values.Set("request", requestObject)
	} else {
		addQueryParams(values, extraParams)
	}

	if parURL := strings.TrimSpace(config.Get("par_url").AsStringOr("")); parURL != "" {
		requestURI, err := step.pushAuthorizationRequest(transaction, config, parURL, values)
		if err != nil {
			step.setOIDCError(transaction, "par_failed", err.Error(), nil)
			step.cleanup(transaction, baseKey)
			return "false", nil
		}
		values = url.Values{}
		values.Set("client_id", config.Get("client_id").AsStringOr(""))
		values.Set("request_uri", requestURI)
	}
	authorizeURL.RawQuery = values.Encode()

	err = transaction.ClientInputsBuilder.AddMessageInput(&inputs.Message{
		ID:       transaction.CurrentStepID,
		StepType: types.OIDCAuthorizationCodeStep,
		Value: map[string]any{
			"type":          "oidc_authorization",
			"method":        http.MethodGet,
			"authorize_url": authorizeURL.String(),
		},
	})
	return "", err
}

func (step *OIDCAuthorizationCode) buildRequestObject(config goutils.TreeMapImpl, values url.Values, audience string, extraParams map[string]any) (string, error) {
	claims := map[string]any{}
	for key, items := range values {
		if len(items) == 1 {
			claims[key] = items[0]
		} else if len(items) > 1 {
			claims[key] = append([]string(nil), items...)
		}
	}

	maps.Copy(claims, extraParams)

	clientID := config.Get("client_id").AsStringOr("")
	return SignCompactJWT(JWTSignOptions{
		Algorithm: oidcSigningAlgorithm(config),
		Key:       SignJWTKey{Type: signJWTKeyTypeJWK, Value: config.Get("private_key_jwk").AsAnyOr(nil)},
		Claims:    claims,
		Headers:   map[string]any{"typ": "oauth-authz-req+jwt"},
		Issuer:    clientID,
		Audience:  []string{audience},
		SetIAT:    true,
		SetJTI:    true,
		ExpiresIn: 300,
	})
}

func (step *OIDCAuthorizationCode) pushAuthorizationRequest(transaction *types.JourneyTransaction, config goutils.TreeMapImpl, parURL string, values url.Values) (string, error) {
	headers := map[string]string{
		"Accept":       "application/json",
		"Content-Type": "application/x-www-form-urlencoded",
	}
	if err := step.applyClientAuthentication(transaction, config, parURL, values, headers); err != nil {
		return "", err
	}
	response, err := requestHTTP(
		transaction,
		transactionHTTPClientInstance(transaction, config.Get("http_client").AsStringOr("")),
		http.MethodPost,
		parURL,
		headers, []byte(values.Encode()))
	if err != nil {
		return "", err
	}
	if response.Status < 200 || response.Status >= 300 {
		return "", fmt.Errorf("OIDC PAR endpoint returned status %d: %s", response.Status, string(response.Body))
	}
	var payload map[string]any
	if err := json.Unmarshal(response.Body, &payload); err != nil {
		return "", err
	}
	requestURI, _ := payload["request_uri"].(string)
	if strings.TrimSpace(requestURI) == "" {
		return "", errors.New("OIDC PAR endpoint did not return request_uri")
	}
	return requestURI, nil
}

func (step *OIDCAuthorizationCode) exchangeCallback(trx *types.JourneyTransaction, config goutils.TreeMapImpl, baseKey string, expectedState string) (string, error) {
	request := trx.Request
	if request == nil {
		step.setOIDCError(trx, "missing_request", "OIDC callback requires request query parameters", nil)
		step.cleanup(trx, baseKey)
		return "false", nil
	}
	if actualState := request.QueryFirst("state"); actualState == "" || actualState != expectedState {
		step.setOIDCError(trx, "invalid_state", "OIDC callback state does not match stored state", map[string]any{"received_state": request.QueryFirst("state")})
		step.cleanup(trx, baseKey)
		return "false", nil
	}
	if providerError := request.QueryFirst("error"); providerError != "" {
		step.setOIDCError(trx, providerError, request.QueryFirst("error_description", providerError), map[string]any{"error_uri": request.QueryFirst("error_uri")})
		step.cleanup(trx, baseKey)
		return "false", nil
	}
	code := request.QueryFirst("code")
	if code == "" {
		step.setOIDCError(trx, "missing_code", "OIDC callback does not include an authorization code", nil)
		step.cleanup(trx, baseKey)
		return "false", nil
	}

	trx.EmitEvent(&types.Event{
		Type:    types.EventInfo,
		Message: "required data processed, starting exchange for tokens",
		Subject: types.EventSubject{
			Type: "step", ID: trx.CurrentStepID, Name: trx.Journey.Steps[trx.CurrentStepID].Name,
		},
	})

	tokens, err := step.exchangeToken(trx, config, baseKey, code)
	if err != nil {
		step.setOIDCError(trx, "token_exchange_failed", err.Error(), nil)
		step.cleanup(trx, baseKey)
		return "false", nil
	}
	output := config.Get("output").AsStringOr("")
	ctx, key := trx.State.GetCtxPath(output)
	if ctx == nil || key == "" {
		return "", types.StepInvalidConfig(trx.CurrentStepID, "invalid output context path")
	}
	ctx.Set(key, tokens)
	step.cleanup(trx, baseKey)
	return "true", nil
}

func (step *OIDCAuthorizationCode) exchangeToken(transaction *types.JourneyTransaction, config goutils.TreeMapImpl, baseKey, code string) (map[string]any, error) {
	authMethod := oidcClientAuthMethod(config)
	clientID := config.Get("client_id").AsStringOr("")
	clientSecret := config.Get("client_secret").AsStringOr("")
	values := url.Values{}
	values.Set("grant_type", "authorization_code")
	values.Set("code", code)
	values.Set("redirect_uri", config.Get("redirect_uri").AsStringOr(""))
	codeVerifier := transaction.State.GetClosedCtx().Get(baseKey + ".code_verifier").AsStringOr("")
	if codeVerifier != "" {
		values.Set("code_verifier", codeVerifier)
	}
	if authMethod == oidcClientAuthNone || authMethod == oidcClientAuthPost {
		values.Set("client_id", clientID)
	}
	if authMethod == oidcClientAuthPost {
		values.Set("client_secret", clientSecret)
	}
	addQueryParams(values, anyMap(config.Get("extra_token_params")))

	headers := map[string]string{
		"Accept":       "application/json",
		"Content-Type": "application/x-www-form-urlencoded",
	}
	if err := step.applyClientAuthentication(transaction, config, config.Get("token_url").AsStringOr(""), values, headers); err != nil {
		return nil, err
	}
	response, err := requestHTTP(
		transaction,
		transactionHTTPClientInstance(transaction, config.Get("http_client").AsStringOr("")),
		http.MethodPost,
		config.Get("token_url").AsStringOr(""),
		headers,
		[]byte(values.Encode()))
	if err != nil {
		return nil, err
	}
	if response.Status < 200 || response.Status >= 300 {
		return nil, fmt.Errorf("OIDC token endpoint returned status %d: %s", response.Status, string(response.Body))
	}
	var tokens map[string]any
	if err := json.Unmarshal(response.Body, &tokens); err != nil {
		return nil, err
	}
	return tokens, nil
}

func (step *OIDCAuthorizationCode) applyClientAuthentication(transaction *types.JourneyTransaction, config goutils.TreeMapImpl, audience string, values url.Values, headers map[string]string) error {
	authMethod := oidcClientAuthMethod(config)
	clientID := config.Get("client_id").AsStringOr("")
	clientSecret := config.Get("client_secret").AsStringOr("")
	switch authMethod {
	case oidcClientAuthNone:
		values.Set("client_id", clientID)
	case oidcClientAuthPost:
		values.Set("client_id", clientID)
		values.Set("client_secret", clientSecret)
	case oidcClientAuthBasic:
		if headers != nil {
			headers["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte(clientID+":"+clientSecret))
		}
	case oidcClientAuthPrivateJWT:
		assertion, err := SignCompactJWT(JWTSignOptions{
			Algorithm: oidcSigningAlgorithm(config),
			Key:       SignJWTKey{Type: signJWTKeyTypeJWK, Value: config.Get("private_key_jwk").AsAnyOr(nil)},
			Claims:    map[string]any{},
			Issuer:    clientID,
			Subject:   clientID,
			Audience:  []string{audience},
			SetIAT:    true,
			SetJTI:    true,
			ExpiresIn: 300,
			Now:       time.Now(),
		})
		if err != nil {
			return err
		}
		values.Set("client_id", clientID)
		values.Set("client_assertion_type", "urn:ietf:params:oauth:client-assertion-type:jwt-bearer")
		values.Set("client_assertion", assertion)
	case oidcClientAuthTLS, oidcClientAuthSelfSignedTLS:
		values.Set("client_id", clientID)
	}
	return nil
}

func (step *OIDCAuthorizationCode) setOIDCError(transaction *types.JourneyTransaction, code, message string, attrs map[string]any) {
	value := map[string]any{"code": code, "message": message}
	for key, attr := range attrs {
		if strings.TrimSpace(key) != "" && attr != nil {
			value[key] = attr
		}
	}
	transaction.State.GetTempCtx().Set(env.GetContextKey(oidcErrorContextName+"."+transaction.CurrentStepID+".error"), value)
}

func (*OIDCAuthorizationCode) cleanup(transaction *types.JourneyTransaction, baseKey string) {
	transaction.State.GetClosedCtx().Delete(baseKey)
}

func oidcClientAuthMethod(config goutils.TreeMapImpl) string {
	method := strings.TrimSpace(config.Get("client_auth_method").AsStringOr(""))
	if method != "" {
		return method
	}
	if !config.Get("private_key_jwk").IsEmpty() && strings.TrimSpace(config.Get("client_secret").AsStringOr("")) == "" {
		return oidcClientAuthPrivateJWT
	}
	if strings.TrimSpace(config.Get("client_secret").AsStringOr("")) != "" {
		return oidcClientAuthBasic
	}
	return oidcClientAuthNone
}

func oidcSigningAlgorithm(config goutils.TreeMapImpl) string {
	algorithm := strings.TrimSpace(config.Get("signing_algorithm").AsStringOr(""))
	if algorithm == "" {
		return oidcDefaultSigningAlgorithm
	}
	return algorithm
}

func oidcScopes(config goutils.TreeMapImpl) []string {
	values, err := config.Get("scope").AsSlice()
	if err != nil || len(values) == 0 {
		scopeText := strings.TrimSpace(config.Get("scope").AsStringOr(""))
		if scopeText != "" {
			return strings.Fields(scopeText)
		}
		return []string{oidcDefaultScope}
	}
	scopes := make([]string, 0, len(values))
	for _, value := range values {
		scope := strings.TrimSpace(value.AsStringOr(""))
		if scope != "" {
			scopes = append(scopes, scope)
		}
	}
	if len(scopes) == 0 {
		return []string{oidcDefaultScope}
	}
	return scopes
}

func addQueryParams(values url.Values, params map[string]any) {
	for key, raw := range params {
		key = strings.TrimSpace(key)
		if key == "" || raw == nil {
			continue
		}
		switch typed := raw.(type) {
		case []any:
			val, _ := json.Marshal(raw)
			values.Add(key, string(val))
		case []string:
			for _, item := range typed {
				values.Add(key, item)
			}
		default:
			values.Set(key, fmt.Sprint(raw))
		}
	}
}

func anyMap(value goutils.TreeMapImpl) map[string]any {
	var result map[string]any
	if err := value.AsStruct(&result); err != nil || result == nil {
		return map[string]any{}
	}
	return result
}

func secureToken() (string, error) {
	data, err := crypto.GetRandBytes(32)
	if err != nil {
		return "", err
	}

	return encoding.EncodeBase64URL(data), nil
}

func init() {
	defaultStepRegistry.AddStep(&OIDCAuthorizationCode{}, map[string]map[string]any{
		".": {
			"x-category":  types.AuthCategory,
			"x-flow-type": []string{types.AuthJourney},
			"x-order": []string{
				"authorize_url", "token_url", "aud_url", "par_url", "client_id", "client_secret", "private_key_jwk", "signing_algorithm", "client_auth_method", "http_client",
				"redirect_uri", "use_jar", "scope", "pkce", "nonce", "extra_authorize_params", "extra_token_params", "output", "outcome",
			},
		},
		"authorize_url":   {"x-type": "text-expandable"},
		"token_url":       {"x-type": "text-expandable"},
		"redirect_uri":    {"x-type": "text-expandable"},
		"par_url":         {"x-type": "text-expandable"},
		"aud_url":         {"x-type": "text-expandable"},
		"private_key_jwk": {"x-type": "json-object"},
		"output":          {"x-error": "Value must be an internal path: encCtx.<path> or closedCtx.<path>."},
	})
}
