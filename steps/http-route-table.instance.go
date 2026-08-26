package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/nitsugaro/go-journey/types"
)

const HTTPRouteTableCacheKey = "http_route_table"

const (
	httpRoutePolicyHeaderPresent = "header_present"
	httpRoutePolicyHeaderEquals  = "header_equals"
	httpRoutePolicyHeaderMatches = "header_matches"
	httpRoutePolicyQueryPresent  = "query_present"
	httpRoutePolicyQueryEquals   = "query_equals"
	httpRoutePolicyQueryMatches  = "query_matches"
)

type HTTPRouteTableConfig struct {
	DefaultHTTPInstance string                       `json:"default_http_instance,omitempty" description:"Default HTTP client instance used when a route or upstream does not specify one."`
	Upstreams           map[string]HTTPRouteUpstream `json:"upstreams,omitempty" additionalProperties.type:"object" description:"Reusable upstream definitions by key."`
	Routes              []HTTPRoute                  `json:"routes" required:"true" minItems:"1" description:"Route rules compiled in memory for fast matching."`
	AddHeaders          map[string]string            `json:"add_headers,omitempty" additionalProperties.type:"string" description:"Headers added to every proxied request."`
	StripHeaders        []string                     `json:"strip_headers,omitempty" description:"Headers removed before proxying."`
}

type HTTPRouteUpstream struct {
	URL          string            `json:"url" required:"true" minLength:"1" description:"Base upstream URL."`
	HTTPInstance string            `json:"http_instance,omitempty" description:"HTTP client instance used for this upstream."`
	AddHeaders   map[string]string `json:"add_headers,omitempty" additionalProperties.type:"string" description:"Headers added when this upstream is selected."`
	StripHeaders []string          `json:"strip_headers,omitempty" description:"Headers removed when this upstream is selected."`
}

type HTTPRoute struct {
	Name         string            `json:"name" required:"true" minLength:"1" description:"Readable route name."`
	URIs         []string          `json:"uris" required:"true" minItems:"1" description:"Complete URI patterns (protocol + host + path) that share this route configuration. Supports * as protocol, wildcard hosts like *.example.com, and path parameters, *, and **."`
	Methods      []string          `json:"methods,omitempty" description:"Allowed HTTP methods. Empty or * allows every method."`
	Upstream     string            `json:"upstream,omitempty" description:"Optional upstream key or absolute upstream URL. The HTTPProxy base_url overrides it."`
	HTTPInstance string            `json:"http_instance,omitempty" description:"HTTP client instance used for this route."`
	Rewrite      string            `json:"rewrite,omitempty" description:"Optional target path template. Supports {param}, {wildcard}, and {path}."`
	Policies     []HTTPRoutePolicy `json:"policies,omitempty" description:"Policies evaluated before proxying. String shorthand is accepted as a registered policy name."`
	Metadata     map[string]any    `json:"metadata,omitempty" description:"Developer metadata returned with the route match."`
	AddHeaders   map[string]string `json:"add_headers,omitempty" additionalProperties.type:"string" description:"Headers added when this route matches."`
	StripHeaders []string          `json:"strip_headers,omitempty" description:"Headers removed when this route matches."`
}

type HTTPRoutePolicy struct {
	Name   string         `json:"name,omitempty" description:"Registered custom policy name. Used when type is empty."`
	Type   string         `json:"type,omitempty" enum:"header_present,header_equals,header_matches,query_present,query_equals,query_matches" description:"Built-in policy type."`
	Config map[string]any `json:"config,omitempty" description:"Policy configuration."`
}

func (policy *HTTPRoutePolicy) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "" || raw == "null" {
		return nil
	}
	if strings.HasPrefix(raw, "\"") {
		var name string
		if err := json.Unmarshal(data, &name); err != nil {
			return err
		}
		policy.Name = name
		return nil
	}
	type alias HTTPRoutePolicy
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*policy = HTTPRoutePolicy(decoded)
	return nil
}

func (policy HTTPRoutePolicy) key() string {
	if value := strings.TrimSpace(policy.Type); value != "" {
		return value
	}
	return strings.TrimSpace(policy.Name)
}

