package steps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	goconf "github.com/nitsugaro/go-conf"
	jcache "github.com/nitsugaro/go-journey/cache"
	jenv "github.com/nitsugaro/go-journey/env"
	"github.com/nitsugaro/go-journey/inputs"
	"github.com/nitsugaro/go-journey/types"
	jsrun "github.com/nitsugaro/go-jsruntime/v2"
	"github.com/nitsugaro/go-utils/crypto"
	"github.com/nitsugaro/go-utils/encoding"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type Script struct {
	BasicStep

	_              struct{}          `description:"Execute a custom script with custom outcomes."`
	ScriptID       string            `json:"script_id" required:"true" format:"uuid" description:"Script to run in current step."`
	TimeoutSeconds int               `json:"timeout_seconds" default:"60" minimum:"1" description:"Maximum script execution time in seconds."`
	Args           map[string]any    `json:"args,omitempty" description:"Values exposed to the script through the args binding."`
	Outcome        map[string]string `json:"outcome" required:"true"`
}

const (
	JourneyScript          = "auth"
	ResourceScript         = "resource"
	WorkflowScript         = "workflow"
	ScheduleScript         = "schedule"
	LibraryScript          = "library"
	AsyncScript            = "async"
	LegacyJourneyScript    = "journey"
	ScriptManagerCacheKey  = "script_manager"
	ScriptStorageCacheKey  = "script_storage"
	ScriptBindingsCacheKey = "script_bindings"
	ScheduleCacheKey       = "schedule_cache"
	ScriptOutcomesProp     = "outcomes"
)

type ScriptBindingsMode string

const (
	ScriptBindingsExtend  ScriptBindingsMode = "extend"
	ScriptBindingsReplace ScriptBindingsMode = "replace"
)

type ScriptBindingContext struct {
	Transaction     *types.JourneyTransaction
	Args            map[string]any
	ScriptType      string
	DefaultBindings map[string]any
}

type ScriptBindingDescriptor struct {
	Name        string                    `json:"name"`
	Type        string                    `json:"type"`
	Signature   string                    `json:"signature,omitempty"`
	Description string                    `json:"description,omitempty"`
	Example     string                    `json:"example,omitempty"`
	Children    []ScriptBindingDescriptor `json:"children,omitempty"`
}

type ScriptBindingSetDescriptor struct {
	Type        string                    `json:"type"`
	Name        string                    `json:"name,omitempty"`
	Description string                    `json:"description,omitempty"`
	Runnable    bool                      `json:"runnable"`
	Bindings    []ScriptBindingDescriptor `json:"bindings"`
}

type ScriptBindingDescriptorContext struct {
	Realm           string
	Script          *jsrun.Script
	ScriptType      string
	DefaultBindings []ScriptBindingDescriptor
}

type ScriptBindingsProvider interface {
	Bindings(*ScriptBindingContext) (map[string]any, error)
}

type ScriptBindingsDescriptorProvider interface {
	BindingDescriptors(*ScriptBindingDescriptorContext) ([]ScriptBindingDescriptor, error)
}

type ScriptRuntimeBindingsProvider interface {
	GetBindings(*ScriptBindingContext) (map[string]any, error)
}

type ScriptHTTPBinding struct {
	transaction  *types.JourneyTransaction
	cacheManager *jcache.Manager
	context      context.Context
}

type ScriptScheduleCacheBinding struct {
	cacheManager *jcache.Manager
	context      context.Context
	realm        string
}

type ScriptTypeDefinition struct {
	Type        string
	Name        string
	Description string
	Runnable    bool
}

type configuredScriptBindings struct {
	mode     ScriptBindingsMode
	provider any
}

var scriptTypesRegistry = struct {
	sync.RWMutex
	values map[string]ScriptTypeDefinition
}{
	values: map[string]ScriptTypeDefinition{
		JourneyScript: {
			Type:        JourneyScript,
			Name:        "Auth script",
			Description: "Runs inside auth journey steps with context, outcome, client input and request bindings.",
			Runnable:    true,
		},
		ResourceScript: {
			Type:        ResourceScript,
			Name:        "Resource script",
			Description: "Runs inside resource journeys with request-oriented bindings.",
			Runnable:    true,
		},
		WorkflowScript: {
			Type:        WorkflowScript,
			Name:        "Workflow script",
			Description: "Runs inside backend workflow journeys with context, args, HTTP, crypto and logger bindings.",
			Runnable:    true,
		},
		ScheduleScript: {
			Type:        ScheduleScript,
			Name:        "Schedule script",
			Description: "Runs from scheduler jobs and publishes shared values explicitly with SetResult and previousResult.",
			Runnable:    true,
		},
		LibraryScript: {
			Type:        LibraryScript,
			Name:        "Library script",
			Description: "Reusable JavaScript helpers shared by executable scripts.",
			Runnable:    false,
		},
		AsyncScript: {
			Type:        AsyncScript,
			Name:        "Async script",
			Description: "Script definition reserved for asynchronous usage. It has no default runtime bindings.",
			Runnable:    false,
		},
	},
}

var scriptTypeSortPriority = map[string]int{
	JourneyScript: 0, ResourceScript: 1, WorkflowScript: 2, ScheduleScript: 3, LibraryScript: 4, AsyncScript: 5,
}

func GetAllowedTypes() []string {
	definitions := ScriptTypeDefinitions()
	types := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		types = append(types, definition.Type)
	}
	return types
}

func IsAllowedType(typ string) bool {
	_, found := scriptTypeDefinition(NormalizeScriptType(typ))
	return found
}

func IsRunnableScriptType(typ string) bool {
	definition, found := scriptTypeDefinition(NormalizeScriptType(typ))
	return found && definition.Runnable
}

func NormalizeScriptType(scriptType string) string {
	switch strings.TrimSpace(strings.ToLower(scriptType)) {
	case LegacyJourneyScript:
		return JourneyScript
	default:
		return strings.TrimSpace(strings.ToLower(scriptType))
	}
}

func RegisterScriptType(definition ScriptTypeDefinition) error {
	definition.Type = NormalizeScriptType(definition.Type)
	if definition.Type == "" {
		return errors.New("script type is required")
	}
	if strings.ContainsAny(definition.Type, " \t\r\n") {
		return fmt.Errorf("script type %q cannot contain whitespace", definition.Type)
	}
	if strings.TrimSpace(definition.Name) == "" {
		definition.Name = definition.Type
	}
	scriptTypesRegistry.Lock()
	defer scriptTypesRegistry.Unlock()
	scriptTypesRegistry.values[definition.Type] = definition
	return nil
}

func ScriptTypeDefinitions() []ScriptTypeDefinition {
	scriptTypesRegistry.RLock()
	defer scriptTypesRegistry.RUnlock()
	definitions := make([]ScriptTypeDefinition, 0, len(scriptTypesRegistry.values))
	for _, definition := range scriptTypesRegistry.values {
		definitions = append(definitions, definition)
	}
	sort.Slice(definitions, func(i, j int) bool {
		left, leftOK := scriptTypeSortPriority[definitions[i].Type]
		right, rightOK := scriptTypeSortPriority[definitions[j].Type]
		if leftOK || rightOK {
			if !leftOK {
				left = 1000
			}
			if !rightOK {
				right = 1000
			}
			if left != right {
				return left < right
			}
		}
		return definitions[i].Type < definitions[j].Type
	})
	return definitions
}

func scriptTypeDefinition(scriptType string) (ScriptTypeDefinition, bool) {
	scriptTypesRegistry.RLock()
	defer scriptTypesRegistry.RUnlock()
	definition, found := scriptTypesRegistry.values[NormalizeScriptType(scriptType)]
	return definition, found
}

