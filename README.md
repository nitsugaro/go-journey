# Go Journey

Go Journey is a configuration-driven workflow engine for Go. You define journeys as JSON graphs, then invoke them from Go code or through the optional REST API. A journey can collect client input, branch, call HTTP services, verify JWTs, run scripts, execute child journeys, suspend for callbacks, and resume safely with signed tokens.

Use it when you want authentication, onboarding, verification, or orchestration flows to be configurable without hardcoding every path in application code.

## Developer path

1. Install the dependency.
2. Add `.config.json`.
3. Initialize `go-conf`, environment keys, storage, cache, and manager.
4. Save or load journey definitions.
5. Invoke a journey.
6. Resume with the returned journey token when client input is required.
7. Add REST, custom steps, scripts, state storage, and cache-managed dependencies only when needed.

## Install

```bash
go get github.com/nitsugaro/go-journey
```

The module currently targets Go 1.24 or newer.

## Initial performance setup

Copy this shape for a normal production service:

```go
import (
    "time"

    goconf "github.com/nitsugaro/go-conf"
    gojourney "github.com/nitsugaro/go-journey"
    jcache "github.com/nitsugaro/go-journey/cache"
    "github.com/nitsugaro/go-journey/env"
    "github.com/nitsugaro/go-journey/steps"
    goutils "github.com/nitsugaro/go-utils/v2"
)

if err := goconf.LoadConfig(); err != nil {
    panic(err)
}
env.SetEnvironment()

registry := steps.GetDefaultStepRegistry()

storage, err := gojourney.NewJourneyStorage("config/auth/journeys", registry)
if err != nil {
    panic(err)
}

cacheManager, err := jcache.NewManager(&jcache.ManagerConfig{
    FolderPath: "config/auth/cache",
    Caches: map[string]jcache.CacheConfig{
        // Used by HttpRequest and VerifyJWT jwk_uri fetches.
        // This is the only built-in cache category you normally configure manually.
        steps.HTTPClientCacheKey: {
            Factory:      steps.HTTPClientFactory,
            MaxInstances: 10,
            MaxSizeBytes: 1 << 20,
        },

        // Optional: only needed if you install your own regexp cache before NewManager.
        jcache.RegexpCacheKey: {
            MaxInstances: 1,
        },

        // Optional: only needed if you replace the default script runtime/storage.
        steps.ScriptManagerCacheKey: {
            MaxInstances: 1,
        },
        steps.ScriptStorageCacheKey: {
            MaxInstances: 1,
        },
        steps.ScriptBindingsCacheKey: {
            MaxInstances: 1,
        },

        // Optional: only needed if replacing jwtek.external_jwks with Redis/custom storage.
        steps.JWKCacheKey: {
            MaxInstances: 1,
        },

        // Optional: LDAP repositories for LDAPSearch/LDAPBind/LDAPCompare/write steps.
        steps.LDAPClientCacheKey: {
            Factory:      steps.LDAPClientFactory,
            MaxInstances: 10,
        },
    },
})
if err != nil {
    panic(err)
}

if err := cacheManager.UpdateCacheInstance(
    steps.HTTPClientCacheKey,
    jcache.DefaultInstanceID,
    goutils.ClientConfig{
        Timeout: 30 * time.Second,
    },
); err != nil {
    panic(err)
}

if err := cacheManager.UpdateCacheInstance(
    steps.LDAPClientCacheKey,
    "corporate_ad",
    steps.LDAPClientConfig{
        URLs:   []string{"ldaps://ldap.example.com:636"},
        BaseDN: "dc=example,dc=com",
        Bind: steps.LDAPBindConfig{
            Method:   "simple",
            DN:       "cn=journey,ou=services,dc=example,dc=com",
            Password: "...",
        },
        Pool: steps.LDAPPoolConfig{
            MaxOpen: 10,
            MaxIdle: 5,
        },
    },
); err != nil {
    panic(err)
}

manager := gojourney.NewManager(&gojourney.JourneyManagerConfig{
    JourneyStorage: storage,
    CacheManager:   cacheManager,
    EncryptKey:     []byte("0123456789abcdef0123456789abcdef"),
    Observer:       gojourney.NewJSONEventObserver(os.Stdout), // optional
})
```