type HTTPRouteTable struct {
	config HTTPRouteTableConfig
	tree   compiledHTTPRouteTree
	groups []compiledHTTPRouteGroup
}

type HTTPRouteMatch struct {
	Route        HTTPRoute
	Upstream     HTTPRouteUpstream
	Params       map[string]string
	Wildcard     string
	RequestPath  string
	RequestURI   string
	HTTPInstance string
	Metadata     map[string]any
}

type pathTokenKind uint8

const (
	pathLiteral pathTokenKind = iota
	pathParam
	pathWildcard
	pathRest
)

type pathToken struct {
	kind  pathTokenKind
	value string
}

type compiledHTTPRouteGroup struct {
	route    HTTPRoute
	upstream HTTPRouteUpstream
	methods  []string
}

type compiledHTTPRouteRef struct {
	groupIndex int
	order      int
	uriOrder   int
	pattern    string
	paramNames []string
}

type compiledHTTPRouteTree struct {
	schemes   map[string]*compiledHTTPSchemeNode
	anyScheme *compiledHTTPSchemeNode
}

type compiledHTTPSchemeNode struct {
	hosts   compiledHTTPHostNode
	anyHost *compiledHTTPPathNode
}

type compiledHTTPHostNode struct {
	children map[string]*compiledHTTPHostNode
	exact    *compiledHTTPPathNode
	wildcard *compiledHTTPPathNode
}

type compiledHTTPPathNode struct {
	literals map[string]*compiledHTTPPathNode
	param    *compiledHTTPPathNode
	wildcard *compiledHTTPPathNode
	rest     *compiledHTTPPathNode
	terminal compiledHTTPRouteTerminal
	minRef   *compiledHTTPRouteRef
}

type compiledHTTPRouteTerminal struct {
	byMethod map[string][]*compiledHTTPRouteRef
}

type compiledHTTPPathMatch struct {
	ref      *compiledHTTPRouteRef
	captures []string
	wildcard string
}

type HTTPProxyPolicy interface {
	Evaluate(context.Context, *types.JourneyTransaction, *HTTPRouteMatch, HTTPRoutePolicy) error
}

type HTTPProxyPolicyFunc func(context.Context, *types.JourneyTransaction, *HTTPRouteMatch, HTTPRoutePolicy) error

func (policy HTTPProxyPolicyFunc) Evaluate(ctx context.Context, transaction *types.JourneyTransaction, match *HTTPRouteMatch, config HTTPRoutePolicy) error {
	return policy(ctx, transaction, match, config)
}

var httpProxyPolicies = struct {
	sync.RWMutex
	values map[string]HTTPProxyPolicy
}{values: map[string]HTTPProxyPolicy{}}

func RegisterHTTPProxyPolicy(name string, policy HTTPProxyPolicy) {
	name = strings.TrimSpace(name)
	if name == "" || policy == nil {
		return
	}
	httpProxyPolicies.Lock()
	defer httpProxyPolicies.Unlock()
	httpProxyPolicies.values[name] = policy
}