var scriptFolder atomic.Value

func ConfigureScriptRuntime(cacheManager *jcache.Manager, manager *jsrun.ScriptManager, storage *jsrun.ScriptStorage) error {
	if cacheManager == nil || manager == nil || storage == nil {
		return errors.New("script manager and storage are required")
	}
	if err := cacheManager.UpdateRuntimeCacheInstance(ScriptManagerCacheKey, jcache.DefaultInstanceID, manager, 0); err != nil {
		return err
	}
	return cacheManager.UpdateRuntimeCacheInstance(ScriptStorageCacheKey, jcache.DefaultInstanceID, storage, 0)
}

func ConfigureScriptBindings(cacheManager *jcache.Manager, mode ScriptBindingsMode, provider any) error {
	if cacheManager == nil || provider == nil {
		return errors.New("script cache manager and bindings provider are required")
	}
	if mode != ScriptBindingsExtend && mode != ScriptBindingsReplace {
		return fmt.Errorf("unsupported script bindings mode %q", mode)
	}
	return cacheManager.UpdateRuntimeCacheInstance(
		ScriptBindingsCacheKey,
		jcache.DefaultInstanceID,
		&configuredScriptBindings{mode: mode, provider: provider},
		0,
	)
}

func ConfigureScriptTypeBindings(cacheManager *jcache.Manager, scriptType string, mode ScriptBindingsMode, provider any) error {
	scriptType = NormalizeScriptType(scriptType)
	if cacheManager == nil || provider == nil {
		return errors.New("script cache manager and bindings provider are required")
	}
	if !IsAllowedType(scriptType) {
		return fmt.Errorf("unsupported script type %q", scriptType)
	}
	if mode != ScriptBindingsExtend && mode != ScriptBindingsReplace {
		return fmt.Errorf("unsupported script bindings mode %q", mode)
	}
	return cacheManager.UpdateRuntimeCacheInstance(
		ScriptBindingsCacheKey,
		scriptType,
		&configuredScriptBindings{mode: mode, provider: provider},
		0,
	)
}

func ScriptBindingDescriptors(cacheManager *jcache.Manager, realm string, scriptType string, script *jsrun.Script) ([]ScriptBindingDescriptor, error) {
	scriptType = NormalizeScriptType(scriptType)
	defaults := defaultScriptBindingDescriptors(scriptType)
	configured, found, err := configuredBindings(cacheManager, scriptType)
	if err != nil {
		return nil, err
	}
	if !found {
		return defaults, nil
	}
	descriptorProvider, ok := configured.provider.(ScriptBindingsDescriptorProvider)
	if !ok {
		return defaults, nil
	}
	custom, err := descriptorProvider.BindingDescriptors(&ScriptBindingDescriptorContext{
		Realm:           realm,
		Script:          script,
		ScriptType:      scriptType,
		DefaultBindings: cloneScriptBindingDescriptors(defaults),
	})
	if err != nil {
		return nil, err
	}
	if custom == nil {
		custom = []ScriptBindingDescriptor{}
	}
	if configured.mode == ScriptBindingsReplace {
		return custom, nil
	}
	return mergeScriptBindingDescriptors(defaults, custom), nil
}

func ScriptBindingSets(cacheManager *jcache.Manager, realm string) ([]ScriptBindingSetDescriptor, error) {
	definitions := ScriptTypeDefinitions()
	sets := make([]ScriptBindingSetDescriptor, 0, len(definitions))
	for _, definition := range definitions {
		bindings, err := ScriptBindingDescriptors(cacheManager, realm, definition.Type, nil)
		if err != nil {
			return nil, err
		}
		sets = append(sets, ScriptBindingSetDescriptor{
			Type:        definition.Type,
			Name:        definition.Name,
			Description: definition.Description,
			Runnable:    definition.Runnable,
			Bindings:    bindings,
		})
	}
	return sets, nil
}