What you add first:

| Cache/config                   | Required setup                                                                                                                                        |
| ------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| `steps.HTTPClientCacheKey`     | Add to `CacheManager.Caches`; call `UpdateCacheInstance` for the default HTTP client.                                                                 |
| `jwtek.external_jwks`          | Add to `.config.json`; this is the default persistent JWKS cache for `VerifyJWT` `jwk_uri`.                                                           |
| `jcache.RegexpCacheKey`        | Usually no instance setup; `NewManager` installs it. Configure size with `journey.cache.regexp.max`.                                                  |
| `steps.ScriptManagerCacheKey`  | Usually no instance setup; `NewManager` installs default from `journey.scripts.folder`.                                                               |
| `steps.ScriptStorageCacheKey`  | Usually no instance setup; `NewManager` installs default from `journey.scripts.folder`.                                                               |
| `steps.ScriptBindingsCacheKey` | Only call `steps.ConfigureScriptTypeBindings` for per-script-type bindings, or `steps.ConfigureScriptBindings` for a legacy/global binding extension. |
| `steps.JWKCacheKey`            | Only call `UpdateRuntimeCacheInstance` when replacing `jwtek.external_jwks`.                                                                          |
| `steps.LDAPClientCacheKey`     | Add to `CacheManager.Caches`; call `UpdateCacheInstance` once per LDAP repository ID. Steps reference that ID with `connection`.                      |
| custom app cache key           | Add your own `CacheManager.Caches` entry only when you need limits/persistence; then use `UpdateCacheInstance` or `UpdateRuntimeCacheInstance`.       |

## `.config.json`

This is a practical baseline. Keep it even if you only use direct Go invocation; the same config supports schemas, scripts, regexp cache, REST routes, and remote JWKS cache.

```json
{
  "jwtek": {
    "external_jwks": {
      "folder": "config/auth/jwks",
      "cache_seconds": 300
    }
  },
  "journey": {
    "base_url": "https://journey.example.com",
    "client_inputs_key": "client_inputs",
    "context_prefix_key": "journey_",
    "scripts": {
      "folder": "config/auth/scripts"
    },
    "cache": {
      "regexp": {
        "max": 1000
      }
    },
    "rest_api": {
      "base_path": "/journey",
      "return_vars": false,
      "ui": {
        "enabled": false,
        "path": "/journey-ui"
      },
      "routes": {
        "journey_routes": ["/:realm"],
        "journey_item_routes": ["/:realm/:journeyId"],
        "script_routes": ["/:realm/scripts"],
        "script_item_routes": ["/:realm/scripts/:scriptId"],
        "script_binding_routes": ["/:realm/scripts/:scriptId/bindings"],
        "script_type_binding_routes": ["/:realm/script-bindings"],
        "schedule_routes": ["/:realm/schedules"],
        "schedule_item_routes": ["/:realm/schedules/:scheduleId"],
        "schedule_trigger_routes": ["/:realm/schedules/:scheduleId/trigger"],
        "step_schema_routes": ["/step-schemas"],
        "step_schema_item_routes": ["/step-schemas/:stepType"],
        "invoke_routes": ["/:realm/invoke"]
      }
    }
  }
}
```

Important:

- `journey.*` config belongs to this library.
- `jwtek.external_jwks.*` belongs to `github.com/nitsugaro/go-jwte-manager`; Go Journey uses it as the default persistent cache for `VerifyJWT` steps that use `jwk_uri`.
- You do not need to register a custom JWK cache for normal production use. Configure `jwtek.external_jwks` by default. Register `steps.JWKCacheKey` only when replacing that backend with Redis, another shared cache, or a custom implementation.

