# Extension points and production advice

Go Journey is meant to be embedded in an application. The host owns storage durability, HTTP policy, authentication, authorization, observability, and custom business integrations.

Primary extension points:

| Need | Extension point |
|---|---|
| Custom business behavior | `types.IStep` registered in `types.Steps` |
| Journey definitions outside files | `JourneyConfigurations` |
| Durable/resumable state | `JourneyStates` |
| Token format/key system | `JourneyTokens` |
| HTTP transport policy | `steps.HTTPClient` cache instance |
| Shared singleton services | `cache.Manager` |
| Client input validation | `inputs.RegisterValidator` |
| Terminal side effects | `manager.OnJourneySuccess` / `manager.OnJourneyFailure` |
| REST authentication/authorization | Gin middleware and `RESTAPIConfig.PrepareExecution` |

## Custom steps

A step implements `types.IStep`:

```go
type AccountEnabled struct {
    steps.BasicStep
    Outcome struct {
        True  string `json:"true"`
        False string `json:"false"`
    } `json:"outcome"`
}

func (*AccountEnabled) GetStepType() string { return "AccountEnabled" }

func (*AccountEnabled) Execute(tx *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
    if tx.State.GetClosedCtx().Get("account.enabled").AsBoolOr(false) {
        return "true", nil
    }
    return "false", nil
}
```

Register it before invoking journeys:

```go
registry := steps.GetDefaultStepRegistry()
registry.AddStep(&AccountEnabled{}, map[string]map[string]any{
    ".": {"x-category": types.FlowCategory},
})
```

If you provide `JourneyManagerConfig.Steps`, the manager merges every missing built-in implementation and its existing schema into it. A custom implementation registered with the same step type intentionally overrides the default. The registry is concurrency-safe, but complete custom registration before serving requests so schemas and behavior remain stable.

Custom steps can access:

- `tx.Context` for cancellation.
- `tx.Payload` and its realm.
- `tx.CacheManager` for reusable singleton dependencies.
- `tx.Journey.GetProp` for journey-level metadata.
- `tx.State` for contexts.
- `tx.ClientInputsBuilder` for interactive steps.
- `tx.Steps` for registry-aware composition.

## Cache-managed dependencies

Register reusable dependencies once rather than passing them on every invocation. Persistent instances store constructor configuration and rebuild the live pointer after deployment:

```go
cacheManager, err := cache.NewManager(&cache.ManagerConfig{
    FolderPath: "cache-instances",
    Caches: map[string]cache.CacheConfig{
        steps.HTTPClientCacheKey: {
            Factory: steps.HTTPClientFactory,
            MaxInstances: 100,
            MaxSizeBytes: 16 << 20,
        },
        "accounts": {
            Factory: accountServiceFactory,
            MaxInstances: 20,
        },
    },
})

err = cacheManager.UpdateCacheInstance(
    steps.HTTPClientCacheKey,
    cache.DefaultInstanceID,
    goutils.ClientConfig{Timeout: time.Minute},
)

manager := gojourney.NewManager(&gojourney.JourneyManagerConfig{
    JourneyStorage: journeyStorage,
    CacheManager: cacheManager,
})
```

`UpdateCacheInstance` serializes configuration through `nstore`, calls the registered factory, requires a non-nil pointer, and atomically replaces the live singleton. On restart, saved configurations are loaded and passed to their factories. `UpdateRuntimeCacheInstance` stores a pointer without persistence for resources that cannot or should not be reconstructed.

Each `Caches` entry configures one independent category:

- `Factory` reconstructs persisted instances and is required only when `UpdateCacheInstance` or persisted records use that category.
- `MaxInstances` limits the number of IDs inside that category.
- `MaxSizeBytes` limits tracked bytes inside that category. Persisted instances use the encoded constructor-configuration size; runtime instances use the estimate passed to `UpdateRuntimeCacheInstance`.
- Zero limits mean unlimited. Negative limits reject manager construction.

Thus, an `http_client` limit of 100 does not count `accounts`, `jwk_cache`, or any other category. Categories omitted from `Caches` can still hold runtime instances and are unlimited, but they cannot persist instances because they have no factory.

### Remote JWKS cache for VerifyJWT

`VerifyJWT` with `jwk_uri` requires JWKS caching. By default, no custom `CacheManager` instance is needed: configure `go-jwte-manager` external JWKS storage and the step uses it as the default persistent cache.

```json
{
  "jwtek": {
    "external_jwks": {
      "folder": "jwtek/jwks",
      "cache_seconds": 60
    }
  }
}
```

Register `steps.JWKCacheKey` in `CacheManager` only when replacing that default backend, for example with Redis:

```go
type JWKCache interface {
    Get(uri string) ([]byte, bool)
    Set(uri string, value []byte) error
}

_ = cacheManager.UpdateRuntimeCacheInstance(
    steps.JWKCacheKey,
    cache.DefaultInstanceID,
    myJWKCache,
    0,
)
```

The custom instance must be concurrency-safe. If neither the default `jwtek.external_jwks` storage nor a custom `steps.JWKCacheKey` instance is available, `jwk_uri` verification is rejected instead of fetching uncached.