func registerHTTPRouteBuiltInPolicies() {
	RegisterHTTPProxyPolicy(httpRoutePolicyHeaderPresent, HTTPProxyPolicyFunc(func(_ context.Context, transaction *types.JourneyTransaction, _ *HTTPRouteMatch, policy HTTPRoutePolicy) error {
		name, err := httpRoutePolicyRequiredString(policy, "name")
		if err != nil {
			return err
		}
		if httpRoutePolicyRequestHeader(transaction, name) == "" {
			return fmt.Errorf("header %q is required", name)
		}
		return nil
	}))
	RegisterHTTPProxyPolicy(httpRoutePolicyHeaderEquals, HTTPProxyPolicyFunc(func(_ context.Context, transaction *types.JourneyTransaction, _ *HTTPRouteMatch, policy HTTPRoutePolicy) error {
		name, err := httpRoutePolicyRequiredString(policy, "name")
		if err != nil {
			return err
		}
		expected, err := httpRoutePolicyRequiredString(policy, "value")
		if err != nil {
			return err
		}
		return httpRoutePolicyRequireEquals("header", name, httpRoutePolicyRequestHeader(transaction, name), expected)
	}))
	RegisterHTTPProxyPolicy(httpRoutePolicyHeaderMatches, HTTPProxyPolicyFunc(func(_ context.Context, transaction *types.JourneyTransaction, _ *HTTPRouteMatch, policy HTTPRoutePolicy) error {
		name, err := httpRoutePolicyRequiredString(policy, "name")
		if err != nil {
			return err
		}
		pattern, err := httpRoutePolicyRequiredString(policy, "pattern")
		if err != nil {
			return err
		}
		return httpRoutePolicyRequireMatches("header", name, httpRoutePolicyRequestHeader(transaction, name), pattern)
	}))
	RegisterHTTPProxyPolicy(httpRoutePolicyQueryPresent, HTTPProxyPolicyFunc(func(_ context.Context, transaction *types.JourneyTransaction, _ *HTTPRouteMatch, policy HTTPRoutePolicy) error {
		name, err := httpRoutePolicyRequiredString(policy, "name")
		if err != nil {
			return err
		}
		if httpRoutePolicyRequestQuery(transaction, name) == "" {
			return fmt.Errorf("query parameter %q is required", name)
		}
		return nil
	}))
	RegisterHTTPProxyPolicy(httpRoutePolicyQueryEquals, HTTPProxyPolicyFunc(func(_ context.Context, transaction *types.JourneyTransaction, _ *HTTPRouteMatch, policy HTTPRoutePolicy) error {
		name, err := httpRoutePolicyRequiredString(policy, "name")
		if err != nil {
			return err
		}
		expected, err := httpRoutePolicyRequiredString(policy, "value")
		if err != nil {
			return err
		}
		return httpRoutePolicyRequireEquals("query parameter", name, httpRoutePolicyRequestQuery(transaction, name), expected)
	}))
	RegisterHTTPProxyPolicy(httpRoutePolicyQueryMatches, HTTPProxyPolicyFunc(func(_ context.Context, transaction *types.JourneyTransaction, _ *HTTPRouteMatch, policy HTTPRoutePolicy) error {
		name, err := httpRoutePolicyRequiredString(policy, "name")
		if err != nil {
			return err
		}
		pattern, err := httpRoutePolicyRequiredString(policy, "pattern")
		if err != nil {
			return err
		}
		return httpRoutePolicyRequireMatches("query parameter", name, httpRoutePolicyRequestQuery(transaction, name), pattern)
	}))
}

func httpRoutePolicyRequiredString(policy HTTPRoutePolicy, key string) (string, error) {
	value, _ := policy.Config[key].(string)
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("policy %q requires config.%s", policy.key(), key)
	}
	return value, nil
}

func httpRoutePolicyRequestHeader(transaction *types.JourneyTransaction, name string) string {
	if transaction == nil || transaction.Request == nil {
		return ""
	}
	return transaction.Request.HeaderFirst(name)
}

func httpRoutePolicyRequestQuery(transaction *types.JourneyTransaction, name string) string {
	if transaction == nil || transaction.Request == nil {
		return ""
	}
	return transaction.Request.QueryFirst(name)
}

func httpRoutePolicyRequireEquals(kind, name, value, expected string) error {
	if value != expected {
		return fmt.Errorf("%s %q must equal %q", kind, name, expected)
	}
	return nil
}

func httpRoutePolicyRequireMatches(kind, name, value, pattern string) error {
	expression, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid pattern for %s %q: %w", kind, name, err)
	}
	if !expression.MatchString(value) {
		return fmt.Errorf("%s %q must match %q", kind, name, pattern)
	}
	return nil
}

func HTTPRouteTableFactory(raw json.RawMessage) (any, error) {
	var config HTTPRouteTableConfig
	if len(raw) != 0 {
		if err := json.Unmarshal(raw, &config); err != nil {
			return nil, err
		}
	}
	return NewHTTPRouteTable(&config)
}