| Key                                 |                  Default | Purpose                                                                                                       |
| ----------------------------------- | -----------------------: | ------------------------------------------------------------------------------------------------------------- |
| `jwtek.external_jwks.folder`        |             `jwtek/jwks` | Persistent folder for remote JWKS fetched by `VerifyJWT` `jwk_uri` mode.                                      |
| `jwtek.external_jwks.cache_seconds` |                     `60` | TTL for remote JWKS entries. Use a value compatible with the issuer key-rotation policy.                      |
| `journey.base_url`                  | `https://localhost:3000` | Base URL used in generated step-schema resource IDs.                                                          |
| `journey.client_inputs_key`         |          `client_inputs` | Internal context key used to store pending client-input definitions.                                          |
| `journey.context_prefix_key`        |               `journey_` | Prefix for internal context keys such as terminal data, suspension metadata, and expiration extension flags.  |
| `journey.scripts.folder`            |             `js-scripts` | Default filesystem folder for script definitions.                                                             |
| `journey.cache.regexp.max`          |                   `1000` | Maximum compiled-regexp entries used by input validation.                                                     |
| `journey.rest_api.base_path`        |               `/journey` | Base path for the optional Gin REST API.                                                                      |
| `journey.rest_api.return_vars`      |                  `false` | Whether REST journey CRUD responses include generated `vars`. Persisted configs still keep `vars` internally. |
| `journey.rest_api.ui.enabled`       |                  `false` | Serves the embedded production UI build through Gin. Node/React source is not needed at runtime.              |
| `journey.rest_api.ui.path`          |            `/journey-ui` | Gin path where the embedded UI is mounted. Keep it outside wildcard API routes such as `/:realm`.             |
| `journey.rest_api.routes.*`         |           see JSON above | Route templates for optional REST APIs. Item routes must include their required ID param.                     |

## Scheduler

Schedules are persisted with NStore in `<folder_path>/schedules` unless you pass `ScheduleStorage` to `JourneyManagerConfig` or `RESTAPIConfig`.

Supported targets:

- `script`: runs a stored script with optional `args`. The script must use type `schedule`.
- `workflow`: runs a `workflow` journey using `initial_data`. Workflow journeys have no client interaction and should finish with the `End` step.

Supported schedule kinds:

- `cron`: standard cron expression, optional `timezone`.
- `interval`: repeats every `interval_seconds`.

There is no separate `once` kind. Use `max_runs: 1` for one-time schedules. When `timeout_seconds` is empty or `0`, the scheduler does not add its own timeout and waits until the target finishes.

Schedule scripts get backend-safe bindings: `args`, `http`, `scheduleCache`, `previousResult`, `SetResult`, `realm`, `encoding`, `crypto` and `logger`. They do not get `ctx`, `encCtx`, `closedCtx`, `tempCtx`, request or client-input bindings because they run outside a journey execution. A schedule script publishes a shared result explicitly with `SetResult(value)`; its final JavaScript expression is not cached. `previousResult` is `null` on the first run and contains the last successfully published value afterwards.

Workflow scripts run inside a workflow journey, so they also receive the normal journey contexts plus `args`, `http`, `realm`, `encoding`, `crypto`, `logger` and `setOutcome`.

Every successful schedule execution that publishes a value updates an internal, process-local cache. Interval, cron and manual trigger executions all use the same cache. Workflow targets publish through the optional `result` property of their terminal `End` step and receive the previous value at `ctx.initial_data.previousResult`. An `End` without `result`, or a schedule script that does not call `SetResult`, completes normally without creating or replacing a cache entry.

```js
const token = http.Send("https://auth.example.test/token", { method: "POST" })
SetResult({ token: JSON.parse(token.body), previous: previousResult })
```

Journey scripts can consume those values through the shared binding:

