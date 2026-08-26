package types

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	jcache "github.com/nitsugaro/go-journey/cache"
	"github.com/nitsugaro/go-journey/inputs"
	goutils "github.com/nitsugaro/go-utils/v2"
	"github.com/nitsugaro/go-utils/v2/cipher"
	"github.com/nitsugaro/go-utils/v2/encoding"
)

const (
	AuthJourney     = "auth"
	ResourceJourney = "resource"
	WorkflowJourney = "workflow"

	LegacyAuthJourney     = "auth-journey"
	LegacyResourceJourney = "resource-journey"
	LegacyProxyJourney    = "proxy-journey"
	LegacyWorkflowJourney = "workflow-journey"
)

const (
	FlowCategory        = "flow"
	ContextCategory     = "context"
	OperationalCategory = "operational"
	SessionCategory     = "session"
	AuthCategory        = "auth"
	IdentityCategory    = "identity"
)

//######## ENCRYPT CONTEXT

func EncryptCtx(encryptedCtx goutils.DefaultMap, key []byte) (string, error) {
	bytes, err := json.Marshal(encryptedCtx)
	if err != nil {
		return "", fmt.Errorf("invalid encrypt")
	}

	encryptedData, err := cipher.EncryptAESGCM(key, bytes)
	if err != nil {
		return "", fmt.Errorf("invalid encrypt")
	}

	return encoding.EncodeBase64(encryptedData), nil
}

func DecryptCtx(encryptedCtxStr string, key []byte) (goutils.TreeMapImpl, error) {
	decodedData, err := encoding.DecodeBase64(encryptedCtxStr)
	if err != nil {
		return nil, err
	}

	decryptedData, err := cipher.DecryptAESGCM(key, decodedData)
	if err != nil {
		return nil, err
	}

	var encryptedCtx goutils.DefaultMap
	err = json.Unmarshal(decryptedData, &encryptedCtx)
	if err != nil {
		return nil, err
	}

	return goutils.NewTreeMap(encryptedCtx), nil
}

//#######