func NewHTTPRouteTable(config *HTTPRouteTableConfig) (*HTTPRouteTable, error) {
	if config == nil {
		config = &HTTPRouteTableConfig{}
	}
	table := &HTTPRouteTable{
		config: cloneHTTPRouteTableConfig(*config),
		tree: compiledHTTPRouteTree{
			schemes: map[string]*compiledHTTPSchemeNode{},
		},
	}
	for index, route := range table.config.Routes {
		if err := table.compileRoute(route, index); err != nil {
			return nil, err
		}
	}
	return table, nil
}

// MatchURI matches protocol, host, and path as one routing constraint.
func (table *HTTPRouteTable) MatchURI(rawURI, method string) (*HTTPRouteMatch, bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURI))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, false
	}
	return table.match(parsed.Scheme, parsed.Host, method, parsed.EscapedPath(), parsed.String())
}

func (table *HTTPRouteTable) match(scheme, host, method, path, rawURI string) (*HTTPRouteMatch, bool) {
	if table == nil {
		return nil, false
	}
	scheme = strings.ToLower(strings.TrimSpace(scheme))
	host = normalizeHTTPRouteHost(host)
	method = strings.ToUpper(strings.TrimSpace(method))
	path = cleanHTTPRoutePath(path)
	parts := splitHTTPRoutePath(path)
	best := compiledHTTPPathMatch{}
	if schemeNode := table.tree.schemes[scheme]; schemeNode != nil {
		schemeNode.matchHost(host, parts, method, &best)
	}
	if table.tree.anyScheme != nil {
		table.tree.anyScheme.matchHost(host, parts, method, &best)
	}
	if best.ref == nil {
		return nil, false
	}

	group := table.groups[best.ref.groupIndex]
	route := group.route
	route.URIs = []string{best.ref.pattern}
	params := make(map[string]string, len(best.ref.paramNames))
	for index, name := range best.ref.paramNames {
		if index < len(best.captures) {
			params[name] = best.captures[index]
		}
	}
	instanceID := firstNonEmpty(route.HTTPInstance, group.upstream.HTTPInstance, table.config.DefaultHTTPInstance)
	return &HTTPRouteMatch{
		Route: route, Upstream: group.upstream, Params: params, Wildcard: best.wildcard, RequestPath: path,
		RequestURI: rawURI, HTTPInstance: instanceID, Metadata: cloneHTTPRouteAnyMap(route.Metadata),
	}, true
}