```js
const token = scheduleCache.Get("external-token", { maxAgeSeconds: 3300, staleIfError: true })
const refreshed = scheduleCache.Refresh("external-token")
scheduleCache.Clear("external-token")
```

The equivalent journey steps are `ScheduleCacheGet`, `ScheduleCacheRefresh` and `ScheduleCacheClear`. They resolve the schedule name inside the current realm. `Get` executes the producer on a miss or expired value, `Refresh` always executes it, and `Clear` only removes the current value. Concurrent refreshes for the same schedule share one execution.

```json
{
  "name": "nightly-sync",
  "realm": "alpha",
  "active": true,
  "kind": "cron",
  "cron": "0 3 * * *",
  "timezone": "America/Argentina/Buenos_Aires",
  "max_runs": 0,
  "trigger_enabled": true,
  "trigger_wait": true,
  "target": {
    "type": "workflow",
    "journey_id": "00000000-0000-0000-0000-000000000016",
    "initial_data": {
      "source": "scheduler"
    }
  }
}
```

REST routes, when schedule storage is configured:

```text
GET    /journey/:realm/schedules
PUT    /journey/:realm/schedules
GET    /journey/:realm/schedules/:scheduleId
DELETE /journey/:realm/schedules/:scheduleId
POST   /journey/:realm/schedules/:scheduleId/trigger
```

## Events and JSON logs

Go Journey is silent unless you pass an observer. Use the built-in JSON observer for REST examples or provide your own adapter to `slog`, Zap, Zerolog, OpenTelemetry, etc.

```go
manager := gojourney.NewManager(&gojourney.JourneyManagerConfig{
    JourneyStorage: storage,
    CacheManager:   cacheManager,
    EncryptKey:     encryptKey,
    Observer:       gojourney.NewJSONEventObserver(os.Stdout),
})
```

Custom observer:

```go
type appObserver struct{}

func (appObserver) OnEvent(ctx context.Context, event *types.Event) {
    // event.Type: started, finished, failed, suspended
    // event.Subject: rest, journey, step, script
    // event.Message, event.Error, event.Duration, event.Attrs carry details.
}
```

The REST API emits access events with method, URI, headers, query parameters, status and duration. The executor emits journey and step start/finish/failure/suspension events. Scripts use `logger`, not `console`:

```js
logger.Info('loaded profile', { user_id: args.user_id });
logger.Error('remote check failed', err, { provider: 'risk' });
```

JSON logs keep repeated metadata grouped:

```json
{
  "type": "finished",
  "message": "step execution finished",
  "subject": { "type": "step", "id": "check", "name": "Check request" },
  "attrs": {
    "journey": { "id": "journey-id", "name": "login", "realm": "alpha" },
    "step": { "id": "check", "name": "Check request", "type": "IfExpression" },
    "outcome": "true"
  }
}
```

## Create and save a journey

Journey files can be created manually, saved through `JourneyStorage`, or saved through the optional REST API. Saving through code or REST is preferred because it validates the graph and generates `vars` for placeholders.

```go
journey := &types.JourneyConfiguration{
    Name:        "hello",
    Realm:       "alpha",
    Active:      true,
    DefaultExp:  10,
    StartStepID: "ask_name",
    Steps: map[string]*types.Step{
        "ask_name": {
            Name:     "Ask name",
            StepType: types.FormStep,
            Config: map[string]any{
                "context": "ctx",
                "object":  "profile",
                "inputs": []any{
                    map[string]any{
                        "id":          "name",
                        "external_id": "profile.name",
                        "label":       "Name",
                        "type":        "string",
                        "required":    true,
                    },
                },
                "outcome": map[string]any{"true": "success"},
            },
        },
        "success": {
            Name:     "Done",
            StepType: types.SuccessStep,
            Config: map[string]any{
                "data": map[string]any{
                    "message": "Welcome ${ctx.profile.name}",
                },
            },
        },
    },
}

if err := storage.Save(journey); err != nil {
    panic(err)
}
```

