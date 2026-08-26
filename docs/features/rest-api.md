# Optional Gin REST API

The library can register administrative and invocation routes on a host-owned Gin engine or route group. It never starts an HTTP server automatically.

Use the REST API for admin tooling, development servers, or internal services that need journey/script CRUD and HTTP invocation. Do not expose it publicly without host-owned authentication, authorization, rate limits, and audit logging.

## Local deployment and smoke test

The repository includes a runnable development server. From the repository root:

```bash
go run ./test/cmd/rest-api \
  -config test/.config.json \
  -journeys test/config/auth/journeys \
  -scripts test/js-scripts
```

It listens on `127.0.0.1:8080`, exposes health at `GET /health`, and mounts the API at `/api/journey`. The example intentionally does not install authentication middleware. Run it only as a local example—the host application should supply production identity, authorization, TLS, limits, and storage paths.

The deployment smoke test builds the executable, starts it on a real TCP port, checks health, creates and executes a journey, deletes it, and verifies graceful shutdown:

```bash
go test ./test -run TestRESTServerDeploymentSmoke -count=1 -v
```

```go
router := gin.New()

manager := gojourney.NewManager(&gojourney.JourneyManagerConfig{
    JourneyStorage: storage,
    CacheManager:   cacheManager,
    EncryptKey:     []byte("0123456789abcdef0123456789abcdef"),
    RESTAPI: &gojourney.RESTAPIConfig{
        Enabled:  true,
        Router:   router,
        BasePath: "/api/journey",
        Middleware: []gin.HandlerFunc{
            requestIDMiddleware,
        },
        AdminMiddleware: []gin.HandlerFunc{
            requireAdministrator,
        },
        InvocationMiddleware: []gin.HandlerFunc{
            authenticateClient,
        },
        MaxBodyBytes: 1 << 20,
        PrepareExecution: func(c *gin.Context, execution *types.JourneyExecute) error {
            execution.IsConfidential = mayInvokeConfidentialJourney(c)
            execution.Payload.SetRealm(&types.Realm{Name: realmFromRequest(c)})
            return nil
        },
        OnSuccess: func(c *gin.Context, state *types.JourneyState) {
            auditSuccessfulJourney(c, state)
        },
        OnFailure: func(c *gin.Context, state *types.JourneyState, err error) {
            auditFailedJourney(c, state, err)
        },
    },
})
```

There are three middleware groups:

- `Middleware` applies to every REST API route.
- `AdminMiddleware` applies only to configuration CRUD routes: journeys, scripts, and step schemas.
- `InvocationMiddleware` applies only to journey invocation routes.

The example server leaves all three empty. Hosts can use them to enforce different authentication and authorization policies. Route registration fails during manager construction if enabled without a router or writable administrative stores.

## Routes

With the default base path `/journey`:

| Method   | Route                       | Behavior                                                                                                                                                                                                                                                                    |
| -------- | --------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GET`    | `/:realm`                   | List loaded journey definitions for one realm. Supports `name` and `limit` query filters.                                                                                                                                                                                   |
| `PUT`    | `/:realm`                   | Validate and save a journey in the route realm. If `id` exists and is found it updates that journey; otherwise it creates a new journey only when `name + realm` is not already used. Returns `201` for creation, `200` for update, and `409` for duplicate `name + realm`. |
| `GET`    | `/:realm/:journeyId`        | Read one journey, requiring its configured realm to match the route realm.                                                                                                                                                                                                  |
| `DELETE` | `/:realm/:journeyId`        | Delete one journey, requiring its configured realm to match the route realm.                                                                                                                                                                                                |
| `GET`    | `/:realm/scripts`           | List stored scripts. Supports `name`, `type`, and `limit` query filters.                                                                                                                                                                                                    |
| `PUT`    | `/:realm/scripts`           | Validate and save a journey/library script. If `id` exists and is found it updates that script; otherwise it creates a new script only when `name + type` is not already used. Returns `201` for creation, `200` for update, and `409` for duplicate `name + type`.         |
| `GET`    | `/:realm/scripts/:scriptId` | Read one script.                                                                                                                                                                                                                                                            |
| `DELETE` | `/:realm/scripts/:scriptId` | Delete the script and its compiled-program cache entry.                                                                                                                                                                                                                     |
| `GET`    | `/step-schemas`             | List registered step schemas. Supports `type`, `name`, and `limit` query filters.                                                                                                                                                                                           |
| `GET`    | `/step-schemas/:stepType`   | Read one registered step schema.                                                                                                                                                                                                                                            |
| `POST`   | `/:realm/invoke`            | Start or resume a journey in one realm through the ordinary invocation engine.                                                                                                                                                                                              |
| `ANY`    | `/<journey-name>/*path`     | After registered routing misses, use the first path segment to select an active `resource` journey.                                                                                                                                                                         |

CRUD defaults to the manager's `JourneyStorage` and cache-managed `ScriptStorage`. Custom administrative stores can be supplied through `RESTAPIConfig.JourneyStorage` and `ScriptStorage`. Journey CRUD responses hide generated step `vars` by default; set `RESTAPIConfig.ReturnVars` or `journey.rest_api.return_vars: true` to return them.

Route segments can be overridden in code through `RESTAPIConfig.Routes` or with `go-conf`:

```json
{
  "journey": {
    "rest_api": {
      "base_path": "/api/journey",
      "return_vars": false,
      "routes": {
        "journey_routes": ["/:realm/flows"],
        "journey_item_routes": ["/:realm/flows/:journeyId"],
        "script_routes": ["/:realm/js"],
        "script_item_routes": ["/:realm/js/:scriptId"],
        "script_binding_routes": ["/:realm/js/:scriptId/bindings"],
        "script_type_binding_routes": ["/:realm/script-bindings"],
        "step_schema_routes": ["/schemas"],
        "step_schema_item_routes": ["/schemas/:stepType"],
        "invoke_routes": ["/:realm/run"]
      }
    }
  }
}
```

With that configuration and base path, examples become `PUT /api/journey/alpha/flows`, `GET /api/journey/alpha/js`, `GET /api/journey/schemas/Success`, and `POST /api/journey/alpha/run`. By default the administrative API reads realm from `:realm`; custom host-based routes may omit `:realm` when earlier middleware sets Gin context key `realm`. Item routes must still include their resource id parameter (`:journeyId`, `:scriptId`, or `:stepType`). The broad journey item route is registered after more specific script, schema, and invoke routes so Gin does not capture paths like `/alpha/scripts` as a journey id. Resource journeys require no route template: they are attempted only after Gin finds no registered route. A non-empty `RESTAPIConfig.BasePath` overrides `journey.rest_api.base_path`.

The former `route_invoke_routes` setting and `/route/:realm/*routePath` endpoint are no longer used. `RESTAPIRoutes.RouteInvoke` remains only for Go source compatibility.

List responses use a stable query envelope:

```json
{
  "result": [],
  "resultCount": 0
}
```

## Invocation

Send the same payload used by the Go invocation API directly to `POST /:realm/invoke` or to a custom invoke route whose middleware sets Gin context key `realm`:

```json
{
  "journey_id": "journey-id"
}
```

For token continuation, send `journey_token` and submitted `client_inputs` in the same top-level object. For suspended continuation, send `resume_id`. The route realm is assigned to the execution before `PrepareExecution` runs, and it must match the configured realm of the journey being invoked. The client cannot set confidential permission directly; the host can still reject or refine execution in `PrepareExecution`.

A paused response is the normal journey payload directly:

```json
{
  "journey_token": "...",
  "client_inputs": []
}
```

A completed Success response exposes realm and terminal Success fields when configured:

```json
{
  "realm": "alpha",
  "success_url": "/home",
  "data": {}
}
```

A completed Failure response uses HTTP `401` and exposes realm and terminal Failure fields when configured:

```json
{
  "realm": "alpha",
  "failure_url": "/login",
  "data": {}
}
```

If a Failure step does not configure terminal response fields, the default error envelope is returned. `OnSuccess` receives the complete server-side state, including closed and encrypted context accessors. Those contexts are never serialized by the default REST response except for the explicit terminal fields written by Success/Failure steps. `OnFailure` receives execution failures such as an explicit Failure step.

The invocation handler creates a typed `JourneyRequest` snapshot from the HTTP envelope and propagates its cancellation context. Request bodies are bounded by `MaxBodyBytes` (default 1 MiB) before decoding. Oversized requests return `413`.

## Resource journey fallback invocation

Resource fallback invocation is for HTTP-like flows where the body, headers, query string and method belong to the target resource instead of the ordinary JSON journey payload. Gin evaluates registered REST routes first. For fallback resolution, the REST base path is removed and only the first remaining path segment is treated as the journey name:

- journey name `recurso` handles `/journey/recurso/...`;
- `/journey/recurso/users/42` selects `recurso` and exposes `/users/42` as `request.route_path`;
- names containing additional path segments are not considered: selection is exclusively by the first segment;
- only `resource` journeys are eligible;
- realm is not part of matching; if duplicate names exist across realms, the first eligible journey returned by storage is selected;
- the selected journey still executes with its own configured realm;
- matched names are cached as `name -> journey id` and the cache is cleared on journey save/delete;
- if no journey matches, the server returns an empty `404 Not Found` response, without exposing storage errors.

The selected journey receives a normal typed request snapshot plus:

```json
{
  "route_prefix": "/recurso",
  "route_path": "/users/42"
}
```

## Security

The routes have no built-in authentication policy because hosts use different identity systems. Enabling the API without appropriate middleware can expose journey definitions, script source code, invocation tokens, and mutation operations. At minimum:

- Protect administrative routes with strong authentication and authorization.
- Apply tenant/realm policy in middleware and `PrepareExecution`.
- Restrict confidential invocation explicitly.
- Set a body-size limit appropriate to the deployment.
- Treat uploaded JavaScript as trusted server-side code.
- Add rate limiting and audit logging at the Gin layer.
