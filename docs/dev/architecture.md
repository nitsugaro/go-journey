# Architecture

This page explains ownership boundaries. If you are adding a feature, use it to decide which package should own the behavior and which public extension point should be used.

## Packages

| Package | Responsibility |
|---|---|
| Root `gojourney` | Manager construction, journey loading, token lifecycle, execution, persistence, and public errors. |
| `types` | Journey configuration, state, transaction, step interfaces, registry, schemas, encryption helpers, and API payloads. |
| `steps` | Built-in step implementations, placeholder resolution, and composite execution helpers. |
| `inputs` | Client-input models, builders, and validators. |
| `env` | Environment-derived internal key names. |
| `cache` | Concurrency-safe registry of shared singleton dependencies, per-category limits, optional persisted constructor configurations, and the manager-owned compiled-regexp LRU. |
| `utils` | PromiseAll/PromiseAny concurrency primitives. |
| `test` | Executable configuration-driven integration scenarios and trace runner. |

## Main objects

`journeyManager` owns:

- Immutable journey configuration storage.
- A step registry.
- Single-use journey state storage.
- The AES context-encryption key.
- A shared cache manager for reusable dependencies.

`JourneyConfiguration` is the loaded graph. Configurations are shared between invocations and should be treated as immutable after manager construction.

`JourneyState` is resumable state:

- Tracking frames.
- Token ID (`Jti`).
- Four context scopes.
- Expiration duration.
- Host callbacks retained in server state.

`JourneyTransaction` is invocation-local execution state. It connects the current configuration, current step, input builder, registry, payload, request context, shared dependencies, and JourneyState.

## Control flow ownership

The executor is deliberately step-type agnostic. Steps communicate by:

- Returning an outcome string.
- Adding client inputs to request a pause.
- Pushing tracking frames to yield to another journey.
- Returning an error.
- Implementing `JourneyCompletion` when terminal success differs from the default.

The executor maps outcomes, persists pauses, and consumes the tracking stack. Do not add concrete step-type switches to the engine.

All ordinary and composite dispatch passes through `types.ExecuteStepConfig`. It resolves `vars` exactly once immediately before calling the implementation. Composite children retain their own variable descriptors so a sequential child observes state produced by its predecessors. The resolver has a zero-clone fast path when a configuration has no variables.

## Extension boundaries

- `types.IStep` is the behavioral extension point.
- `types.Steps` is the registry and schema boundary.
- `JourneyConfigurations` is the journey-definition loading boundary.
- `JourneyStates` is the persistence boundary.
- `JourneyTokens` is the authenticated resume-token boundary.
- `steps.HTTPClient` is the HTTP transport boundary.
- `inputs.RegisterValidator` is the custom input-validation boundary.
- `CacheManager` is manager-level singleton dependency injection with optional configuration persistence. Capacity policies belong to each cache category, so unrelated services never consume one another's quota.
- Manager-level journey terminal listeners and `OnAsyncError` are host lifecycle hooks.

Preserve these boundaries when adding features; importing application-specific services into the engine would reverse dependency direction.