func defaultScriptBindingDescriptors(scriptType string) []ScriptBindingDescriptor {
	scriptType = NormalizeScriptType(scriptType)
	descriptors := []ScriptBindingDescriptor{
		treeMapBindingDescriptor("ctx", "Public journey context. Values saved here can be returned to clients and reused by later steps."),
		treeMapBindingDescriptor("encCtx", "Encrypted journey context. Values saved here are persisted encrypted inside journey state."),
		treeMapBindingDescriptor("closedCtx", "Server-only closed journey context. Values saved here are not exposed to clients."),
		treeMapBindingDescriptor("tempCtx", "Execution-temporary context. Values saved here live only during the current invocation."),
		{Name: "args", Type: "object", Description: "Plain object with values configured in the Script step args map.", Example: `args.userId`},
		{
			Name: "request", Type: "object", Description: "Typed HTTP request snapshot for the current invocation.",
			Children: []ScriptBindingDescriptor{
				{Name: "RequestURI", Type: "string", Description: "Original request URI including path and query.", Example: `request.RequestURI`},
				{Name: "Path", Type: "string", Description: "Request path.", Example: `request.Path`},
				{Name: "RawQuery", Type: "string", Description: "Raw query string without the leading question mark.", Example: `request.RawQuery`},
				{Name: "QueryParameters", Type: "object", Description: "Query values as map[string][]string. Prefer requestQuery helpers for safe access.", Example: `request.QueryParameters.name`},
				{Name: "Headers", Type: "object", Description: "Headers as map[string][]string. Prefer requestHeader helpers for case-insensitive safe access.", Example: `request.Headers.Authorization`},
				{Name: "Body", Type: "object", Description: "Request body metadata and raw bytes.", Example: `request.Body.Data`},
				{Name: "Method", Type: "string", Description: "HTTP method.", Example: `request.Method`},
				{Name: "Origin", Type: "string", Description: "Origin header value when present.", Example: `request.Origin`},
				{Name: "BaseURL", Type: "string", Description: "Request base URL built from scheme and host.", Example: `request.BaseURL`},
				{Name: "Host", Type: "string", Description: "Request host.", Example: `request.Host`},
				{Name: "Port", Type: "number", Description: "Request port.", Example: `request.Port`},
				{Name: "Protocol", Type: "string", Description: "Request protocol, usually http or https.", Example: `request.Protocol`},
				{Name: "HTTPVersion", Type: "string", Description: "HTTP protocol version.", Example: `request.HTTPVersion`},
				{Name: "RemoteAddress", Type: "string", Description: "Remote client address.", Example: `request.RemoteAddress`},
				{Name: "TLSVersion", Type: "string", Description: "TLS version when request used TLS.", Example: `request.TLSVersion`},
				{Name: "Certificates", Type: "array", Description: "Client certificates snapshot when present.", Example: `request.Certificates`},
				{Name: "Cookies", Type: "array", Description: "Request cookies snapshot.", Example: `request.Cookies`},
			},
		},
		{
			Name: "requestQuery", Type: "object", Description: "Safe helpers for query parameters.",
			Children: []ScriptBindingDescriptor{
				{Name: "First", Type: "function", Signature: `First(key string, defaultValue?: string): string`, Description: "Returns the first query value or the optional default.", Example: `requestQuery.First("name", "")`},
				{Name: "All", Type: "function", Signature: `All(key string): string[]`, Description: "Returns all query values.", Example: `requestQuery.All("name")`},
				{Name: "Has", Type: "function", Signature: `Has(key string): boolean`, Description: "Returns true when a query key exists.", Example: `requestQuery.Has("name")`},
			},
		},
		{
			Name: "requestHeader", Type: "object", Description: "Safe helpers for request headers.",
			Children: []ScriptBindingDescriptor{
				{Name: "First", Type: "function", Signature: `First(key string, defaultValue?: string): string`, Description: "Returns the first header value or the optional default.", Example: `requestHeader.First("X-Request-ID", "")`},
				{Name: "All", Type: "function", Signature: `All(key string): string[]`, Description: "Returns all header values.", Example: `requestHeader.All("X-Request-ID")`},
				{Name: "Has", Type: "function", Signature: `Has(key string): boolean`, Description: "Returns true when a header exists.", Example: `requestHeader.Has("Authorization")`},
			},
		},
		{
			Name: "http", Type: "object", Description: "HTTP client helper backed by configured http_client cache instances.",
			Children: []ScriptBindingDescriptor{
				{
					Name:        "Send",
					Type:        "function",
					Signature:   `Send(url string, options?: { method?: string, headers?: object, body?: any, instance?: string }): object`,
					Description: "Sends an HTTP request using http_client/default unless options.instance selects another configured instance.",
					Example:     `const res = http.Send("https://api.example.test/users", { method: "POST", headers: { "Content-Type": "application/json" }, body: { id: args.user_id }, instance: "analytics" })`,
				},
			},
		},
		{
			Name: "scheduleCache", Type: "object", Description: "Reads and refreshes shared results produced by schedules in the current realm.",
			Children: []ScriptBindingDescriptor{
				{Name: "Get", Type: "function", Signature: `Get(schedule string, options?: { maxAgeSeconds?: number, staleIfError?: boolean }): any`, Description: "Returns a fresh cached result or executes the schedule once when it is absent or too old.", Example: `scheduleCache.Get("external-token", { maxAgeSeconds: 3300 })`},
				{Name: "Refresh", Type: "function", Signature: `Refresh(schedule string): any`, Description: "Forces the schedule to execute and atomically replaces its cached result.", Example: `scheduleCache.Refresh("external-token")`},
				{Name: "Clear", Type: "function", Signature: `Clear(schedule string): void`, Description: "Removes the cached result without executing the schedule.", Example: `scheduleCache.Clear("external-token")`},
			},
		},
		{Name: "previousResult", Type: "any", Description: "Last result published by this schedule, or null on its first execution.", Example: `previousResult`},
		{Name: "SetResult", Type: "function", Signature: `SetResult(result any): void`, Description: "Explicitly publishes the value returned by this schedule to its shared cache.", Example: `SetResult({ access_token: token.access_token })`},
		{Name: "realm", Type: "string", Description: "Current execution realm name.", Example: `realm`},
		{
			Name: "encoding", Type: "object", Description: "Encoding helper functions.",
			Children: []ScriptBindingDescriptor{
				{Name: "EncodeBase64", Type: "function", Signature: `EncodeBase64(data []byte): string`, Description: "Encodes bytes using standard Base64.", Example: `encoding.EncodeBase64(bytes)`},
				{Name: "DecodeBase64", Type: "function", Signature: `DecodeBase64(value string): []byte`, Description: "Decodes standard Base64 into bytes.", Example: `encoding.DecodeBase64("dmFsdWU=")`},
				{Name: "EncodeBase64URL", Type: "function", Signature: `EncodeBase64URL(data []byte): string`, Description: "Encodes bytes using raw Base64 URL encoding.", Example: `encoding.EncodeBase64URL(bytes)`},
				{Name: "DecodeBase64URL", Type: "function", Signature: `DecodeBase64URL(value string): []byte`, Description: "Decodes raw Base64 URL into bytes.", Example: `encoding.DecodeBase64URL("dmFsdWU")`},
				{Name: "EncodeHex", Type: "function", Signature: `EncodeHex(data []byte): string`, Description: "Encodes bytes as hexadecimal.", Example: `encoding.EncodeHex(bytes)`},
				{Name: "DecodeHex", Type: "function", Signature: `DecodeHex(value string): []byte`, Description: "Decodes hexadecimal into bytes.", Example: `encoding.DecodeHex("76616c7565")`},
			},
		},
		{
			Name: "crypto", Type: "object", Description: "Cryptographic helper functions.",
			Children: []ScriptBindingDescriptor{
				{Name: "NewUUID", Type: "function", Signature: `NewUUID(): string`, Description: "Generates a random UUID string.", Example: `crypto.NewUUID()`},
				{Name: "GetRandBytes", Type: "function", Signature: `GetRandBytes(size int): []byte`, Description: "Generates cryptographically secure random bytes.", Example: `crypto.GetRandBytes(32)`},
				{Name: "SHA1", Type: "function", Signature: `SHA1(value string): []byte`, Description: "Returns SHA-1 bytes for a string.", Example: `crypto.SHA1("value")`},
				{Name: "SHA256", Type: "function", Signature: `SHA256(value string): []byte`, Description: "Returns SHA-256 bytes for a string.", Example: `crypto.SHA256("value")`},
				{Name: "SHA384", Type: "function", Signature: `SHA384(value string): []byte`, Description: "Returns SHA-384 bytes for a string.", Example: `crypto.SHA384("value")`},
				{Name: "SHA512", Type: "function", Signature: `SHA512(value string): []byte`, Description: "Returns SHA-512 bytes for a string.", Example: `crypto.SHA512("value")`},
				{Name: "HmacSHA1", Type: "function", Signature: `HmacSHA1(value string, base64urlKey string): []byte`, Description: "Returns HMAC-SHA1 bytes using a raw Base64 URL encoded key.", Example: `crypto.HmacSHA1("value", "base64url-secret")`},
				{Name: "HmacSHA256", Type: "function", Signature: `HmacSHA256(value string, base64urlKey string): []byte`, Description: "Returns HMAC-SHA256 bytes using a raw Base64 URL encoded key.", Example: `crypto.HmacSHA256("value", "base64url-secret")`},
				{Name: "HmacSHA384", Type: "function", Signature: `HmacSHA384(value string, base64urlKey string): []byte`, Description: "Returns HMAC-SHA384 bytes using a raw Base64 URL encoded key.", Example: `crypto.HmacSHA384("value", "base64url-secret")`},
				{Name: "HmacSHA512", Type: "function", Signature: `HmacSHA512(value string, base64urlKey string): []byte`, Description: "Returns HMAC-SHA512 bytes using a raw Base64 URL encoded key.", Example: `crypto.HmacSHA512("value", "base64url-secret")`},
			},
		},
		{Name: "setOutcome", Type: "function", Signature: `setOutcome(outcome string): void`, Description: "Selects the Script step outcome.", Example: `setOutcome("true")`},
		{
			Name: "clientInputs", Type: "object", Description: "Client input helpers.",
			Children: []ScriptBindingDescriptor{
				{Name: "IsClientEmpty", Type: "function", Signature: `IsClientEmpty(): boolean`, Description: "Returns true when the invocation did not include client input responses.", Example: `clientInputs.IsClientEmpty()`},
				{Name: "IsNewEmpty", Type: "function", Signature: `IsNewEmpty(): boolean`, Description: "Returns true when this script has not queued new client input requests.", Example: `clientInputs.IsNewEmpty()`},
				{Name: "GetByExternalID", Type: "function", Signature: `GetByExternalID(externalID string): ClientInput`, Description: "Returns the first received client input matching an external id.", Example: `clientInputs.GetByExternalID("profile.email")`},
				{Name: "GetByType", Type: "function", Signature: `GetByType(type string): ClientInput[]`, Description: "Returns received client inputs matching an input type.", Example: `clientInputs.GetByType("string")`},
				{Name: "AddValueInput", Type: "function", Signature: `AddValueInput(input object): error`, Description: "Queues a typed value input request for the client.", Example: `clientInputs.AddValueInput({ id: "email", type: "string", label: "Email" })`},
				{Name: "AddMessageInput", Type: "function", Signature: `AddMessageInput(message object): error`, Description: "Queues a message input request for the client.", Example: `clientInputs.AddMessageInput({ message: "Continue" })`},
			},
		},
		{
			Name: "logger", Type: "object", Description: "Structured observer event helpers.",
			Children: []ScriptBindingDescriptor{
				{Name: "Info", Type: "function", Signature: `Info(message string, attrs?: object): void`, Example: `logger.Info("loaded profile", { user_id: args.user_id })`},
				{Name: "Event", Type: "function", Signature: `Event(message string, attrs?: object): void`, Example: `logger.Event("custom checkpoint", { stage: "before_remote_call" })`},
				{Name: "Error", Type: "function", Signature: `Error(message string, error?: any, attrs?: object): void`, Example: `logger.Error("script failed", err, { script: "login" })`},
			},
		},
	}
	return scriptBindingDescriptorsForType(scriptType, descriptors)
}