func (table *HTTPRouteTable) TargetURI(match *HTTPRouteMatch, rawQuery string) (string, error) {
	if table == nil || match == nil {
		return "", fmt.Errorf("route match is required")
	}
	base := strings.TrimSpace(match.Upstream.URL)
	if base == "" {
		base = strings.TrimSpace(match.Route.Upstream)
	}
	if base == "" {
		if match.RequestURI == "" {
			return "", fmt.Errorf("matched route has no upstream target")
		}
		parsed, err := url.Parse(match.RequestURI)
		if err != nil {
			return "", err
		}
		parsed.RawQuery = rawQuery
		return parsed.String(), nil
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	targetPath := match.RequestPath
	if strings.TrimSpace(match.Route.Rewrite) != "" {
		targetPath = applyHTTPRouteRewrite(match.Route.Rewrite, match)
	}
	parsed.Path = joinHTTPURLPath(parsed.Path, targetPath)
	parsed.RawQuery = rawQuery
	return parsed.String(), nil
}

func (table *HTTPRouteTable) EvaluatePolicies(ctx context.Context, transaction *types.JourneyTransaction, match *HTTPRouteMatch) error {
	if match == nil {
		return nil
	}
	for _, routePolicy := range match.Route.Policies {
		name := routePolicy.key()
		if name == "" {
			continue
		}
		httpProxyPolicies.RLock()
		policy := httpProxyPolicies.values[name]
		httpProxyPolicies.RUnlock()
		if policy == nil {
			return fmt.Errorf("HTTP proxy policy %q is not registered", name)
		}
		if err := policy.Evaluate(ctx, transaction, match, routePolicy); err != nil {
			return err
		}
	}
	return nil
}

func (table *HTTPRouteTable) RouteHeaders(requestHeaders map[string][]string, match *HTTPRouteMatch) map[string]string {
	headers := map[string]string{}
	for key, values := range requestHeaders {
		if len(values) == 0 {
			continue
		}
		headers[key] = strings.Join(values, ", ")
	}

	for _, header := range hopByHopHeaders() {
		deleteHeaderCaseInsensitive(headers, header)
	}
	if table != nil {
		for _, header := range table.config.StripHeaders {
			deleteHeaderCaseInsensitive(headers, header)
		}
		for key, value := range table.config.AddHeaders {
			headers[key] = value
		}
	}
	if match != nil {
		for _, header := range match.Upstream.StripHeaders {
			deleteHeaderCaseInsensitive(headers, header)
		}
		for key, value := range match.Upstream.AddHeaders {
			headers[key] = value
		}
		for _, header := range match.Route.StripHeaders {
			deleteHeaderCaseInsensitive(headers, header)
		}
		for key, value := range match.Route.AddHeaders {
			headers[key] = value
		}
	}
	return headers
}

func (table *HTTPRouteTable) compileRoute(route HTTPRoute, order int) error {
	if strings.TrimSpace(route.Name) == "" {
		return fmt.Errorf("route name is required")
	}
	upstream := table.config.Upstreams[route.Upstream]
	if upstream.URL == "" && route.Upstream != "" && !looksLikeURL(route.Upstream) {
		return fmt.Errorf("route %q references unknown upstream %q", route.Name, route.Upstream)
	}
	if upstream.URL == "" {
		upstream.URL = route.Upstream
	}

	uris := dedupeHTTPRouteStrings(route.URIs)
	if len(uris) == 0 {
		return fmt.Errorf("route %q uris is required", route.Name)
	}
	groupIndex := len(table.groups)
	group := compiledHTTPRouteGroup{
		route: route, upstream: upstream, methods: normalizeHTTPRouteMethods(route.Methods),
	}
	table.groups = append(table.groups, group)

	for uriOrder, pattern := range uris {
		scheme, host, path, err := parseHTTPRouteURIPattern(pattern)
		if err != nil {
			return fmt.Errorf("route %q uri %q: %w", route.Name, pattern, err)
		}
		tokens, _, err := compileHTTPRoutePath(path)
		if err != nil {
			return fmt.Errorf("route %q uri %q: %w", route.Name, pattern, err)
		}
		ref := &compiledHTTPRouteRef{
			groupIndex: groupIndex, order: order, uriOrder: uriOrder, pattern: pattern,
		}
		pathRoot := table.tree.schemeFor(scheme).pathForHost(host)
		pathRoot.insert(tokens, group.methods, ref)
	}
	return nil
}

func (tree *compiledHTTPRouteTree) schemeFor(scheme string) *compiledHTTPSchemeNode {
	if scheme == "*" {
		if tree.anyScheme == nil {
			tree.anyScheme = &compiledHTTPSchemeNode{}
		}
		return tree.anyScheme
	}
	node := tree.schemes[scheme]
	if node == nil {
		node = &compiledHTTPSchemeNode{}
		tree.schemes[scheme] = node
	}
	return node
}

func (scheme *compiledHTTPSchemeNode) pathForHost(host string) *compiledHTTPPathNode {
	if host == "*" {
		if scheme.anyHost == nil {
			scheme.anyHost = &compiledHTTPPathNode{}
		}
		return scheme.anyHost
	}

	wildcard := strings.HasPrefix(host, "*.")
	if wildcard {
		host = strings.TrimPrefix(host, "*.")
	}
	node := &scheme.hosts
	labels := splitHTTPRouteHost(host)
	for index := len(labels) - 1; index >= 0; index-- {
		if node.children == nil {
			node.children = map[string]*compiledHTTPHostNode{}
		}
		child := node.children[labels[index]]
		if child == nil {
			child = &compiledHTTPHostNode{}
			node.children[labels[index]] = child
		}
		node = child
	}
	if wildcard {
		if node.wildcard == nil {
			node.wildcard = &compiledHTTPPathNode{}
		}
		return node.wildcard
	}
	if node.exact == nil {
		node.exact = &compiledHTTPPathNode{}
	}
	return node.exact
}

func splitHTTPRouteHost(host string) []string {
	host = strings.Trim(normalizeHTTPRouteHost(host), ".")
	if host == "" {
		return nil
	}
	return strings.Split(host, ".")
}

func parseHTTPRouteURIPattern(pattern string) (string, string, string, error) {
	pattern = strings.TrimSpace(pattern)
	separator := strings.Index(pattern, "://")
	if separator <= 0 {
		return "", "", "", fmt.Errorf("must include protocol://host/path")
	}
	scheme := strings.ToLower(strings.TrimSpace(pattern[:separator]))
	remainder := pattern[separator+3:]
	slash := strings.Index(remainder, "/")
	host, path := remainder, "/"
	if slash >= 0 {
		host, path = remainder[:slash], remainder[slash:]
	}
	host = normalizeHTTPRouteHost(host)
	if scheme == "" || host == "" {
		return "", "", "", fmt.Errorf("protocol and host are required")
	}
	if scheme != "*" {
		if matched, _ := regexp.MatchString(`^[a-z][a-z0-9+.-]*$`, scheme); !matched {
			return "", "", "", fmt.Errorf("invalid protocol %q", scheme)
		}
	}
	if strings.ContainsAny(path, "?#") {
		return "", "", "", fmt.Errorf("query strings and fragments are not supported in route patterns")
	}
	return scheme, host, cleanHTTPRoutePath(path), nil
}

func dedupeHTTPRouteStrings(values []string) []string {
	result := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := value
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, value)
	}
	return result
}

