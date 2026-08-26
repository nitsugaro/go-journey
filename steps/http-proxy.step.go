package steps

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type HTTPProxy struct {
	BasicStep

	_ struct{} `description:"Proxies a resource journey request through a compiled HTTP route table, writes the upstream response, and continues the flow."`

	RouteTable          string            `json:"route_table" required:"true" minLength:"1" description:"HTTP route table instance used to select the policy and upstream configuration."`
	BaseURL             string            `json:"base_url,omitempty" description:"Optional protocol and host (and optional base path) used instead of the incoming request base URI for matching and proxying."`
	Rewrite             map[string]string `json:"rewrite,omitempty" additionalProperties.type:"string" description:"Optional request path prefix rewrites applied before URI matching and proxying. The longest matching prefix wins."`
	NoMatchStatus       int               `json:"no_match_status" default:"404" minimum:"100" maximum:"599" description:"HTTP status returned when no route matches."`
	PolicyDeniedStatus  int               `json:"policy_denied_status" default:"403" minimum:"100" maximum:"599" description:"HTTP status returned when a route policy denies the request."`
	UpstreamErrorStatus int               `json:"upstream_error_status" default:"502" minimum:"100" maximum:"599" description:"HTTP status returned when the upstream call fails."`
	CopyResponseHeaders bool              `json:"copy_response_headers" default:"true" description:"Copy upstream response headers to the journey response."`
	SaveMatchTo         string            `json:"save_match_to,omitempty" pattern:"^(ctx|encCtx|closedCtx|tempCtx)(\\.\\w+)+$" description:"Optional context path where matched route metadata is stored."`
	Outcome             struct {
		True  string `json:"true" required:"true" format:"uuid"`
		False string `json:"false" required:"true" format:"uuid"`
	} `json:"outcome" required:"true"`
}

func (*HTTPProxy) GetStepType() string {
	return types.HTTPProxyStep
}

func (*HTTPProxy) Execute(transaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	if transaction == nil || transaction.Request == nil {
		return "false", errors.New("request is not available")
	}
	if transaction.Response == nil {
		return "false", errors.New("response is not available")
	}
	if transaction.CacheManager == nil {
		return "false", errors.New("cache manager is not configured")
	}
	if transaction.Context == nil {
		return "false", errors.New("transaction context is not available")
	}

	tableID := strings.TrimSpace(config.Get("route_table").AsStringOr(""))
	if tableID == "" {
		return "false", errors.New("route_table is required")
	}
	value, found := transaction.CacheManager.GetCacheInstance(HTTPRouteTableCacheKey, tableID)
	if !found {
		httpProxyWriteError(transaction.Response, httpProxyStatus(config, "no_match_status", http.StatusNotFound), "HTTP proxy route table is not configured")
		return "false", nil
	}
	table, ok := value.(*HTTPRouteTable)
	if !ok {
		return "false", fmt.Errorf("http route table %q has an invalid type", tableID)
	}

	method := strings.TrimSpace(transaction.Request.GetMethod())
	if method == "" {
		method = http.MethodGet
	}
	path := strings.TrimSpace(transaction.Request.RoutePathValue())
	if path == "" {
		path = transaction.Request.GetPath()
	}
	rewrite := map[string]string{}
	_ = config.Get("rewrite").AsStruct(&rewrite)
	path = httpProxyRewritePath(path, rewrite)
	effectiveURI, baseURLOverridden, err := httpProxyEffectiveURI(transaction.Request.Snapshot(), strings.TrimSpace(config.Get("base_url").AsStringOr("")), path)
	if err != nil {
		return "false", err
	}

	match, matched := table.MatchURI(effectiveURI, method)
	if !matched {
		httpProxyWriteError(transaction.Response, httpProxyStatus(config, "no_match_status", http.StatusNotFound), "no HTTP proxy route matched")
		return "false", nil
	}
	if err := table.EvaluatePolicies(transaction.Context, transaction, match); err != nil {
		transaction.State.GetTempCtx().Set("http_proxy_error", err.Error())
		httpProxyWriteError(transaction.Response, httpProxyStatus(config, "policy_denied_status", http.StatusForbidden), err.Error())
		return "false", nil
	}

	if saveMatchTo := strings.TrimSpace(config.Get("save_match_to").AsStringOr("")); saveMatchTo != "" {
		ctx, key := transaction.State.GetCtxPath(saveMatchTo)
		if ctx == nil || key == "" {
			return "false", fmt.Errorf("unsupported save_match_to path %q", saveMatchTo)
		}
		ctx.Set(key, map[string]any{
			"route":         match.Route.Name,
			"upstream":      match.Upstream.URL,
			"http_instance": match.HTTPInstance,
			"params":        match.Params,
			"wildcard":      match.Wildcard,
			"request_path":  match.RequestPath,
			"request_uri":   match.RequestURI,
			"metadata":      match.Metadata,
		})
	}

	targetURI := effectiveURI
	if !baseURLOverridden && (match.Upstream.URL != "" || match.Route.Upstream != "") {
		targetURI, err = table.TargetURI(match, transaction.Request.GetRawQuery())
		if err != nil {
			return "false", err
		}
	}
	body, err := transaction.Request.BodyBytes()
	if err != nil {
		return "false", err
	}
	headers := table.RouteHeaders(transaction.Request.HeaderMap(), match)
	if !httpProxyHasHeader(headers, "Content-Type") {
		if contentType := transaction.Request.BodyContentType(); contentType != "" {
			headers["Content-Type"] = contentType
		}
	}

	client := transactionHTTPClientInstance(transaction, match.HTTPInstance)
	response, err := requestHTTP(transaction, client, method, targetURI, headers, body)
	if err != nil {
		transaction.State.GetTempCtx().Set("http_proxy_error", err.Error())
		httpProxyWriteError(transaction.Response, httpProxyStatus(config, "upstream_error_status", http.StatusBadGateway), err.Error())
		return "false", nil
	}

	if config.Get("copy_response_headers").AsBoolOr(true) {
		httpProxyCopyResponseHeaders(transaction.Response, response.Headers)
	}
	transaction.Response.Status(response.Status)
	transaction.Response.Body(httpProxyFirstHeader(response.Headers, "Content-Type", "application/octet-stream"), response.Body)
	transaction.State.GetTempCtx().TryDelete("http_proxy_error")
	return "true", nil
}