func scriptBindingDescriptorsForType(scriptType string, descriptors []ScriptBindingDescriptor) []ScriptBindingDescriptor {
	switch NormalizeScriptType(scriptType) {
	case JourneyScript:
		return filterScriptBindingDescriptors(descriptors, "previousResult", "SetResult")
	case ResourceScript:
		return filterScriptBindingDescriptors(descriptors, "clientInputs", "previousResult", "SetResult")
	case WorkflowScript:
		return filterScriptBindingDescriptors(descriptors, "clientInputs", "request", "requestQuery", "requestHeader", "previousResult", "SetResult")
	case ScheduleScript:
		return filterScriptBindingDescriptors(descriptors, "ctx", "encCtx", "closedCtx", "tempCtx", "clientInputs", "request", "requestQuery", "requestHeader", "setOutcome")
	default:
		return []ScriptBindingDescriptor{}
	}
}

func filterScriptBindingDescriptors(descriptors []ScriptBindingDescriptor, excluded ...string) []ScriptBindingDescriptor {
	excludedSet := make(map[string]struct{}, len(excluded))
	for _, name := range excluded {
		excludedSet[name] = struct{}{}
	}
	filtered := make([]ScriptBindingDescriptor, 0, len(descriptors))
	for _, descriptor := range descriptors {
		if _, skip := excludedSet[descriptor.Name]; skip {
			continue
		}
		filtered = append(filtered, descriptor)
	}
	return filtered
}

func treeMapBindingDescriptor(name string, description string) ScriptBindingDescriptor {
	return ScriptBindingDescriptor{
		Name: name, Type: "TreeMap", Description: description,
		Children: treeMapBindingChildren(name),
	}
}

func treeMapBindingChildren(root string) []ScriptBindingDescriptor {
	return []ScriptBindingDescriptor{
		{Name: "Get", Type: "function", Signature: `Get(path string): TreeMap`, Description: "Returns a TreeMap positioned at a nested dot path. The result can be converted with As* methods.", Example: root + `.Get("profile.email").AsStringOr("")`},
		{Name: "IsDefined", Type: "function", Signature: `IsDefined(path string): boolean`, Description: "Returns true when the dot path exists in this TreeMap.", Example: root + `.IsDefined("profile.email")`},
		{Name: "Exists", Type: "function", Signature: `Exists(): boolean`, Description: "Returns true when the current TreeMap cursor points to an existing value.", Example: root + `.Get("profile.email").Exists()`},
		{Name: "IsEmpty", Type: "function", Signature: `IsEmpty(): boolean`, Description: "Returns true when the current value is empty or missing.", Example: root + `.Get("profile.email").IsEmpty()`},
		{Name: "Or", Type: "function", Signature: `Or(path string): TreeMap`, Description: "Returns the current TreeMap when it exists, otherwise returns another path from the same root.", Example: root + `.Get("profile.email").Or("fallback.email")`},
		{Name: "Set", Type: "function", Signature: `Set(path string, value any): TreeMap`, Description: "Sets a value at a nested dot path and returns the TreeMap.", Example: root + `.Set("profile.email", "user@example.com")`},
		{Name: "Delete", Type: "function", Signature: `Delete(path string): TreeMap`, Description: "Deletes a nested dot path and returns the TreeMap. Errors if the path cannot be deleted.", Example: root + `.Delete("profile.email")`},
		{Name: "TryDelete", Type: "function", Signature: `TryDelete(path string): TreeMap`, Description: "Deletes a nested dot path when present and ignores missing-path errors.", Example: root + `.TryDelete("profile.email")`},
		{Name: "Clone", Type: "function", Signature: `Clone(): TreeMap`, Description: "Returns a deep copy of the current TreeMap value.", Example: root + `.Clone()`},
		{Name: "ToJsonString", Type: "function", Signature: `ToJsonString(pretty boolean): string`, Description: "Serializes the current TreeMap value as JSON.", Example: root + `.ToJsonString(true)`},
		{Name: "AsMap", Type: "function", Signature: `AsMap(): object`, Description: "Converts the current TreeMap value to a map/object.", Example: root + `.Get("profile").AsMap()`},
		{Name: "AsSlice", Type: "function", Signature: `AsSlice(): TreeMap[]`, Description: "Converts the current TreeMap value to a slice of TreeMap items.", Example: root + `.Get("items").AsSlice()`},
		{Name: "AsString", Type: "function", Signature: `AsString(): string`, Description: "Converts the current TreeMap value to string or returns an error.", Example: root + `.Get("profile.email").AsString()`},
		{Name: "AsInt", Type: "function", Signature: `AsInt(): int`, Description: "Converts the current TreeMap value to int or returns an error.", Example: root + `.Get("age").AsInt()`},
		{Name: "AsFloat", Type: "function", Signature: `AsFloat(): float`, Description: "Converts the current TreeMap value to float or returns an error.", Example: root + `.Get("score").AsFloat()`},
		{Name: "AsBool", Type: "function", Signature: `AsBool(): boolean`, Description: "Converts the current TreeMap value to boolean or returns an error.", Example: root + `.Get("enabled").AsBool()`},
		{Name: "AsAny", Type: "function", Signature: `AsAny(): any`, Description: "Returns the current TreeMap value as-is or returns an error.", Example: root + `.Get("payload").AsAny()`},
		{Name: "AsStruct", Type: "function", Signature: `AsStruct(target any): error`, Description: "Decodes the current TreeMap value into a Go struct target.", Example: root + `.Get("profile").AsStruct(target)`},
		{Name: "AsStringOr", Type: "function", Signature: `AsStringOr(default string): string`, Description: "Converts the current TreeMap value to string or returns the default.", Example: root + `.Get("profile.email").AsStringOr("")`},
		{Name: "AsIntOr", Type: "function", Signature: `AsIntOr(default int): int`, Description: "Converts the current TreeMap value to int or returns the default.", Example: root + `.Get("age").AsIntOr(0)`},
		{Name: "AsFloatOr", Type: "function", Signature: `AsFloatOr(default float): float`, Description: "Converts the current TreeMap value to float or returns the default.", Example: root + `.Get("score").AsFloatOr(0)`},
		{Name: "AsBoolOr", Type: "function", Signature: `AsBoolOr(default boolean): boolean`, Description: "Converts the current TreeMap value to boolean or returns the default.", Example: root + `.Get("enabled").AsBoolOr(false)`},
		{Name: "AsAnyOr", Type: "function", Signature: `AsAnyOr(default any): any`, Description: "Returns the current TreeMap value as-is or returns the default.", Example: root + `.Get("payload").AsAnyOr(null)`},
		{Name: "AsStrSlice", Type: "function", Signature: `AsStrSlice(): string[]`, Description: "Converts the current TreeMap value to a string slice.", Example: root + `.Get("roles").AsStrSlice()`},
		{Name: "AsIntSlice", Type: "function", Signature: `AsIntSlice(): int[]`, Description: "Converts the current TreeMap value to an int slice.", Example: root + `.Get("ids").AsIntSlice()`},
		{Name: "AsBoolSlice", Type: "function", Signature: `AsBoolSlice(): boolean[]`, Description: "Converts the current TreeMap value to a boolean slice.", Example: root + `.Get("flags").AsBoolSlice()`},
		{Name: "AsAnySlice", Type: "function", Signature: `AsAnySlice(): any[]`, Description: "Converts the current TreeMap value to a generic slice.", Example: root + `.Get("items").AsAnySlice()`},
	}
}