`storage.Save` manages ID/revision metadata, validates step configs, validates outcomes, rejects broken graph links, and generates `config.vars` for placeholders.

## Invoke and resume

Start by journey ID:

```go
response, state, err := manager.InvokeJourney(&types.JourneyExecute{
    Payload: (&types.JourneyPayloadReq{JourneyID: "hello"}).
        SetRealm(&types.Realm{Name: "alpha"}),
})
```

If the journey needs client input, `response` contains:

- `journey_token`: signed, single-use resume token.
- `client_inputs`: safe field definitions to render on the client.

Resume with the same token and submitted inputs:

```go
response.ClientInputs[0].Input = "Ada"

response, state, err = manager.InvokeJourney(&types.JourneyExecute{
    Payload: &types.JourneyPayloadReq{
        Jwt:          response.Jwt,
        ClientInputs: response.ClientInputs,
    },
})
```

Terminal behavior:

- Success returns `response == nil` and a final `state`.
- Failure returns `ErrJourneyFailure`; terminal Failure data is exposed through the configured terminal response behavior.
- Validation errors return client-safe input errors and restore the state so the user can retry.

## Optional REST API

Use the Gin REST API when you want out-of-the-box HTTP CRUD and invocation for development, admin panels, or internal tooling.

```go
router := gin.New()

manager := gojourney.NewManager(&gojourney.JourneyManagerConfig{
    JourneyStorage: storage,
    CacheManager:   cacheManager,
    EncryptKey:     []byte("0123456789abcdef0123456789abcdef"),
    RESTAPI: &gojourney.RESTAPIConfig{
        Enabled:        true,
        Router:         router,
        BasePath:       "/journey",
        JourneyStorage: storage,
    },
})
```

Default route groups under `journey.rest_api.base_path`:

| Method   | Route                       | Purpose                                                               |
| -------- | --------------------------- | --------------------------------------------------------------------- |
| `GET`    | `/:realm`                   | List journeys. Supports filters.                                      |
| `PUT`    | `/:realm`                   | Create or update a journey. Returns `201` on create, `200` on update. |
| `GET`    | `/:realm/:journeyId`        | Read one journey.                                                     |
| `DELETE` | `/:realm/:journeyId`        | Delete one journey.                                                   |
| `GET`    | `/:realm/scripts`           | List scripts.                                                         |
| `PUT`    | `/:realm/scripts`           | Create or update a script.                                            |
| `GET`    | `/:realm/scripts/:scriptId` | Read one script.                                                      |
| `DELETE` | `/:realm/scripts/:scriptId` | Delete one script.                                                    |
| `GET`    | `/step-schemas`             | List step schemas.                                                    |
| `GET`    | `/step-schemas/:stepType`   | Read one step schema.                                                 |
| `POST`   | `/:realm/invoke`            | Invoke or resume a journey.                                           |
| `ANY`    | `/<journey-name>/*path`     | Fallback invocation using the first path segment as the resource journey name. |

The REST API has no authentication middleware by default. Add global, admin, and invocation middleware in the host application according to your security model.

Resource invocation is the REST API fallback: registered routes are always evaluated first. After removing the configured base path, only the first path segment is used as the resource journey name; all remaining segments become `request.route_path`. Matching does not filter by realm. A journey named `recurso` handles `/journey/recurso/users/42`; the execution request receives `request.route_prefix == "/recurso"` and `request.route_path == "/users/42"`. If the same name exists in multiple realms, the first eligible journey returned by storage is used. Name resolution is cached after the first lookup and cleared when journeys are saved or deleted. If no resource journey matches, the request returns an empty `404 Not Found` response.

The previous `/route/:realm/*routePath` endpoint and `route_invoke_routes` setting are no longer used.

## Core concepts

### Journey

A journey is a graph:

- `start_step_id` chooses the first step.
- each step has `step_type`, `config`, and `outcome`.
- outcomes map runtime decisions to next step IDs.
- terminal steps complete the journey.

### Contexts

| Context   | Placeholder         | Visibility                                  |
| --------- | ------------------- | ------------------------------------------- |
| Public    | `${ctx.path}`       | Signed into token; visible to token holder. |
| Encrypted | `${encCtx.path}`    | Encrypted into token.                       |
| Closed    | `${closedCtx.path}` | Server-side only.                           |
| Temporary | `${tempCtx.path}`   | Current execution only.                     |

Never store secrets in `ctx`.

### Placeholders and `vars`

Developers write placeholders in ordinary config values:

```json
{
  "body": "{\"user\":\"${closedCtx.user_id}\"}"
}
```

`JourneyStorage.Save` generates `config.vars` automatically. Do not manually create `vars` unless you are building low-level tooling. Runtime resolves placeholders once immediately before each step executes, preserving typed values for exact placeholders and using strings for mixed text.

### Client inputs

Use `Form` for generic typed input. `id` is the private context attribute; `external_id` is the client-facing stable ID. The client should not depend on internal context keys.

### Cache-managed dependencies

`CacheManager` owns shared singleton dependencies such as HTTP clients, custom JWK caches, regexp cache, and application services. Limits are configured per cache category. Use it for reusable dependencies, not per-request data.

## Built-in steps

Common step groups:

- Terminal: `Success`, `Failure`.
- Client: `Form`, `Choice`, `Metadata`.
- Branching: `Condition`, `IfExpression`, `SwitchExpression`, `Assert`, `NotEmpty`, `Random`, `Retry`.
- Context: `SetCtxProperty`, `RemoveCtxProperty`, `Transform`.
- HTTP/auth: `HttpRequest`, `VerifyJWT`, `SignJWT`.
- Composition: `Chain`, `AsyncWait`, `AsyncExec`, `SubJourney`, `SuspendFlow`, `WaitUntil`.
- Runtime: `Script`, `ExtendExp`.

See [Built-in steps](docs/features/built-in-steps.md) for behavior and configuration notes.

## Production checklist

- Use a stable `EncryptKey` from a secret manager.
- Use shared `JourneyStates` storage for multiple replicas.
- Configure `jwtek.external_jwks` for `VerifyJWT` `jwk_uri` performance.
- Use HTTPS and never log journey tokens in production.
- Inject a restricted HTTP client if journey configs can call external URLs.
- Restrict who can save journey and script configurations.
- Register custom steps, validators, scripts, and middleware during startup.
- Run race tests for async/composition changes.

## Documentation

Start here:

- [Feature guide](docs/features/README.md): application integration docs.
- [Developer guide](docs/dev/README.md): internals and maintainer docs.

Feature references:

- [Journey configuration](docs/features/journey-configuration.md)
- [Journey storage lifecycle](docs/features/journey-storage-lifecycle.md)
- [Invocation lifecycle](docs/features/invocation-lifecycle.md)
- [Optional Gin REST API](docs/features/rest-api.md)
- [Generic client inputs](docs/features/client-inputs.md)
- [Contexts and placeholders](docs/features/contexts-and-placeholders.md)
- [Built-in steps](docs/features/built-in-steps.md)
- [Composition and asynchronous work](docs/features/composition-and-async.md)
- [Extension points and production advice](docs/features/extensions-and-production.md)

Maintainer references:

- [Architecture](docs/dev/architecture.md)
- [Execution engine](docs/dev/execution-engine.md)
- [State, tokens, and persistence](docs/dev/state-tokens-and-persistence.md)
- [Step development](docs/dev/step-development.md)
- [Concurrency and security](docs/dev/concurrency-and-security.md)
- [Testing and release checks](docs/dev/testing.md)

## Verification

```bash
go vet ./...
go test ./...
go test -race ./...
```

Executable journey examples live in `test/config/auth/journeys`.