func httpProxyEffectiveURI(request *types.JourneyRequest, baseURL, path string) (string, bool, error) {
	overridden := baseURL != ""
	if baseURL == "" && request != nil {
		baseURL = strings.TrimSpace(request.BaseURL)
		if baseURL == "" && request.Host != "" {
			scheme := strings.TrimSpace(request.Protocol)
			if scheme == "" {
				scheme = "http"
			}
			baseURL = scheme + "://" + request.Host
		}
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", overridden, fmt.Errorf("HTTP proxy base_url must include protocol and host")
	}
	parsed.Path = joinHTTPURLPath(parsed.Path, path)
	parsed.RawQuery = ""
	if request != nil {
		parsed.RawQuery = request.RawQuery
	}
	parsed.Fragment = ""
	return parsed.String(), overridden, nil
}

func httpProxyRewritePath(path string, rewrites map[string]string) string {
	path = cleanHTTPRoutePath(path)
	prefixes := make([]string, 0, len(rewrites))
	normalized := make(map[string]string, len(rewrites))
	for prefix, replacement := range rewrites {
		prefix = cleanHTTPRoutePath(prefix)
		prefixes = append(prefixes, prefix)
		normalized[prefix] = replacement
	}
	sort.SliceStable(prefixes, func(i, j int) bool { return len(prefixes[i]) > len(prefixes[j]) })
	for _, prefix := range prefixes {
		if path != prefix && !strings.HasPrefix(path, strings.TrimRight(prefix, "/")+"/") {
			continue
		}
		suffix := strings.TrimPrefix(path, prefix)
		return cleanHTTPRoutePath(joinHTTPURLPath(normalized[prefix], suffix))
	}
	return path
}

func httpProxyStatus(config goutils.TreeMapImpl, key string, fallback int) int {
	status := int(config.Get(key).AsIntOr(int64(fallback)))
	if status < 100 || status > 599 {
		return fallback
	}
	return status
}

func httpProxyWriteError(response types.ResponseMutator, status int, message string) {
	data, _ := json.Marshal(map[string]any{"error": message})
	response.Status(status)
	response.Body("application/json; charset=utf-8", data)
}

func httpProxyCopyResponseHeaders(response types.ResponseMutator, headers http.Header) {
	for name, values := range headers {
		if httpProxySkipHeader(name) {
			continue
		}
		for index, value := range values {
			if index == 0 {
				response.Header(name, value)
			} else {
				response.AddHeader(name, value)
			}
		}
	}
}

func httpProxySkipHeader(name string) bool {
	if name == "" || strings.EqualFold(name, "Content-Length") {
		return true
	}
	for _, header := range hopByHopHeaders() {
		if strings.EqualFold(name, header) {
			return true
		}
	}
	return false
}

func httpProxyFirstHeader(headers http.Header, name string, fallback string) string {
	if value := headers.Get(name); value != "" {
		return value
	}
	return fallback
}

func httpProxyHasHeader(headers map[string]string, name string) bool {
	for candidate := range headers {
		if strings.EqualFold(candidate, name) {
			return true
		}
	}
	return false
}

func init() {
	defaultStepRegistry.AddStep(&HTTPProxy{}, map[string]map[string]any{
		".": {
			"x-category": types.FlowCategory,
			"x-order":    []string{"route_table", "base_url", "rewrite", "no_match_status", "policy_denied_status", "upstream_error_status", "copy_response_headers", "save_match_to", "outcome"},
		},
		"route_table": {
			"x-type": "selectable",
			"x-props": map[string]any{
				"endpoint":      "/journey/:realm/instances?cache=http_route_table",
				"nameProperty":  "instance_id",
				"valueProperty": "instance_id",
			},
		},
	})
}