func compileHTTPRoutePath(pattern string) ([]pathToken, int, error) {
	parts := splitHTTPRoutePath(pattern)
	tokens := make([]pathToken, 0, len(parts))
	specificity := 0
	for index, part := range parts {
		switch {
		case part == "**":
			if index != len(parts)-1 {
				return nil, 0, fmt.Errorf("** wildcard must be the last path segment")
			}
			tokens = append(tokens, pathToken{kind: pathRest})
			specificity--
		case part == "*":
			tokens = append(tokens, pathToken{kind: pathWildcard})
			specificity++
		case strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") && len(part) > 2:
			tokens = append(tokens, pathToken{kind: pathParam, value: strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")})
			specificity += 3
		default:
			tokens = append(tokens, pathToken{kind: pathLiteral, value: part})
			specificity += 10
		}
	}
	return tokens, specificity, nil
}

func (node *compiledHTTPPathNode) insert(tokens []pathToken, methods []string, ref *compiledHTTPRouteRef) {
	node.updateMinRef(ref)
	for _, token := range tokens {
		switch token.kind {
		case pathLiteral:
			if node.literals == nil {
				node.literals = map[string]*compiledHTTPPathNode{}
			}
			child := node.literals[token.value]
			if child == nil {
				child = &compiledHTTPPathNode{}
				node.literals[token.value] = child
			}
			node = child
		case pathParam:
			if node.param == nil {
				node.param = &compiledHTTPPathNode{}
			}
			ref.paramNames = append(ref.paramNames, token.value)
			node = node.param
		case pathWildcard:
			if node.wildcard == nil {
				node.wildcard = &compiledHTTPPathNode{}
			}
			node = node.wildcard
		case pathRest:
			if node.rest == nil {
				node.rest = &compiledHTTPPathNode{}
			}
			node = node.rest
		}
		node.updateMinRef(ref)
	}
	if node.terminal.byMethod == nil {
		node.terminal.byMethod = map[string][]*compiledHTTPRouteRef{}
	}
	for _, method := range methods {
		node.terminal.byMethod[method] = append(node.terminal.byMethod[method], ref)
	}
}

func (node *compiledHTTPPathNode) updateMinRef(ref *compiledHTTPRouteRef) {
	if node.minRef == nil || compiledHTTPRouteRefLess(ref, node.minRef) {
		node.minRef = ref
	}
}

func compiledHTTPRouteRefLess(left, right *compiledHTTPRouteRef) bool {
	if right == nil {
		return left != nil
	}
	if left == nil {
		return false
	}
	if left.order != right.order {
		return left.order < right.order
	}
	return left.uriOrder < right.uriOrder
}

func (scheme *compiledHTTPSchemeNode) matchHost(host string, parts []string, method string, best *compiledHTTPPathMatch) {
	if scheme.anyHost != nil {
		scheme.anyHost.match(parts, 0, method, nil, "", best)
	}

	labels := splitHTTPRouteHost(host)
	node := &scheme.hosts
	for index := len(labels) - 1; index >= 0; index-- {
		node = node.children[labels[index]]
		if node == nil {
			return
		}
		remainingLabels := index
		if node.wildcard != nil && remainingLabels > 0 {
			node.wildcard.match(parts, 0, method, nil, "", best)
		}
		if remainingLabels == 0 && node.exact != nil {
			node.exact.match(parts, 0, method, nil, "", best)
		}
	}
}

func (node *compiledHTTPPathNode) match(parts []string, index int, method string, captures []string, wildcard string, best *compiledHTTPPathMatch) {
	if node == nil || (best.ref != nil && !compiledHTTPRouteRefLess(node.minRef, best.ref)) {
		return
	}
	if index == len(parts) {
		node.terminal.selectMatch(method, captures, wildcard, best)
		if node.rest != nil {
			node.rest.terminal.selectMatch(method, captures, "", best)
		}
		return
	}

	if child := node.literals[parts[index]]; child != nil {
		child.match(parts, index+1, method, captures, wildcard, best)
	}
	if node.param != nil {
		node.param.match(parts, index+1, method, append(captures, parts[index]), wildcard, best)
	}
	if node.wildcard != nil {
		node.wildcard.match(parts, index+1, method, captures, wildcard, best)
	}
	if node.rest != nil {
		node.rest.terminal.selectMatch(method, captures, strings.Join(parts[index:], "/"), best)
	}
}

func (terminal *compiledHTTPRouteTerminal) selectMatch(method string, captures []string, wildcard string, best *compiledHTTPPathMatch) {
	if terminal == nil {
		return
	}
	var candidate *compiledHTTPRouteRef
	if routes := terminal.byMethod[method]; len(routes) != 0 {
		candidate = routes[0]
	}
	if method != "*" {
		if routes := terminal.byMethod["*"]; len(routes) != 0 && compiledHTTPRouteRefLess(routes[0], candidate) {
			candidate = routes[0]
		}
	}
	if candidate == nil || (best.ref != nil && !compiledHTTPRouteRefLess(candidate, best.ref)) {
		return
	}
	best.ref = candidate
	best.captures = append(best.captures[:0], captures...)
	best.wildcard = wildcard
}

func normalizeHTTPRouteMethods(methods []string) []string {
	if len(methods) == 0 {
		return []string{"*"}
	}
	result := make([]string, 0, len(methods))
	seen := map[string]bool{}
	for _, method := range methods {
		method = strings.ToUpper(strings.TrimSpace(method))
		if method == "" {
			continue
		}
		if method == "*" {
			return []string{"*"}
		}
		if !seen[method] {
			seen[method] = true
			result = append(result, method)
		}
	}
	if len(result) == 0 {
		return []string{"*"}
	}
	return result
}

func normalizeHTTPRouteHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return ""
	}
	if withoutPort, _, err := net.SplitHostPort(host); err == nil {
		return withoutPort
	}
	return host
}

func cleanHTTPRoutePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

func splitHTTPRoutePath(path string) []string {
	path = strings.Trim(cleanHTTPRoutePath(path), "/")
	if path == "" {
		return []string{}
	}
	return strings.Split(path, "/")
}

func applyHTTPRouteRewrite(template string, match *HTTPRouteMatch) string {
	value := template
	for key, param := range match.Params {
		value = strings.ReplaceAll(value, "{"+key+"}", param)
	}
	value = strings.ReplaceAll(value, "{wildcard}", match.Wildcard)
	value = strings.ReplaceAll(value, "{path}", strings.TrimPrefix(match.RequestPath, "/"))
	return cleanHTTPRoutePath(value)
}

func joinHTTPURLPath(base, route string) string {
	base = strings.TrimRight(base, "/")
	route = strings.TrimLeft(route, "/")
	if base == "" {
		return "/" + route
	}
	if route == "" {
		return base
	}
	return base + "/" + route
}

func deleteHeaderCaseInsensitive(headers map[string]string, name string) {
	for key := range headers {
		if strings.EqualFold(key, name) {
			delete(headers, key)
		}
	}
}

func hopByHopHeaders() []string {
	return []string{"Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade"}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func looksLikeURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	return err == nil && parsed.Scheme != "" && parsed.Host != ""
}