func cloneScriptBindingDescriptors(source []ScriptBindingDescriptor) []ScriptBindingDescriptor {
	cloned := make([]ScriptBindingDescriptor, len(source))
	for index, descriptor := range source {
		cloned[index] = descriptor
		if len(descriptor.Children) > 0 {
			cloned[index].Children = cloneScriptBindingDescriptors(descriptor.Children)
		}
	}
	return cloned
}

func mergeScriptBindingDescriptors(defaults []ScriptBindingDescriptor, custom []ScriptBindingDescriptor) []ScriptBindingDescriptor {
	merged := cloneScriptBindingDescriptors(defaults)
	positions := make(map[string]int, len(merged))
	for index, descriptor := range merged {
		positions[descriptor.Name] = index
	}
	for _, descriptor := range custom {
		if descriptor.Name == "" {
			continue
		}
		if index, found := positions[descriptor.Name]; found {
			merged[index] = descriptor
			continue
		}
		positions[descriptor.Name] = len(merged)
		merged = append(merged, descriptor)
	}
	return merged
}

func configuredBindings(cacheManager *jcache.Manager, scriptType string) (*configuredScriptBindings, bool, error) {
	scriptType = NormalizeScriptType(scriptType)
	if cacheManager == nil {
		return nil, false, nil
	}
	value, found := cacheManager.GetCacheInstance(ScriptBindingsCacheKey, scriptType)
	if !found && isDefaultExtensibleScriptType(scriptType) {
		value, found = cacheManager.GetCacheInstance(ScriptBindingsCacheKey, jcache.DefaultInstanceID)
	}
	if !found {
		return nil, false, nil
	}
	configured, ok := value.(*configuredScriptBindings)
	if !ok || configured.provider == nil {
		return nil, false, errors.New("script bindings cache has an invalid type")
	}
	return configured, true, nil
}

func isDefaultExtensibleScriptType(scriptType string) bool {
	switch NormalizeScriptType(scriptType) {
	case JourneyScript, ResourceScript, WorkflowScript, ScheduleScript:
		return true
	default:
		return false
	}
}

func getScriptBindings(provider any, context *ScriptBindingContext) (map[string]any, error) {
	if getter, ok := provider.(ScriptRuntimeBindingsProvider); ok {
		return getter.GetBindings(context)
	}
	if legacy, ok := provider.(ScriptBindingsProvider); ok {
		return legacy.Bindings(context)
	}
	return nil, errors.New("script bindings provider must implement GetBindings or Bindings")
}

func EnsureDefaultScriptRuntime(cacheManager *jcache.Manager) error {
	managerValue, hasManager := cacheManager.GetCacheInstance(ScriptManagerCacheKey, jcache.DefaultInstanceID)
	storageValue, hasStorage := cacheManager.GetCacheInstance(ScriptStorageCacheKey, jcache.DefaultInstanceID)
	if hasManager || hasStorage {
		if !hasManager || !hasStorage {
			return errors.New("script runtime cache is incomplete")
		}
		if _, ok := managerValue.(*jsrun.ScriptManager); !ok {
			return errors.New("script manager cache has an invalid type")
		}
		if _, ok := storageValue.(*jsrun.ScriptStorage); !ok {
			return errors.New("script storage cache has an invalid type")
		}
		return nil
	}
	folder, _ := scriptFolder.Load().(string)
	if folder == "" {
		folder = "js-scripts"
	}
	manager, storage := jsrun.NewDefaultStorage(folder)
	if err := storage.LoadFromDisk(); err != nil {
		return err
	}
	return ConfigureScriptRuntime(cacheManager, manager, storage)
}

func getScriptRuntime(cacheManager *jcache.Manager) (*jsrun.ScriptManager, *jsrun.ScriptStorage, error) {
	if cacheManager == nil {
		return nil, nil, errors.New("script cache manager is not configured")
	}
	managerValue, hasManager := cacheManager.GetCacheInstance(ScriptManagerCacheKey, jcache.DefaultInstanceID)
	storageValue, hasStorage := cacheManager.GetCacheInstance(ScriptStorageCacheKey, jcache.DefaultInstanceID)
	manager, managerOK := managerValue.(*jsrun.ScriptManager)
	storage, storageOK := storageValue.(*jsrun.ScriptStorage)
	if !hasManager || !hasStorage || !managerOK || !storageOK {
		return nil, nil, errors.New("script runtime is not configured")
	}
	return manager, storage, nil
}

func init() {
	scriptFolder.Store("js-scripts")
	goconf.OnLoad(func() {
		scriptFolder.Store(jenv.GetOptionalJourneyField("scripts.folder", "js-scripts"))
	})
}

func scriptStruct[T any](obj map[string]any) (T, error) {
	var value T
	data, err := json.Marshal(obj)
	if err != nil {
		return value, err
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return value, err
	}
	return value, nil
}

func addScriptValueInput(builder *inputs.ClientInputsBuilder, obj map[string]any) error {
	config, err := scriptStruct[inputs.ValueInputConfig](obj)
	if err != nil {
		return err
	}
	if config.ExternalID == "" || !isValueInputType(config.Type) {
		return errors.New("script value input requires external_id and a supported type")
	}
	if _, configured := obj["required"]; !configured {
		config.Required = true
	}
	if config.Pattern != "" && config.Type != inputs.STRING_INPUT {
		return errors.New("script value input pattern requires string type")
	}
	if config.UserName && config.Type != inputs.STRING_INPUT {
		return errors.New("script user_name input requires string type")
	}
	config.ID = ""
	config.StepType = types.ScriptStep
	return builder.AddValueInput(&config)
}

func addScriptMessageInput(builder *inputs.ClientInputsBuilder, stepID string, obj map[string]any) error {
	message, err := scriptStruct[inputs.Message](obj)
	if err != nil {
		return err
	}
	message.ID = stepID
	message.StepType = types.ScriptStep
	return builder.AddMessageInput(&message)
}

func scriptArgs(config goutils.TreeMapImpl) map[string]any {
	args, err := config.Get("args").AsMap()
	if err != nil {
		return map[string]any{}
	}
	data, err := json.Marshal(args)
	if err != nil {
		return map[string]any{}
	}
	var cloned map[string]any
	if json.Unmarshal(data, &cloned) != nil {
		return map[string]any{}
	}
	return cloned
}

func scriptOutcomeSetter(outcome *string, normalize bool) func(string) error {
	return func(value string) error {
		if strings.TrimSpace(value) == "" {
			return errors.New("script outcome cannot be empty")
		}
		if normalize {
			value = strings.ToLower(strings.TrimSpace(value))
		}
		*outcome = value
		return nil
	}
}