The journey manager also installs one runtime singleton under `cache.RegexpCacheKey` and `cache.DefaultInstanceID`. This singleton owns the bounded LRU of compiled input-validation patterns; `journey.cache.regexp.max` controls its entry capacity and defaults to 1000. It participates in the same manager lookup and lifecycle as other runtime dependencies, while LRU entries remain process-local and are not persisted. A host may install its own `*cache.RegexpCache` before constructing the journey manager:

```go
cacheManager, _ := cache.NewManager(&cache.ManagerConfig{
    Caches: map[string]cache.CacheConfig{
        cache.RegexpCacheKey: {MaxInstances: 1},
    },
})
_ = cacheManager.UpdateRuntimeCacheInstance(
    cache.RegexpCacheKey,
    cache.DefaultInstanceID,
    cache.NewRegexpCache(2000),
    0,
)
```

Reads use a shared lock, while update and removal operations are exclusive. Returned singleton instances must support their own concurrent operations. `GetCache` returns a snapshot of one category; `GetCacheInstance`, `RemoveCache`, and `RemoveCacheInstance` provide granular access. `Stats` reports manager totals and `CacheStats` reports one category.

## Custom state storage

Implement `gojourney.JourneyStates` for Redis, a database, or another shared store:

```go
type JourneyStates interface {
    Store(*types.JourneyState, time.Duration) bool
    Get(string) (*types.JourneyState, bool)
    GetAndDelete(string) (*types.JourneyState, bool)
    Delete(string)
}
```

`GetAndDelete` must be atomic. Token and resume-ID replay protection depends on exactly one consumer receiving the state.

## Custom journey-definition storage

Filesystem loading is only the default. Implement `gojourney.JourneyConfigurations` to load definitions from a database, API, embedded files, or another source:

```go
type JourneyConfigurations interface {
    Load(id string) (*types.JourneyConfiguration, error)
}
```

Set `JourneyManagerConfig.JourneyStorage`; `FolderPath` then becomes optional. Implementations must support concurrent reads and should return immutable configurations.

## Custom token service

Implement `gojourney.JourneyTokens` when a deployment needs a different authenticated token format or key system:

```go
type JourneyTokens interface {
    Validate(string) (*types.JourneyState, error)
    Sign(*types.JourneyState) ([]byte, error)
}
```

`Validate` must cryptographically authenticate every returned claim. The engine still enforces the single-use JTI through JourneyStates.

## Custom client-input validators

Custom interactive input types can register a process-wide concurrency-safe validator:

```go
inputs.RegisterValidator("email", func(config goutils.TreeMapImpl, input *inputs.ClientInput) *inputs.ClientError {
    // Return nil when valid, otherwise a client-safe validation error.
    return nil
})
```

Register validators during application startup before requests are served.

## What is intentionally internal

Not every helper is an application extension point:

- The regex cache is an internal optimization; custom input behavior belongs in a registered validator.
- Cryptographically secure ID generation is a protocol safety requirement rather than a replaceable random source.
- AES-GCM is the encrypted-context wire format. Replacing it would require versioned token migration, not a runtime callback.
- Placeholder replacement is part of step configuration semantics; custom value behavior should live in a custom step.

All boundaries that connect the engine to application infrastructure are injectable: journey definitions, resumable state, token authentication, steps, input validation, HTTP transport, request context, callbacks, and cache-managed services.

For multiple replicas:

- Use shared state storage.
- Use the same encryption key on every replica.
- Use shared JWT key storage configured by `go-jwte-manager`.
- Keep clocks synchronized and define cache eviction monitoring.

## Journey terminal listeners

```go
successListenerID := manager.OnJourneySuccess(func(event *types.JourneyExecutionEvent) {
    // create session, audit, metrics, etc.
    _ = event.State
})

failureListenerID := manager.OnJourneyFailure(func(event *types.JourneyExecutionEvent) {
    // audit or metrics for failed terminal journeys.
    _ = event.Error
})

manager.RemoveJourneyListener(successListenerID)
manager.RemoveJourneyListener(failureListenerID)
```

Terminal listeners belong to the manager, not to a single serialized journey state. They run only when a journey finishes successfully or fails; suspended journeys do not emit these callbacks. Listener execution is asynchronous in goroutines, and a listener panic cannot crash or block the engine, so listeners should do their own error reporting and be concurrency-safe.

## Security checklist

- Never put secrets in `ctx`; it is signed but readable.
- Use `encCtx` for confidential token-carried values and `closedCtx` for server-only values.
- Load `EncryptKey` from a secret manager; never generate a different key per replica.
- Treat journey tokens and resume IDs as credentials.
- Use HTTPS when transferring tokens, inputs, or callback URIs.
- Do not log full tokens in production. The verbose fixture runner is intentionally diagnostic.
- Restrict who can publish journey configurations.
- Prevent SSRF by allowlisting HTTP destinations when URI placeholders can contain user-influenced values.
- Apply request timeouts and response-size limits in a custom HTTP client.
- Monitor `OnAsyncError`, state-store failures, invalid tokens, and repeated client validation failures.
- Do not rename active journey or step IDs until outstanding states expire.

## Environment initialization

Call `goconf.LoadConfig` before `env.SetEnvironment`. Supported optional fields under `journey` are:

```json
{
  "journey": {
    "client_inputs_key": "client_inputs",
    "context_prefix_key": "journey_"
  }
}
```

The context prefix determines internal keys such as expiration extension, suspension metadata, and username storage.