type Realm struct {
	ID       uint   `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	Active   bool   `json:"active,omitempty"`
	OAuthAlg int16  `json:"oauth_alg,omitempty"`
	OidcAlg  int16  `json:"oidc_alg,omitempty"`
}

type JourneyPayloadReq struct {
	JourneyID    string                `json:"journey_id,omitempty"`
	ResumeID     string                `json:"resume_id,omitempty"`
	Jwt          string                `json:"journey_token,omitempty"`
	ClientInputs []*inputs.ClientInput `json:"client_inputs,omitempty"`
	ClientError  *inputs.ClientError   `json:"client_error,omitempty"`
	State        string                `json:"state,omitempty"`
	InitialData  map[string]any        `json:"initial_data,omitempty"`

	realm *Realm
}

type JourneyRequestBody struct {
	ContentType     string            `json:"content_type,omitempty"`
	MediaType       string            `json:"media_type,omitempty"`
	Parameters      map[string]string `json:"parameters,omitempty"`
	ContentEncoding string            `json:"content_encoding,omitempty"`
	ContentLength   int64             `json:"content_length"`
	Data            []byte            `json:"data,omitempty"`
}

type JourneyRequestCookie struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Path     string    `json:"path,omitempty"`
	Domain   string    `json:"domain,omitempty"`
	Expires  time.Time `json:"expires,omitempty"`
	MaxAge   int       `json:"max_age,omitempty"`
	Secure   bool      `json:"secure"`
	HTTPOnly bool      `json:"http_only"`
	SameSite string    `json:"same_site,omitempty"`
}

type JourneyRequestCertificate struct {
	Raw          []byte    `json:"raw,omitempty"`
	Subject      string    `json:"subject,omitempty"`
	Issuer       string    `json:"issuer,omitempty"`
	SerialNumber string    `json:"serial_number,omitempty"`
	NotBefore    time.Time `json:"not_before,omitempty"`
	NotAfter     time.Time `json:"not_after,omitempty"`
	DNSNames     []string  `json:"dns_names,omitempty"`
}

type JourneyRequest struct {
	RequestURI      string                      `json:"request_uri,omitempty"`
	Path            string                      `json:"path,omitempty"`
	RoutePrefix     string                      `json:"route_prefix,omitempty"`
	RoutePath       string                      `json:"route_path,omitempty"`
	RawQuery        string                      `json:"raw_query,omitempty"`
	QueryParameters map[string][]string         `json:"query_parameters,omitempty"`
	Headers         map[string][]string         `json:"headers,omitempty"`
	Body            JourneyRequestBody          `json:"body"`
	Method          string                      `json:"method,omitempty"`
	Origin          string                      `json:"origin,omitempty"`
	BaseURL         string                      `json:"base_url,omitempty"`
	Host            string                      `json:"host,omitempty"`
	Port            uint16                      `json:"port,omitempty"`
	Protocol        string                      `json:"protocol,omitempty"`
	HTTPVersion     string                      `json:"http_version,omitempty"`
	RemoteAddress   string                      `json:"remote_address,omitempty"`
	TLSVersion      string                      `json:"tls_version,omitempty"`
	Certificates    []JourneyRequestCertificate `json:"certificates,omitempty"`
	Cookies         []JourneyRequestCookie      `json:"cookies,omitempty"`
}

type RequestAccessor interface {
	GetMethod() string
	GetPath() string
	GetRequestURI() string
	RoutePrefixValue() string
	RoutePathValue() string
	GetRawQuery() string
	QueryValues(name string) []string
	QueryFirst(name string, defaultValue ...string) string
	QueryMap() map[string][]string
	HeaderValues(name string) []string
	HeaderFirst(name string, defaultValue ...string) string
	HeaderMap() map[string][]string
	Cookie(name string) (*http.Cookie, bool)
	CookiesList() []*http.Cookie
	BodyBytes() ([]byte, error)
	BodyContentType() string
	Snapshot() *JourneyRequest
}

type ResponseMutator interface {
	Header(name string, value string)
	AddHeader(name string, value string)
	SetCookie(cookie *http.Cookie)
	Status(code int)
	Body(contentType string, data []byte)
}

type HTTPRequestAccessor struct {
	request     *http.Request
	body        []byte
	bodyRead    bool
	maxBodySize int64
	routePrefix string
	routePath   string
}

func NewHTTPRequestAccessor(request *http.Request, maxBodySize int64) *HTTPRequestAccessor {
	return &HTTPRequestAccessor{request: request, maxBodySize: maxBodySize}
}

func NewHTTPRequestAccessorWithBody(request *http.Request, body []byte, maxBodySize int64) *HTTPRequestAccessor {
	return &HTTPRequestAccessor{request: request, body: append([]byte(nil), body...), bodyRead: true, maxBodySize: maxBodySize}
}

func (request *HTTPRequestAccessor) SetRoute(prefix, path string) {
	if request == nil {
		return
	}
	request.routePrefix = prefix
	request.routePath = path
}

func (request *HTTPRequestAccessor) GetMethod() string {
	if request == nil || request.request == nil {
		return ""
	}
	return request.request.Method
}

func (request *HTTPRequestAccessor) GetPath() string {
	if request == nil || request.request == nil || request.request.URL == nil {
		return ""
	}
	return request.request.URL.Path
}

func (request *HTTPRequestAccessor) GetRequestURI() string {
	if request == nil || request.request == nil {
		return ""
	}
	return request.request.RequestURI
}

func (request *HTTPRequestAccessor) RoutePrefixValue() string {
	if request == nil {
		return ""
	}
	return request.routePrefix
}

func (request *HTTPRequestAccessor) RoutePathValue() string {
	if request == nil {
		return ""
	}
	return request.routePath
}

func (request *HTTPRequestAccessor) GetRawQuery() string {
	if request == nil || request.request == nil || request.request.URL == nil {
		return ""
	}
	return request.request.URL.RawQuery
}

func (request *HTTPRequestAccessor) QueryValues(name string) []string {
	if request == nil || request.request == nil || request.request.URL == nil {
		return []string{}
	}
	return append([]string(nil), request.request.URL.Query()[name]...)
}

func (request *HTTPRequestAccessor) QueryFirst(name string, defaultValue ...string) string {
	return firstOrDefault(request.QueryValues(name), defaultValue...)
}

func (request *HTTPRequestAccessor) QueryMap() map[string][]string {
	if request == nil || request.request == nil || request.request.URL == nil {
		return map[string][]string{}
	}
	return cloneStringValues(request.request.URL.Query())
}

func (request *HTTPRequestAccessor) HeaderValues(name string) []string {
	if request == nil || request.request == nil {
		return []string{}
	}
	return append([]string(nil), request.request.Header.Values(name)...)
}

func (request *HTTPRequestAccessor) HeaderFirst(name string, defaultValue ...string) string {
	if request == nil || request.request == nil {
		return firstOrDefault(nil, defaultValue...)
	}
	value := request.request.Header.Get(name)
	if value != "" {
		return value
	}
	return firstOrDefault(nil, defaultValue...)
}

func (request *HTTPRequestAccessor) HeaderMap() map[string][]string {
	if request == nil || request.request == nil {
		return map[string][]string{}
	}
	return cloneStringValues(request.request.Header)
}

func (request *HTTPRequestAccessor) Cookie(name string) (*http.Cookie, bool) {
	if request == nil || request.request == nil {
		return nil, false
	}
	cookie, err := request.request.Cookie(name)
	return cookie, err == nil
}

func (request *HTTPRequestAccessor) CookiesList() []*http.Cookie {
	if request == nil || request.request == nil {
		return []*http.Cookie{}
	}
	return append([]*http.Cookie(nil), request.request.Cookies()...)
}

func (request *HTTPRequestAccessor) BodyBytes() ([]byte, error) {
	if request == nil || request.request == nil || request.request.Body == nil {
		return nil, nil
	}
	if request.bodyRead {
		return append([]byte(nil), request.body...), nil
	}
	var reader io.Reader = request.request.Body
	if request.maxBodySize > 0 {
		reader = io.LimitReader(request.request.Body, request.maxBodySize+1)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if request.maxBodySize > 0 && int64(len(data)) > request.maxBodySize {
		return nil, fmt.Errorf("request body exceeds %d bytes", request.maxBodySize)
	}
	request.body = append([]byte(nil), data...)
	request.bodyRead = true
	request.request.Body = io.NopCloser(bytes.NewReader(request.body))
	return append([]byte(nil), request.body...), nil
}

func (request *HTTPRequestAccessor) BodyContentType() string {
	if request == nil || request.request == nil {
		return ""
	}
	return request.request.Header.Get("Content-Type")
}

func (request *HTTPRequestAccessor) Snapshot() *JourneyRequest {
	if request == nil || request.request == nil {
		return NewEmptyRequest().Snapshot()
	}
	body, _ := request.BodyBytes()
	snapshot := NewJourneyRequest(request.request, body)
	if snapshot != nil {
		snapshot.RoutePrefix = request.routePrefix
		snapshot.RoutePath = request.routePath
	}
	return snapshot
}

type MemoryRequest struct {
	SnapshotValue *JourneyRequest
}

func NewEmptyRequest() *MemoryRequest {
	return &MemoryRequest{SnapshotValue: &JourneyRequest{}}
}

func NewMemoryRequest(snapshot *JourneyRequest) *MemoryRequest {
	if snapshot == nil {
		snapshot = &JourneyRequest{}
	}
	return &MemoryRequest{SnapshotValue: snapshot}
}

func (request *MemoryRequest) snapshot() *JourneyRequest {
	if request == nil || request.SnapshotValue == nil {
		return &JourneyRequest{}
	}
	return request.SnapshotValue
}

func (request *MemoryRequest) GetMethod() string        { return request.snapshot().Method }
func (request *MemoryRequest) GetPath() string          { return request.snapshot().Path }
func (request *MemoryRequest) GetRequestURI() string    { return request.snapshot().RequestURI }
func (request *MemoryRequest) RoutePrefixValue() string { return request.snapshot().RoutePrefix }
func (request *MemoryRequest) RoutePathValue() string   { return request.snapshot().RoutePath }
func (request *MemoryRequest) GetRawQuery() string      { return request.snapshot().RawQuery }
func (request *MemoryRequest) QueryValues(name string) []string {
	return append([]string(nil), request.snapshot().QueryParameters[name]...)
}
func (request *MemoryRequest) QueryFirst(name string, defaultValue ...string) string {
	return firstOrDefault(request.QueryValues(name), defaultValue...)
}
func (request *MemoryRequest) QueryMap() map[string][]string {
	return cloneStringValues(request.snapshot().QueryParameters)
}
func (request *MemoryRequest) HeaderValues(name string) []string {
	headers := request.snapshot().Headers
	if values, found := headers[name]; found {
		return append([]string(nil), values...)
	}
	for candidate, values := range headers {
		if strings.EqualFold(candidate, name) {
			return append([]string(nil), values...)
		}
	}
	return []string{}
}
func (request *MemoryRequest) HeaderFirst(name string, defaultValue ...string) string {
	return firstOrDefault(request.HeaderValues(name), defaultValue...)
}
func (request *MemoryRequest) HeaderMap() map[string][]string {
	return cloneStringValues(request.snapshot().Headers)
}
func (request *MemoryRequest) Cookie(name string) (*http.Cookie, bool) {
	for _, cookie := range request.snapshot().Cookies {
		if cookie.Name == name {
			return &http.Cookie{Name: cookie.Name, Value: cookie.Value, Path: cookie.Path, Domain: cookie.Domain, Expires: cookie.Expires, MaxAge: cookie.MaxAge, Secure: cookie.Secure, HttpOnly: cookie.HTTPOnly, SameSite: httpSameSite(cookie.SameSite)}, true
		}
	}
	return nil, false
}
func (request *MemoryRequest) CookiesList() []*http.Cookie {
	cookies := request.snapshot().Cookies
	result := make([]*http.Cookie, 0, len(cookies))
	for _, cookie := range cookies {
		result = append(result, &http.Cookie{Name: cookie.Name, Value: cookie.Value, Path: cookie.Path, Domain: cookie.Domain, Expires: cookie.Expires, MaxAge: cookie.MaxAge, Secure: cookie.Secure, HttpOnly: cookie.HTTPOnly, SameSite: httpSameSite(cookie.SameSite)})
	}
	return result
}
func (request *MemoryRequest) BodyBytes() ([]byte, error) {
	return append([]byte(nil), request.snapshot().Body.Data...), nil
}
func (request *MemoryRequest) BodyContentType() string { return request.snapshot().Body.ContentType }
func (request *MemoryRequest) Snapshot() *JourneyRequest {
	source := request.snapshot()
	copy := *source
	copy.QueryParameters = cloneStringValues(source.QueryParameters)
	copy.Headers = cloneStringValues(source.Headers)
	copy.Body.Data = append([]byte(nil), source.Body.Data...)
	copy.Cookies = append([]JourneyRequestCookie(nil), source.Cookies...)
	copy.Certificates = append([]JourneyRequestCertificate(nil), source.Certificates...)
	return &copy
}

type MemoryResponse struct {
	Headers        map[string][]string
	Cookies        []*http.Cookie
	StatusCode     int
	ContentType    string
	BodyBytesValue []byte
	BodySet        bool
}

func NewMemoryResponse() *MemoryResponse {
	return &MemoryResponse{Headers: map[string][]string{}}
}

func (response *MemoryResponse) Header(name string, value string) {
	if response == nil || strings.TrimSpace(name) == "" {
		return
	}
	if response.Headers == nil {
		response.Headers = map[string][]string{}
	}
	response.Headers[name] = []string{value}
}

func (response *MemoryResponse) AddHeader(name string, value string) {
	if response == nil || strings.TrimSpace(name) == "" {
		return
	}
	if response.Headers == nil {
		response.Headers = map[string][]string{}
	}
	response.Headers[name] = append(response.Headers[name], value)
}

func (response *MemoryResponse) SetCookie(cookie *http.Cookie) {
	if response == nil || cookie == nil {
		return
	}
	copy := *cookie
	response.Cookies = append(response.Cookies, &copy)
}

func (response *MemoryResponse) Status(code int) {
	if response == nil {
		return
	}
	response.StatusCode = code
}

func (response *MemoryResponse) Body(contentType string, data []byte) {
	if response == nil {
		return
	}
	response.ContentType = contentType
	response.BodyBytesValue = append([]byte(nil), data...)
	response.BodySet = true
}

func (request *JourneyRequest) MethodValue() string {
	if request == nil {
		return ""
	}
	return request.Method
}

func (request *JourneyRequest) GetMethod() string { return request.MethodValue() }
func (request *JourneyRequest) PathValue() string {
	if request == nil {
		return ""
	}
	return request.Path
}
func (request *JourneyRequest) GetPath() string { return request.PathValue() }
func (request *JourneyRequest) RequestURIValue() string {
	if request == nil {
		return ""
	}
	return request.RequestURI
}
func (request *JourneyRequest) GetRequestURI() string { return request.RequestURIValue() }
func (request *JourneyRequest) RoutePrefixValue() string {
	if request == nil {
		return ""
	}
	return request.RoutePrefix
}
func (request *JourneyRequest) RoutePathValue() string {
	if request == nil {
		return ""
	}
	return request.RoutePath
}
func (request *JourneyRequest) RawQueryValue() string {
	if request == nil {
		return ""
	}
	return request.RawQuery
}
func (request *JourneyRequest) GetRawQuery() string { return request.RawQueryValue() }
func (request *JourneyRequest) QueryValues(name string) []string {
	if request == nil {
		return []string{}
	}
	return append([]string(nil), request.QueryParameters[name]...)
}
func (request *JourneyRequest) QueryFirst(name string, defaultValue ...string) string {
	return firstOrDefault(request.QueryValues(name), defaultValue...)
}
func (request *JourneyRequest) QueryMap() map[string][]string {
	if request == nil {
		return map[string][]string{}
	}
	return cloneStringValues(request.QueryParameters)
}
func (request *JourneyRequest) HeaderValues(name string) []string {
	if request == nil {
		return []string{}
	}
	if values, found := request.Headers[name]; found {
		return append([]string(nil), values...)
	}
	for candidate, values := range request.Headers {
		if strings.EqualFold(candidate, name) {
			return append([]string(nil), values...)
		}
	}
	return []string{}
}
func (request *JourneyRequest) HeaderFirst(name string, defaultValue ...string) string {
	return firstOrDefault(request.HeaderValues(name), defaultValue...)
}
func (request *JourneyRequest) HeaderMap() map[string][]string {
	if request == nil {
		return map[string][]string{}
	}
	return cloneStringValues(request.Headers)
}
func (request *JourneyRequest) Cookie(name string) (*http.Cookie, bool) {
	if request == nil {
		return nil, false
	}
	return NewMemoryRequest(request).Cookie(name)
}
func (request *JourneyRequest) CookiesList() []*http.Cookie {
	return NewMemoryRequest(request).CookiesList()
}
func (request *JourneyRequest) BodyBytes() ([]byte, error) {
	if request == nil {
		return nil, nil
	}
	return append([]byte(nil), request.Body.Data...), nil
}
func (request *JourneyRequest) BodyContentType() string {
	if request == nil {
		return ""
	}
	return request.Body.ContentType
}
func (request *JourneyRequest) Snapshot() *JourneyRequest {
	return NewMemoryRequest(request).Snapshot()
}

func firstOrDefault(values []string, defaultValue ...string) string {
	if len(values) > 0 {
		return values[0]
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return ""
}

// NewJourneyRequest creates a transport-neutral snapshot. body is supplied by
// the caller so this helper never consumes or closes the incoming request body.
func NewJourneyRequest(request *http.Request, body []byte) *JourneyRequest {
	if request == nil {
		return nil
	}
	scheme := request.URL.Scheme
	if scheme == "" {
		if request.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := request.URL.Host
	if host == "" {
		host = request.Host
	}
	hostname := request.URL.Hostname()
	portText := request.URL.Port()
	if hostname == "" {
		if parsedHost, parsedPort, err := net.SplitHostPort(host); err == nil {
			hostname, portText = parsedHost, parsedPort
		} else {
			hostname = host
		}
	}
	port := requestPort(portText, scheme)
	baseHost := hostname
	if strings.Contains(baseHost, ":") && !strings.HasPrefix(baseHost, "[") {
		baseHost = "[" + baseHost + "]"
	}
	baseURL := scheme + "://" + baseHost
	contentType := request.Header.Get("Content-Type")
	mediaType, parameters, _ := mime.ParseMediaType(contentType)
	result := &JourneyRequest{
		RequestURI: request.RequestURI, Path: request.URL.Path, RawQuery: request.URL.RawQuery,
		QueryParameters: cloneStringValues(request.URL.Query()), Headers: cloneStringValues(request.Header),
		Body: JourneyRequestBody{
			ContentType: contentType, MediaType: mediaType, Parameters: parameters,
			ContentEncoding: request.Header.Get("Content-Encoding"), ContentLength: int64(len(body)), Data: append([]byte(nil), body...),
		},
		Method: request.Method, Origin: request.Header.Get("Origin"), BaseURL: baseURL,
		Host: host, Port: port, Protocol: scheme, HTTPVersion: request.Proto, RemoteAddress: request.RemoteAddr,
	}
	if request.TLS != nil {
		result.TLSVersion = tlsVersionName(request.TLS.Version)
		for _, certificate := range request.TLS.PeerCertificates {
			result.Certificates = append(result.Certificates, JourneyRequestCertificate{
				Raw: append([]byte(nil), certificate.Raw...), Subject: certificate.Subject.String(), Issuer: certificate.Issuer.String(),
				SerialNumber: certificate.SerialNumber.String(), NotBefore: certificate.NotBefore, NotAfter: certificate.NotAfter,
				DNSNames: append([]string(nil), certificate.DNSNames...),
			})
		}
	}
	for _, cookie := range request.Cookies() {
		result.Cookies = append(result.Cookies, JourneyRequestCookie{
			Name: cookie.Name, Value: cookie.Value, Path: cookie.Path, Domain: cookie.Domain,
			Expires: cookie.Expires, MaxAge: cookie.MaxAge, Secure: cookie.Secure, HTTPOnly: cookie.HttpOnly,
			SameSite: sameSiteName(cookie.SameSite),
		})
	}
	return result
}

func cloneStringValues(source map[string][]string) map[string][]string {
	result := make(map[string][]string, len(source))
	for key, values := range source {
		result[key] = append([]string(nil), values...)
	}
	return result
}

func requestPort(value, scheme string) uint16 {
	if value == "" {
		if scheme == "https" {
			return 443
		}
		return 80
	}
	parsed, err := strconv.ParseUint(value, 10, 16)
	if err != nil {
		return 0
	}
	return uint16(parsed)
}

func tlsVersionName(version uint16) string {
	switch version {
	case tls.VersionTLS10:
		return "TLS1.0"
	case tls.VersionTLS11:
		return "TLS1.1"
	case tls.VersionTLS12:
		return "TLS1.2"
	case tls.VersionTLS13:
		return "TLS1.3"
	default:
		return ""
	}
}

func sameSiteName(value http.SameSite) string {
	switch value {
	case http.SameSiteDefaultMode:
		return "default"
	case http.SameSiteLaxMode:
		return "lax"
	case http.SameSiteStrictMode:
		return "strict"
	case http.SameSiteNoneMode:
		return "none"
	default:
		return ""
	}
}

func httpSameSite(value string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "lax":
		return http.SameSiteLaxMode
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	case "default":
		return http.SameSiteDefaultMode
	default:
		return 0
	}
}

type JourneyExecute struct {
	Payload        *JourneyPayloadReq
	IsConfidential bool
	Request        RequestAccessor
	Response       ResponseMutator
	Context        context.Context
	CacheManager   *jcache.Manager
	OnAsyncError   func(step *Step, err error)
}

func (jp *JourneyPayloadReq) SetRealm(realm *Realm) *JourneyPayloadReq {
	jp.realm = realm

	return jp
}

func (jp *JourneyPayloadReq) GetRealm() *Realm {
	return jp.realm
}

func parseArgs(args ...any) (string, any) {
	if len(args) == 0 {
		return "", nil
	}

	key, _ := args[0].(string)
	def := any(nil)
	if len(args) > 1 {
		def = args[1]
	}

	return key, def
}

func (jp *JourneyPayloadReq) GetClientInputs() []*inputs.ClientInput {
	if len(jp.ClientInputs) == 0 {
		return []*inputs.ClientInput{}
	}

	return jp.ClientInputs
}

func (jp *JourneyPayloadReq) TokenPresent() bool {
	return jp.Jwt != ""
}

func (jp *JourneyPayloadReq) ToJson() string {
	bytes, _ := json.Marshal(jp)
	return string(bytes)
}

type JourneyInvoker interface {
	InvokeJourney(*JourneyExecute) (*JourneyPayloadReq, *JourneyState, error)
}

func (transaction *JourneyTransaction) ExpressionBindings() map[string]any {
	if transaction == nil || transaction.State == nil {
		return map[string]any{}
	}
	bindings := transaction.State.GetterFunctions()
	if transaction.Request != nil {
		bindings["request"] = transaction.Request.Snapshot()
	} else {
		bindings["request"] = NewEmptyRequest().Snapshot()
	}
	bindings["requestQuery"] = NewRequestQueryBinding(transaction.Request)
	bindings["requestHeader"] = NewRequestHeaderBinding(transaction.Request)
	bindings["payload"] = transaction.Payload
	bindings["journey"] = transaction.Journey
	bindings["currentStepID"] = transaction.CurrentStepID
	if transaction.Payload != nil && transaction.Payload.GetRealm() != nil {
		bindings["realm"] = transaction.Payload.GetRealm()
	} else if realm := transaction.State.GetRealm(); realm != "" {
		bindings["realm"] = &Realm{Name: realm}
	}
	return bindings
}