// SupportsDeclaredScriptOutcomes restricts script-owned outcomes to the three
// script families that can be selected by a journey Script step.
func SupportsDeclaredScriptOutcomes(scriptType string) bool {
	switch NormalizeScriptType(scriptType) {
	case JourneyScript, ResourceScript, WorkflowScript:
		return true
	default:
		return false
	}
}

// DeclaredScriptOutcomes returns the canonical outcomes stored in
// additional_props.outcomes. Invalid values are ignored here; REST validation
// rejects them before scripts are persisted.
func DeclaredScriptOutcomes(script *jsrun.Script) []string {
	if script == nil || !SupportsDeclaredScriptOutcomes(script.Type) || script.AdditionalProps == nil {
		return nil
	}
	raw, ok := script.AdditionalProps[ScriptOutcomesProp]
	if !ok {
		return nil
	}
	var values []string
	switch outcomes := raw.(type) {
	case []string:
		values = outcomes
	case []any:
		for _, outcome := range outcomes {
			value, ok := outcome.(string)
			if !ok {
				return nil
			}
			values = append(values, value)
		}
	default:
		return nil
	}
	return normalizeOutcomeNames(values)
}

func hasDeclaredScriptOutcomes(script *jsrun.Script) bool {
	if script == nil || !SupportsDeclaredScriptOutcomes(script.Type) || script.AdditionalProps == nil {
		return false
	}
	_, found := script.AdditionalProps[ScriptOutcomesProp]
	return found
}

// NormalizeDeclaredScriptOutcomes canonicalizes persisted script metadata.
// Unsupported script types cannot retain the reserved outcomes property.
func NormalizeDeclaredScriptOutcomes(script *jsrun.Script) error {
	if script == nil {
		return nil
	}
	if !SupportsDeclaredScriptOutcomes(script.Type) {
		if script.AdditionalProps != nil {
			delete(script.AdditionalProps, ScriptOutcomesProp)
			if len(script.AdditionalProps) == 0 {
				script.AdditionalProps = nil
			}
		}
		return nil
	}

	if script.AdditionalProps == nil {
		return nil
	}
	raw, found := script.AdditionalProps[ScriptOutcomesProp]
	if !found {
		return nil
	}
	var values []string
	switch outcomes := raw.(type) {
	case []string:
		values = outcomes
	case []any:
		values = make([]string, 0, len(outcomes))
		for _, outcome := range outcomes {
			value, ok := outcome.(string)
			if !ok {
				return errors.New("script outcomes must be an array of strings")
			}
			values = append(values, value)
		}
	default:
		return errors.New("script outcomes must be an array of strings")
	}
	normalized := normalizeOutcomeNames(values)
	if len(normalized) == 0 && len(values) > 0 {
		return errors.New("script outcomes cannot contain empty values")
	}
	script.AdditionalProps[ScriptOutcomesProp] = normalized

	return nil
}

func normalizeOutcomeNames(values []string) []string {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, found := seen[value]; found {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized
}

func scriptClientInputs(jtx *types.JourneyTransaction) map[string]any {
	builder := jtx.ClientInputsBuilder
	return map[string]any{
		"IsClientEmpty":   builder.IsClientEmpty,
		"IsNewEmpty":      builder.IsNewEmpty,
		"GetByExternalID": builder.GetFirstFromExternalID,
		"GetByType":       builder.GetByType,
		"AddValueInput": func(obj map[string]any) error {
			return addScriptValueInput(builder, obj)
		},
		"AddMessageInput": func(obj map[string]any) error {
			return addScriptMessageInput(builder, jtx.CurrentStepID, obj)
		},
	}
}

func scriptLogger(jtx *types.JourneyTransaction, script *jsrun.Script) map[string]any {
	emit := func(eventType types.EventType, message string, params ...any) {
		if jtx == nil {
			return
		}
		attrs := scriptLoggerAttrs(jtx, script)
		var eventErr error
		for _, param := range params {
			if param == nil {
				continue
			}
			if candidate, ok := asScriptLoggerAttrs(param); ok {
				for key, value := range candidate {
					attrs[key] = value
				}
				continue
			}
			if eventErr == nil {
				eventErr = scriptLoggerError(param)
			}
		}
		jtx.EmitEvent(&types.Event{
			Type:    eventType,
			Message: message,
			Error:   eventErr,
			Subject: types.EventSubject{
				Type: "script", ID: scriptID(script), Name: scriptName(script),
			},
			Attrs: attrs,
		})
	}
	return map[string]any{
		"Info":  func(message string, attrs ...any) { emit(types.EventFinished, message, attrs...) },
		"Event": func(message string, attrs ...any) { emit(types.EventFinished, message, attrs...) },
		"Error": func(message string, args ...any) { emit(types.EventFailed, message, args...) },
	}
}

func scriptLoggerAttrs(jtx *types.JourneyTransaction, script *jsrun.Script) map[string]any {
	attrs := map[string]any{
		"step": map[string]any{
			"id": jtx.CurrentStepID,
		},
		"script": map[string]any{
			"id": scriptID(script),
		},
	}
	if script != nil {
		scriptAttrs := attrs["script"].(map[string]any)
		scriptAttrs["name"] = script.Name
		scriptAttrs["type"] = script.Type
	}
	if jtx.Journey != nil {
		attrs["journey"] = map[string]any{
			"id":    jtx.Journey.ID,
			"name":  jtx.Journey.Name,
			"type":  jtx.Journey.JourneyType,
			"realm": jtx.Journey.Realm,
		}
	}
	if jtx.State != nil {
		if realm := jtx.State.GetRealm(); realm != "" {
			journeyAttrs, ok := attrs["journey"].(map[string]any)
			if !ok {
				journeyAttrs = map[string]any{}
				attrs["journey"] = journeyAttrs
			}
			journeyAttrs["realm"] = realm
		}
	}
	return attrs
}

func scriptID(script *jsrun.Script) string {
	if script == nil || script.Metadata == nil {
		return ""
	}
	return script.Metadata.ID
}

func scriptName(script *jsrun.Script) string {
	if script == nil {
		return ""
	}
	return script.Name
}

func asScriptLoggerAttrs(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	default:
		return nil, false
	}
}

func scriptLoggerError(value any) error {
	if err, ok := value.(error); ok {
		return err
	}
	return errors.New(fmt.Sprint(value))
}

func (binding *ScriptHTTPBinding) Send(uri string, options ...map[string]any) (map[string]any, error) {
	if strings.TrimSpace(uri) == "" {
		return nil, errors.New("http.Send requires url")
	}
	var opts map[string]any
	if len(options) > 0 {
		opts = options[0]
	}
	client, err := binding.client(scriptHTTPOptionString(opts, "instance", jcache.DefaultInstanceID))
	if err != nil {
		return nil, err
	}
	method := strings.ToUpper(scriptHTTPOptionString(opts, "method", http.MethodGet))
	headers := scriptHTTPHeaders(opts["headers"])
	body, err := scriptHTTPBody(opts["body"])
	if err != nil {
		return nil, err
	}
	if binding.transaction != nil {
		response, err := client.Request(method, uri, headers, body)
		if err != nil {
			return nil, err
		}
		return scriptHTTPResponse(response), nil
	}
	requestContext := binding.context
	if requestContext == nil {
		requestContext = context.Background()
	}
	response, err := client.RequestWithContext(requestContext, method, uri, headers, body)
	if err != nil {
		return nil, err
	}
	return scriptHTTPResponse(response), nil
}

