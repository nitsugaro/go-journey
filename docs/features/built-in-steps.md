# Built-in steps

This page documents the implementations currently registered by `steps.GetDefaultStepRegistry()`.

Use this page to choose a step and understand its runtime contract. Use generated schemas from the REST API or registry when building editors/forms for exact field metadata.

All runtime configuration properties support generated placeholders, including typed and nested values. Persisted graph structure (`outcome`, nested step identity, and complete composition `steps` collections) remains static. See [Contexts and placeholders](contexts-and-placeholders.md#generated-placeholder-descriptors) for typing and security rules.

## Terminal steps

### Success

Ends the current journey successfully. When returning from a sub-journey, it marks the parent SubJourney step as successful.

### Failure

Ends the current journey with `ErrJourneyFailure`. Optional fields:

- `failure_url`: stored in closed context as `<context-prefix>failure_url`.
- `data`: stored in closed context as `<context-prefix>error_data`.

When used inside a sub-journey, it marks the parent result as false.

## Client interaction

### Form

The primary client-input step. It requests one or more typed fields in one pause, validates the complete submission before execution resumes, and saves accepted values into public, encrypted, closed, or temporary context. Values may be saved at the context root or grouped beneath an object key. See [Generic client inputs](client-inputs.md).

### Choice

Requests one of the keys declared in `outcome`. Values are presented in deterministic sorted order.

```json
{
  "default_choice": "approve",
  "outcome": {
    "approve": "approved",
    "reject": "rejected"
  }
}
```

### Metadata

Sends informational data to the client and pauses once. The next invocation acknowledges the message and follows `outcome.true`.

- `format: "TEXT"` returns a string.
- `format: "JSON"` parses the configured string as an object.
- `metadata` supports registered placeholders.

## Branching

### NotEmpty

Resolves its `string` property and returns `true` when the result is not empty, otherwise `false`.

### IfExpression

Evaluates one gval expression and returns `true` or `false`.

Expression bindings include context getter helpers, `request`, `payload`, `journey`, `currentStepID`, `realm`, and safe multi-value request accessors:

- `requestQuery.First(name, default)` returns the first query value or the default/empty string.
- `requestQuery.All(name)` returns all query values or an empty list.
- `requestQuery.Has(name)` returns true when at least one query value exists.
- `requestHeader` exposes the same methods for headers with case-insensitive names.

Prefer these helpers over direct indexed access such as `request.QueryParameters["name"][0]`, because missing request values are normal and direct gval indexing returns a technical evaluation error.

### SwitchExpression

Evaluates expressions in order and returns the `name` of the first true expression. Configure an outcome with the same name. If no expression matches, execution returns an invalid-outcome error.

### Assert

Evaluates named business invariants in `all` or `any` mode. It routes to `valid`, `invalid`, or `error` and stores structured failed-rule IDs and messages at the configured context target. Expression evaluation failures are technical `error` outcomes rather than business failures.

### VerifyJWT

Verifies a compact JWT using one of these modes:

- `plain-secret`: verifies an HMAC signature with a literal secret and explicit `algorithm` (`HS256`, `HS384`, or `HS512`).
- `base64url-secret`: verifies an HMAC signature with a base64url-encoded secret and explicit `algorithm` (`HS256`, `HS384`, or `HS512`). Invalid base64url fails instead of falling back.
- `jwk`: verifies with an inline JWKS or a remote `jwk_uri`; the expected format is always `{"keys":[...]}`. `algorithm` is a verifier hint/restriction (`HS*`, `RS*`, `PS*`, `ES*`, or `EdDSA`), not a requirement that the JWT header has `alg`. If `algorithm` is empty, the selected JWK must provide `alg`.
- `introspection`: posts the token to an OAuth introspection endpoint and requires `active: true`.
- `userinfo`: calls an OAuth UserInfo endpoint with the token as a bearer credential.

`validate_iat` and `validate_exp` validate those claims when present and default to true. `required_claims` maps claim paths to required values. When `output` is non-empty, the selected context receives `header`, `payload`, and the original base64url-encoded `signature`. Outcomes are `valid`, `invalid`, and `error`.

For JWKS key selection, a single key does not require `kid` unless the JWT header has `kid`; multiple keys require `kid` on every key and a matching JWT header `kid`.

`jwk_uri` requires caching. By default it uses `jwtek.external_jwks` from `go-jwte-manager`; configure `jwtek.external_jwks.folder` and `jwtek.external_jwks.cache_seconds` in `.config.json`. Register a custom `steps.JWKCacheKey` cache instance only when replacing that default backend. Treat secrets, inline private JWKs, decoded token output, and remote endpoint credentials as confidential configuration.

### SignJWT

Signs a compact JWT and saves it in the selected context. `algorithm` supports the same signature family as `VerifyJWT`: `HS*`, `RS*`, `PS*`, `ES*`, `ES256K`, and `EdDSA`.

`key.type` selects the signing key format:

- `plain-secret`: literal HMAC secret for `HS256`, `HS384`, or `HS512`.
- `base64url-secret`: base64url-encoded HMAC secret for `HS256`, `HS384`, or `HS512`. Invalid base64url fails instead of falling back.
- `pem`: private key in PEM format for asymmetric algorithms.
- `jwk`: private JWK for asymmetric algorithms.

`claims` and `headers` are free JSON objects and support placeholders. `alg` in `headers` is ignored because the step owns it through `algorithm`; set `key.kid` to emit `kid`. `issuer`, `subject`, and `audience` write standard claims. `set_jti` writes a random UUID into `jti`. `set_iat` defaults to true. `expires_in_seconds` only writes `exp` when greater than zero. If you need `nbf`, add it directly in `claims`. Outcomes are `true` and `error`.

### LDAP steps

LDAP steps use cache-managed repositories. Connection URLs, bind credentials, TLS, timeouts, and pool limits are configured once under `steps.LDAPClientCacheKey`; journey steps reference the instance with `connection`.

```go
_ = cacheManager.UpdateCacheInstance(
    steps.LDAPClientCacheKey,
    "corporate_ad",
    steps.LDAPClientConfig{
        URLs: []string{"ldaps://ldap.example.com:636"},
        BaseDN: "dc=example,dc=com",
        Bind: steps.LDAPBindConfig{
            Method: "simple",
            DN: "cn=journey,ou=services,dc=example,dc=com",
            Password: "...",
        },
    },
)
```

Search example:

```json
{
  "connection": "corporate_ad",
  "base_dn": "ou=people,dc=example,dc=com",
  "filter": "(uid=${ctx.username})",
  "attributes": ["uid", "mail", "memberOf"],
  "output": "closedCtx.ldap.user",
  "outcome": {
    "found": "next",
    "not_found": "failure",
    "error": "failure"
  }
}
```

`LDAPSearch` stores attributes as arrays, matching LDAP's multi-value model:

```json
{
  "count": 1,
  "entries": [
    {
      "dn": "uid=ada,ou=people,dc=example,dc=com",
      "attributes": {
        "uid": ["ada"],
        "mail": ["ada@example.com"],
        "memberOf": ["cn=admins,dc=example,dc=com"]
      }
    }
  ]
}
```

Available steps:

| Step | Operation | Outcomes |
|---|---|---|
| `LDAPSearch` | Search entries by base DN, scope, filter, and attributes. | `found`, `not_found`, `error` |
| `LDAPBind` | Validate credentials by binding on a fresh connection. | `valid`, `invalid`, `error` |
| `LDAPCompare` | Compare one DN attribute with one value. | `true`, `false`, `error` |
| `LDAPModify` | Add/delete/replace/increment attributes. | `success`, `not_found`, `error` |
| `LDAPAdd` | Add one entry. | `success`, `already_exists`, `error` |
| `LDAPDelete` | Delete one entry. | `success`, `not_found`, `error` |
| `LDAPModifyDN` | Rename or move one entry. | `success`, `not_found`, `error` |

## Context and operations

### Script

Executes a stored `journey` JavaScript program through `go-jsruntime`. Configuration:

- `script_id`: UUID of the stored script.
- `timeout_seconds`: positive execution limit, default 60; supports typed placeholders.
- `args`: optional object resolved with the same nested placeholder system as every other step.
- `outcome`: outcome names mapped to journey step IDs. In the Studio these
  names come from the selected script instead of being entered on each step.

Only `auth`, `resource`, and `workflow` scripts declare their available
outcomes in `additional_props.outcomes`. Values are trimmed, stored in
lowercase, deduplicated, and matched case-insensitively at runtime. Other
script types (`async`, `schedule`, `library`, and custom types) do not use this
reserved property.

Current JavaScript bindings are:

- `ctx`, `encCtx`, `closedCtx`, and `tempCtx`.
- `args`, `request`, `requestQuery`, `requestHeader`, and the string `realm`.
- `encoding`, `crypto`, and `logger` helpers.
- `setOutcome(name)`.
- `clientInputs.IsClientEmpty()`, `IsNewEmpty()`, `GetByExternalID(id)`, `GetByType(type)`, `AddValueInput(config)`, and `AddMessageInput(config)`.

`requestQuery` and `requestHeader` expose `First`, `All`, and `Has`. Missing values return a default, an empty string, or an empty list instead of throwing.

`logger` emits structured observer events instead of printing directly:

- `logger.Info(message, attrs?)`
- `logger.Event(message, attrs?)`
- `logger.Error(message, error?, attrs?)`

`AddValueInput` uses the generic Form input contract. Scripts provide `external_id`, `type`, and optional label, prompt, validation, and `user_name` properties; private IDs are neither required nor exposed, and the engine assigns the Script step type. A script may either set an outcome or request client inputs. Script cannot run inside `AsyncWait` or `AsyncExec` because JavaScript may mutate contexts or pause for client input.

Manager construction initializes the default runtime from `scripts.folder` (default `js-scripts`) and stores the manager/storage singletons under `steps.ScriptManagerCacheKey` and `steps.ScriptStorageCacheKey`. Applications and tests can install explicit instances with `steps.ConfigureScriptRuntime(cacheManager, manager, storage)` before constructing the journey manager. Scripts are trusted server-side code: restrict who can publish them and avoid exposing sensitive request objects unless the script is authorized to inspect them.

Applications can extend or replace bindings per script type through a manager-owned provider:

```go
type bindingsProvider struct{}

func (bindingsProvider) GetBindings(ctx *steps.ScriptBindingContext) (map[string]any, error) {
    return map[string]any{
        "tenant": tenantService.ForRealm(ctx.Transaction.State.GetRealm()),
    }, nil
}

err := steps.ConfigureScriptTypeBindings(
    cacheManager,
    steps.JourneyScript,
    steps.ScriptBindingsExtend,
    &bindingsProvider{},
)
```

`ScriptBindingsExtend` starts with the standard bindings for that script type and applies custom entries afterward, so matching names intentionally override defaults. `ScriptBindingsReplace` uses only the returned map. The provider receives the current transaction, isolated resolved args, script type, and the invocation's `DefaultBindings`; replacement providers can copy selected helpers such as `setOutcome` from that map. Providers are shared singleton dependencies and must support concurrent calls.

Custom script types can be registered once at startup:

```go
_ = steps.RegisterScriptType(steps.ScriptTypeDefinition{
    Type:        "risk-score",
    Name:        "Risk score script",
    Description: "Runs a risk scoring runtime with app-specific bindings.",
    Runnable:    true,
})

_ = steps.ConfigureScriptTypeBindings(
    cacheManager,
    "risk-score",
    steps.ScriptBindingsExtend,
    &riskScoreBindings{},
)
```

If the provider also implements `BindingDescriptors(*steps.ScriptBindingDescriptorContext)`, the REST API exposes those examples to the UI. `steps.ConfigureScriptBindings` still exists as a global compatibility hook; prefer `ConfigureScriptTypeBindings` for new code.

### Condition

Performs a simple typed comparison without an expression language. Configuration:

- `value`: value being evaluated; exact placeholders preserve their native value.
- `type`: `int`, `float`, `bool`, `string`, or `object`.
- `operation`: `present`, `not_present`, `equal`, `not_equal`, `min`, `max`, `starts_with`, `ends_with`, or `contains`.
- `compare_value`: required except for presence operations and also supports placeholders.
- `outcome`: `true` and `false`.

For numbers, `min` means `value >= compare_value` and `max` means `value <= compare_value`. For strings they compare Unicode character length. Prefix, suffix, and containment operations accept strings only. Nil, empty strings, and empty collections are considered not present.

### Random

Selects a named outcome using cryptographically secure randomness. `probabilities` maps each outcome name to a percentage and `outcome` maps the same names to step IDs:

```json
{
  "probabilities": { "control": 70, "variant": 30 },
  "outcome": { "control": "control-step", "variant": "variant-step" }
}
```

Probabilities must be non-negative, have matching outcome entries, and total 100. Decimal percentages are supported.

### Retry

Increments a stored attempt counter every time execution reaches the step. Configuration:

- `max_attempts`: positive attempt limit; supports typed placeholders.
- `context`: `ctx`, `encCtx`, `closedCtx`, or `tempCtx`.
- `counter`: nested property path where the incremented count is stored.
- `outcome.retry`: selected while the new count is less than `max_attempts`.
- `outcome.exhausted`: selected when the new count reaches or exceeds the limit.

The counter is retained after exhaustion. Use a context mutation step to reset or remove it when beginning a new retry cycle.

Retry cannot be nested in `AsyncWait` or `AsyncExec`, because incrementing a shared counter is intentionally serialized by ordinary journey execution.

### Transform

Builds or reshapes context values using static values and placeholders. Fields can preserve their source type or explicitly convert to `string`, `int`, `float`, `bool`, or `object`. `merge: false` replaces the configured parent target before fields are written; conversion failures route to `error`.

### ExtendExp

Adds or subtracts minutes from the TTL used at the next pause. `minutes` supports registered placeholders. The extension is consumed when state is persisted.

### HttpRequest

Sends an HTTP request and writes a response object to the configured context path.

```json
{
  "uri": "https://service.example/profile",
  "method": "GET",
  "headers": { "Accept": "application/json" },
  "response_output": "ctx.profile_response",
  "parse_json": true,
  "outcome": { "true": "next" }
}
```

Stored response fields include `status`, normalized `headers`, `duration`, and either `jsonBody` or `rawBody`. URI, method, and body support registered placeholders. Always set explicit network timeouts and restrict destinations when journey configuration is not fully trusted.

## Composition

### Chain

Runs nested steps sequentially in the same invocation. It is useful when later operations depend on earlier context output. The last non-empty nested outcome becomes the Chain outcome. Chain, SubJourney, AsyncWait, and AsyncExec cannot be nested directly inside Chain.

### AsyncWait

Runs independent non-interactive nested steps concurrently and waits for `ALL` or `ANY`. It supports a timeout in seconds and propagates child errors.

### AsyncExec

Starts non-interactive work in the background and immediately returns `true`. `SEQUENTLY` preserves child order in the background; `CONCURRENT` runs all children concurrently. Errors are reported through `JourneyExecute.OnAsyncError`.

### SubJourney

Pushes the parent return position and child entry onto the tracking stack. Options:

- `pass_tag`: skip a previously tagged child.
- `set_tag`: tag a successful child.
- `tag_name`: custom tag; defaults to the child journey ID.

### SuspendFlow

Pauses execution for an external callback and returns a single-use resume ID. `exp` is measured in seconds. `uri` may contain `{{resume_id}}`.

### WaitUntil

Suspends until an RFC 3339 timestamp. Timestamps without an offset use the configured timezone. `max_wait_seconds` is an optional safety limit and supports typed placeholders. Attempts to resume early receive a fresh single-use resume ID; the step advances only when the timestamp has been reached. Outcomes are `resumed`, `limit_exceeded`, and `invalid`. The host remains responsible for invoking the resume ID at the desired time.

## Steps prohibited in asynchronous containers

Interactive, terminal, and flow-mutating steps cannot run inside AsyncWait or AsyncExec:

- AsyncExec, AsyncWait, Chain, and SubJourney
- Success and Failure
- SuspendFlow and WaitUntil
- Form, Choice, and Metadata

Custom asynchronous steps must not request client input. Each child receives an isolated transaction snapshot and shared synchronized context maps.
