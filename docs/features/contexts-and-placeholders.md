# Contexts and placeholders

Journey state has four scopes. Use them intentionally; they define what survives a pause and who can read it.

| Scope | Placeholder prefix | Behavior |
|---|---|---|
| Public context | `ctx` | Signed into the journey token. Visible to the token holder; do not store secrets. |
| Encrypted context | `encCtx` | AES-GCM encrypted before entering the token. Use for confidential resumable data. |
| Closed context | `closedCtx` | Stored only on the server alongside the single-use token state. |
| Temporary context | `tempCtx` | Exists only during the current execution and is cleared across ordinary token pauses. |

Use getters rather than direct fields:

```go
transaction.State.GetCtx().Set("profile.name", "Ada")
transaction.State.GetEncryptedCtx().Set("oauth.refresh_token", token)
transaction.State.GetClosedCtx().Set("internal.user_id", userID)
transaction.State.GetTempCtx().Set("request.started", time.Now())
```

## Generated placeholder descriptors

Runtime does not repeatedly scan configuration strings. Developers author ordinary values containing placeholders; `JourneyStorage.Save` discovers them recursively and generates `config.vars` before validating and persisting the journey.

```json
{
  "body": "{\"user\":\"${closedCtx.journey_user_name}\"}",
  "vars": {
    "body": {
      "type": "string",
      "placeholders": [
        {
          "template": "closedCtx.journey_user_name",
          "starts_at": 9,
          "ends_at": 39
        }
      ]
    }
  }
}
```

The persisted result above is generated automatically. The configured property (`body`) is the single source value. `vars` contains only type and placeholder metadata; the map key identifies the configured property. `template` stores the context path without the `${...}` syntax because the offsets already identify the placeholder token. Offsets are byte offsets into the configured property; `starts_at` is inclusive and `ends_at` is exclusive. Placeholders are replaced from right to left so earlier offsets remain stable. Resolution happens through one execution boundary immediately before every built-in or custom step executes.

Composite steps keep each child's `vars` inside that child's own `config`. They do not resolve the complete child list when the parent starts. Consequently, sequential children can consume context written by earlier children, while concurrent children resolve against the state visible when their task executes. Configurations without `vars` use the immutable stored map directly and avoid serialization or cloning; configurations with variables receive an isolated resolved copy.

`type` controls the value delivered to the step and accepts `string`, `int`/`integer`, `float`/`number`, `bool`/`boolean`, `object`, or `array`. During `PrepareJourneyConfiguration`/`JourneyStorage.Save`, generation infers this type from the registered step implementation, including nested struct fields, collection elements, and map values. For example, `HttpRequest.re_execute_on_chain_step` produces `bool`, `headers.Authorization` produces `string`, and a complete `headers` property produces `object`. An explicitly authored `vars.type` takes precedence. If the registered field is `any` or its type cannot be determined, an exact placeholder leaves `type` empty to preserve the resolver's native value; a placeholder mixed with surrounding text explicitly generates `type: "string"`.

This allows numeric and boolean properties to use placeholders as well:

```json
{
  "max_wait_seconds": "${ctx.waitLimit}",
  "vars": {
    "max_wait_seconds": {
      "type": "int",
      "placeholders": [
        { "template": "ctx.waitLimit", "starts_at": 0, "ends_at": 16 }
      ]
    }
  }
}
```

The resolved variable replaces the property only in the isolated runtime configuration. Exact object and array placeholders preserve their structured values. A placeholder embedded in surrounding text produces a string. Nested properties use dot/index paths in `vars`, for example `fields.0.value`.

Placeholder-bearing strings are accepted by every built-in step schema for runtime configuration properties, including properties normally declared as numbers, booleans, enums, objects, arrays, and nested map/list values. When one placeholder occupies the complete property, the resolver's native type is preserved and must match the type consumed by the step. When a placeholder is mixed with other text, the result is always a string.

The following graph-defining properties intentionally remain static and reject placeholders:

- `outcome` targets, because they are validated links in the persisted journey graph.
- A nested step's `name` and `step_type`, because runtime selection of executable implementations is unsafe.
- The complete `steps` collection of a composition step. Properties inside each configured child step still support placeholders.

Dynamic URLs, HTTP methods, headers, secrets, expressions, context targets, and similar fields can materially change behavior. Only trusted developers should be allowed to save journey definitions, and custom placeholder resolvers must enforce authorization appropriate to the values they expose.

Supported syntax:

```text
${ctx.profile.id}
${encCtx.oauth.token}
${closedCtx.internal.user_id}
${tempCtx.request.id}
```

Missing built-in context values resolve to an empty string. `JourneyStorage.Save` rejects malformed placeholders, invalid step configurations, a missing start step, and outcomes targeting missing steps. Custom prefixes are checked against the manager's resolver map at execution time. Applications with custom persistence can call `types.PrepareJourneyConfiguration(journey, registry)` before writing.

## Custom placeholder prefixes

Applications can register resolvers for values that do not live in journey context:

```go
manager := gojourney.NewManager(&gojourney.JourneyManagerConfig{
    FolderPath: "journeys",
    PlaceholderResolvers: map[string]types.PlaceholderResolver{
        "secrets": func(path string) (any, error) {
            return secretStore.Get(path)
        },
    },
})
```

`${secrets.payments.apiKey}` invokes the `secrets` resolver with `payments.apiKey`. The prefix is the first path segment; the handler receives everything after it and may return any supported typed value. Handler errors stop step execution. Unknown prefixes also produce a resolution error.

Built-in context prefixes take precedence and cannot be replaced by custom resolvers. Resolver maps are copied when the manager is created and are propagated to ordinary, chained, and asynchronous step executions. Handlers used concurrently must therefore be safe for concurrent calls.

## Expressions

Expression steps use getter functions rather than `${...}` syntax:

```text
getCtxProperty("profile.age", 0) >= 18
getClosedCtxProperty("internal.enabled", false) == true
getEncCtxProperty("oauth.subject", "") != ""
getTempCtxProperty("request.valid", false)
```

The optional second argument is the default value.

## Encryption key requirements

Provide a stable 16-, 24-, or 32-byte `EncryptKey`. All replicas sharing journey state must use the same key. If no key is provided, the manager generates a process-local 32-byte key; that is convenient for local development but unsuitable for restarts, multiple replicas, or durable state.