func (binding *ScriptScheduleCacheBinding) Get(schedule string, options ...map[string]any) (any, error) {
	cache, err := scheduleCacheInstance(binding.cacheManager)
	if err != nil {
		return nil, err
	}
	var rawOptions map[string]any
	if len(options) > 0 {
		rawOptions = options[0]
	}
	return cache.Get(binding.executionContext(), binding.realm, schedule, types.ScheduleCacheOptions{
		MaxAgeSeconds: scriptScheduleCacheInt64(rawOptions, "maxAgeSeconds"),
		StaleIfError:  scriptScheduleCacheBool(rawOptions, "staleIfError"),
	})
}

func (binding *ScriptScheduleCacheBinding) Refresh(schedule string) (any, error) {
	cache, err := scheduleCacheInstance(binding.cacheManager)
	if err != nil {
		return nil, err
	}
	return cache.Refresh(binding.executionContext(), binding.realm, schedule)
}

func (binding *ScriptScheduleCacheBinding) Clear(schedule string) error {
	cache, err := scheduleCacheInstance(binding.cacheManager)
	if err != nil {
		return err
	}
	return cache.Clear(binding.executionContext(), binding.realm, schedule)
}

func (binding *ScriptScheduleCacheBinding) executionContext() context.Context {
	if binding.context != nil {
		return binding.context
	}
	return context.Background()
}

func scheduleCacheInstance(cacheManager *jcache.Manager) (types.ScheduleCache, error) {
	if cacheManager == nil {
		return nil, errors.New("schedule cache is not configured")
	}
	value, found := cacheManager.GetCacheInstance(ScheduleCacheKey, jcache.DefaultInstanceID)
	if !found {
		return nil, errors.New("schedule cache is not configured")
	}
	cache, valid := value.(types.ScheduleCache)
	if !valid || cache == nil {
		return nil, errors.New("schedule cache instance has an invalid type")
	}
	return cache, nil
}

func scriptScheduleCacheInt64(options map[string]any, key string) int64 {
	if options == nil {
		return 0
	}
	switch value := options[key].(type) {
	case int:
		return int64(value)
	case int64:
		return value
	case float64:
		return int64(value)
	default:
		return 0
	}
}

func scriptScheduleCacheBool(options map[string]any, key string) bool {
	value, _ := options[key].(bool)
	return value
}

func (binding *ScriptHTTPBinding) client(instanceID string) (HTTPClient, error) {
	var client HTTPClient = Client
	cacheManager := binding.cacheManager
	if cacheManager == nil && binding.transaction != nil {
		cacheManager = binding.transaction.CacheManager
	}
	if cacheManager != nil {
		if configured, ok := cacheManager.GetCacheInstance(HTTPClientCacheKey, instanceID); ok {
			configuredClient, valid := configured.(HTTPClient)
			if !valid {
				return nil, fmt.Errorf("http_client instance %q has an invalid type", instanceID)
			}
			client = configuredClient
		}
	}
	if client == nil {
		return nil, errors.New("HTTP client is not configured")
	}
	return client, nil
}

func scriptHTTPOptionString(options map[string]any, key string, fallback string) string {
	if options == nil {
		return fallback
	}
	if value, ok := options[key]; ok {
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return typed
			}
		default:
			asString := fmt.Sprint(typed)
			if strings.TrimSpace(asString) != "" {
				return asString
			}
		}
	}
	return fallback
}

func scriptHTTPHeaders(value any) map[string]string {
	headers := map[string]string{}
	if value == nil {
		return headers
	}
	switch typed := value.(type) {
	case map[string]string:
		for key, headerValue := range typed {
			headers[key] = headerValue
		}
	case map[string]any:
		for key, headerValue := range typed {
			if headerValue != nil {
				headers[key] = fmt.Sprint(headerValue)
			}
		}
	}
	return headers
}

func scriptHTTPBody(value any) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	switch typed := value.(type) {
	case string:
		return []byte(typed), nil
	case []byte:
		return typed, nil
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("marshal http.Send body: %w", err)
		}
		return data, nil
	}
}

func scriptHTTPResponse(response *goutils.Response) map[string]any {
	headers := map[string][]string{}
	for key, values := range response.Headers {
		headers[key] = values
	}
	return map[string]any{
		"request_uri":   response.RequestUri,
		"status":        response.Status,
		"headers":       headers,
		"body":          string(response.Body),
		"body_base64":   encoding.EncodeBase64(response.Body),
		"duration":      response.Duration.String(),
		"duration_nano": response.Duration.Nanoseconds(),
	}
}

func scriptBindings(jtx *types.JourneyTransaction, args map[string]any, outcome *string, scriptType string) map[string]any {
	scriptType = NormalizeScriptType(scriptType)
	if !isDefaultExtensibleScriptType(scriptType) {
		return map[string]any{
			"args":       args,
			"setOutcome": scriptOutcomeSetter(outcome, SupportsDeclaredScriptOutcomes(scriptType)),
		}
	}
	bindings := map[string]any{
		"args":          args,
		"http":          &ScriptHTTPBinding{transaction: jtx, cacheManager: jtx.CacheManager, context: jtx.Context},
		"scheduleCache": &ScriptScheduleCacheBinding{cacheManager: jtx.CacheManager, context: jtx.Context, realm: jtx.State.GetRealm()},
		"realm":         jtx.State.GetRealm(),
		"encoding": map[string]any{
			"EncodeBase64":    encoding.EncodeBase64,
			"DecodeBase64":    encoding.DecodeBase64,
			"EncodeBase64URL": encoding.EncodeBase64URL,
			"DecodeBase64URL": encoding.DecodeBase64URL,
			"EncodeHex":       encoding.EncodeHex,
			"DecodeHex":       encoding.DecodeHex,
		},
		"crypto": map[string]any{
			"NewUUID":      crypto.NewUUID,
			"GetRandBytes": crypto.GetRandBytes,
			"SHA1":         crypto.HashSHA1,
			"SHA256":       crypto.HashSHA256,
			"SHA384":       crypto.HashSHA384,
			"SHA512":       crypto.HashSHA512,
			"HmacSHA1":     crypto.HmacSHA1,
			"HmacSHA256":   crypto.HmacSHA256,
			"HmacSHA384":   crypto.HmacSHA384,
			"HmacSHA512":   crypto.HmacSHA512,
		},
		"setOutcome": scriptOutcomeSetter(outcome, SupportsDeclaredScriptOutcomes(scriptType)),
	}
	if scriptType != ScheduleScript {
		bindings["ctx"] = jtx.State.GetCtx()
		bindings["encCtx"] = jtx.State.GetEncryptedCtx()
		bindings["closedCtx"] = jtx.State.GetClosedCtx()
		bindings["tempCtx"] = jtx.State.GetTempCtx()
	}
	if scriptType == JourneyScript || scriptType == ResourceScript {
		request := jtx.Request
		if request == nil {
			request = types.NewEmptyRequest()
		}
		bindings["request"] = request.Snapshot()
		bindings["requestQuery"] = types.NewRequestQueryBinding(request)
		bindings["requestHeader"] = types.NewRequestHeaderBinding(request)
	}
	if scriptType == JourneyScript {
		bindings["clientInputs"] = scriptClientInputs(jtx)
	}
	return bindings
}