func cloneHTTPRouteTableConfig(source HTTPRouteTableConfig) HTTPRouteTableConfig {
	target := source
	target.AddHeaders = cloneStringMap(source.AddHeaders)
	target.StripHeaders = append([]string(nil), source.StripHeaders...)
	target.Upstreams = map[string]HTTPRouteUpstream{}
	for key, upstream := range source.Upstreams {
		upstream.AddHeaders = cloneStringMap(upstream.AddHeaders)
		upstream.StripHeaders = append([]string(nil), upstream.StripHeaders...)
		target.Upstreams[key] = upstream
	}
	target.Routes = append([]HTTPRoute(nil), source.Routes...)
	for index := range target.Routes {
		target.Routes[index].URIs = append([]string(nil), source.Routes[index].URIs...)
		target.Routes[index].Methods = append([]string(nil), source.Routes[index].Methods...)
		target.Routes[index].Policies = cloneHTTPRoutePolicies(source.Routes[index].Policies)
		target.Routes[index].Metadata = cloneHTTPRouteAnyMap(source.Routes[index].Metadata)
		target.Routes[index].AddHeaders = cloneStringMap(source.Routes[index].AddHeaders)
		target.Routes[index].StripHeaders = append([]string(nil), source.Routes[index].StripHeaders...)
	}
	return target
}

func cloneHTTPRoutePolicies(source []HTTPRoutePolicy) []HTTPRoutePolicy {
	if len(source) == 0 {
		return nil
	}
	target := make([]HTTPRoutePolicy, len(source))
	for index, policy := range source {
		target[index] = policy
		target[index].Config = cloneHTTPRouteAnyMap(policy.Config)
	}
	return target
}

func cloneHTTPRouteAnyMap(source map[string]any) map[string]any {
	if len(source) == 0 {
		return map[string]any{}
	}
	target := make(map[string]any, len(source))
	for key, value := range source {
		target[key] = cloneHTTPRouteAnyValue(value)
	}
	return target
}

func cloneHTTPRouteAnySlice(source []any) []any {
	if len(source) == 0 {
		return nil
	}
	target := make([]any, len(source))
	for index, value := range source {
		target[index] = cloneHTTPRouteAnyValue(value)
	}
	return target
}

func cloneHTTPRouteAnyValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneHTTPRouteAnyMap(typed)
	case []any:
		return cloneHTTPRouteAnySlice(typed)
	case []string:
		return append([]string(nil), typed...)
	case map[string]string:
		return cloneStringMap(typed)
	default:
		return value
	}
}

func cloneStringMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return map[string]string{}
	}
	target := make(map[string]string, len(source))
	for key, value := range source {
		target[key] = value
	}
	return target
}

func init() {
	registerHTTPRouteBuiltInPolicies()
	RegisterInstance(&InstanceDefinition{
		Key:          HTTPRouteTableCacheKey,
		Config:       &HTTPRouteTableConfig{},
		Factory:      HTTPRouteTableFactory,
		MaxInstances: 50,
		Description:  "Compiled HTTP route table used by resource journeys to proxy incoming requests.",
	}, map[string]map[string]any{
		".": {"x-category": "network", "x-order": []string{"default_http_instance", "upstreams", "routes", "add_headers", "strip_headers"}},
	})
}