func ResolvedScheduleScriptBindings(ctx context.Context, cacheManager *jcache.Manager, observer types.Observer, realm string, script *jsrun.Script, args map[string]any, timeoutSeconds int, resultContexts ...*types.ScheduleResultContext) (map[string]any, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	scriptType := ScheduleScript
	resultContext := types.NewScheduleResultContext(nil)
	if len(resultContexts) > 0 && resultContexts[0] != nil {
		resultContext = resultContexts[0]
	}
	defaults := scheduleScriptBindings(ctx, cacheManager, observer, realm, script, args, timeoutSeconds, resultContext)
	configured, found, err := configuredBindings(cacheManager, scriptType)
	if err != nil {
		return nil, err
	}
	if !found {
		return defaults, nil
	}
	custom, err := getScriptBindings(configured.provider, &ScriptBindingContext{
		Args: args, ScriptType: scriptType, DefaultBindings: defaults,
	})
	if err != nil {
		return nil, err
	}
	if custom == nil {
		custom = map[string]any{}
	}
	if configured.mode == ScriptBindingsReplace {
		return custom, nil
	}
	for name, binding := range custom {
		defaults[name] = binding
	}
	return defaults, nil
}

func scheduleScriptBindings(ctx context.Context, cacheManager *jcache.Manager, observer types.Observer, realm string, script *jsrun.Script, args map[string]any, timeoutSeconds int, resultContext *types.ScheduleResultContext) map[string]any {
	return map[string]any{
		"args":            args,
		"script_id":       scriptID(script),
		"timeout_seconds": timeoutSeconds,
		"http":            &ScriptHTTPBinding{cacheManager: cacheManager, context: ctx},
		"scheduleCache":   &ScriptScheduleCacheBinding{cacheManager: cacheManager, context: ctx, realm: realm},
		"previousResult":  resultContext.PreviousResult,
		"SetResult":       resultContext.SetResult,
		"realm":           realm,
		"encoding": map[string]any{
			"EncodeBase64":    encoding.EncodeBase64,
			"DecodeBase64":    encoding.DecodeBase64,
			"EncodeBase64URL": encoding.EncodeBase64URL,
			"DecodeBase64URL": encoding.DecodeBase64URL,
			"EncodeHex":       encoding.EncodeHex,
			"DecodeHex":       encoding.DecodeHex,
		},
		"crypto": map[string]any{
			"NewUUID":      crypto.NewUUID,
			"GetRandBytes": crypto.GetRandBytes,
			"SHA1":         crypto.HashSHA1,
			"SHA256":       crypto.HashSHA256,
			"SHA384":       crypto.HashSHA384,
			"SHA512":       crypto.HashSHA512,
			"HmacSHA1":     crypto.HmacSHA1,
			"HmacSHA256":   crypto.HmacSHA256,
			"HmacSHA384":   crypto.HmacSHA384,
			"HmacSHA512":   crypto.HmacSHA512,
		},
		"logger": scheduleScriptLogger(ctx, observer, realm, script),
	}
}

func scheduleScriptLogger(ctx context.Context, observer types.Observer, scriptRealm string, script *jsrun.Script) map[string]any {
	emit := func(eventType types.EventType, message string, params ...any) {
		attrs := map[string]any{
			"script": map[string]any{
				"id": scriptID(script),
			},
			"schedule": map[string]any{
				"realm": scriptRealm,
			},
		}
		if script != nil {
			scriptAttrs := attrs["script"].(map[string]any)
			scriptAttrs["name"] = script.Name
			scriptAttrs["type"] = script.Type
		}
		var eventErr error
		for _, param := range params {
			if param == nil {
				continue
			}
			if candidate, ok := asScriptLoggerAttrs(param); ok {
				for key, value := range candidate {
					attrs[key] = value
				}
				continue
			}
			if eventErr == nil {
				eventErr = scriptLoggerError(param)
			}
		}
		types.EmitEvent(ctx, observer, &types.Event{
			Type:    eventType,
			Message: message,
			Error:   eventErr,
			Subject: types.EventSubject{
				Type: "script", ID: scriptID(script), Name: scriptName(script),
			},
			Attrs: attrs,
		})
	}
	return map[string]any{
		"Info":  func(message string, attrs ...any) { emit(types.EventFinished, message, attrs...) },
		"Event": func(message string, attrs ...any) { emit(types.EventFinished, message, attrs...) },
		"Error": func(message string, args ...any) { emit(types.EventFailed, message, args...) },
	}
}

func resolvedScriptBindings(jtx *types.JourneyTransaction, args map[string]any, outcome *string, scriptType string) (map[string]any, error) {
	defaults := scriptBindings(jtx, args, outcome, scriptType)
	configured, found, err := configuredBindings(jtx.CacheManager, scriptType)
	if err != nil {
		return nil, err
	}
	if !found {
		return defaults, nil
	}
	custom, err := getScriptBindings(configured.provider, &ScriptBindingContext{
		Transaction: jtx, Args: args, ScriptType: scriptType, DefaultBindings: defaults,
	})
	if err != nil {
		return nil, err
	}
	if custom == nil {
		custom = map[string]any{}
	}
	if configured.mode == ScriptBindingsReplace {
		return custom, nil
	}
	for name, binding := range custom {
		defaults[name] = binding
	}
	return defaults, nil
}

func (uns *Script) GetStepType() string {
	return types.ScriptStep
}

func (uns *Script) Execute(jtx *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	manager, storage, err := getScriptRuntime(jtx.CacheManager)
	if err != nil {
		return "", err
	}
	scriptID, err := config.Get("script_id").AsString()
	if err != nil {
		return "", errors.New("script_id is required")
	}
	script, err := storage.Load(scriptID)
	if err != nil || !IsRunnableScriptType(script.Type) {
		return "", jsrun.ErrScriptNotFound
	}
	code, err := script.GetRawCode()
	if err != nil {
		return "", err
	}
	program, err := manager.CompileScript(script.Name, code)
	if err != nil {
		return "", fmt.Errorf("compile script %s: %w", script.Name, err)
	}
	timeout := config.Get("timeout_seconds").AsIntOr(60)
	if timeout < 1 {
		return "", errors.New("timeout_seconds must be positive")
	}
	var outcome string
	args := scriptArgs(config)
	bindings, err := resolvedScriptBindings(jtx, args, &outcome, script.Type)
	if err != nil {
		return "", err
	}
	bindings["logger"] = scriptLogger(jtx, script)
	_, err = manager.ExecuteWithBindings(
		program,
		bindings,
		time.Duration(timeout)*time.Second,
	)
	if err != nil {
		return "", err
	}
	if outcome == "" && jtx.ClientInputsBuilder.IsNewEmpty() {
		return "", errors.New("script returned neither an outcome nor client inputs")
	}
	if outcome != "" {
		declared := DeclaredScriptOutcomes(script)
		if hasDeclaredScriptOutcomes(script) {
			found := false
			for _, allowed := range declared {
				if strings.EqualFold(allowed, outcome) {
					found = true
					outcome = allowed
					break
				}
			}
			if !found {
				return "", fmt.Errorf("script outcome %q is not declared", outcome)
			}
		}
	}
	return outcome, nil
}

func init() {
	defaultStepRegistry.AddStep(&Script{}, map[string]map[string]any{
		".":       {"x-category": types.ContextCategory, "x-order": []string{"script_id", "timeout_seconds", "args", "outcome"}},
		"outcome": {"x-dynamic-outcome": true},
		"script_id": {
			"x-type": "selectable",
			"x-props": map[string]any{
				"resource":      "scripts",
				"query":         map[string]any{"type": JourneyScript},
				"nameProperty":  "name",
				"valueProperty": "id",
			},
		},
		"args": {
			"x-type": "script-args",
			"x-props": map[string]any{
				"sourceProperty": "script_id",
			},
		},
	})
}
