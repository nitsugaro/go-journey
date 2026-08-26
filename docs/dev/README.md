# Developer guide

This section is for maintainers changing the engine, built-in steps, storage, token behavior, or public APIs.

Read in this order:

1. [Architecture](architecture.md): packages, main objects, extension boundaries, and the rule that execution stays step-type agnostic.
2. [Execution engine](execution-engine.md): invocation phases, tracking stack, pause persistence, terminal completion, and invariants.
3. [State, tokens, and persistence](state-tokens-and-persistence.md): split state, signed tokens, resume IDs, encryption, TTL, and storage contracts.
4. [Step development](step-development.md): step interface, schemas, config validation, contexts, inputs, composition, scripts, and terminal behavior.
5. [Concurrency and security](concurrency-and-security.md): async execution, shared caches, token safety, input validation, HTTP security, and logging.
6. [Testing and release checks](testing.md): required unit, fixture, race, and scenario checks.

## Maintainer rules

- Do not add concrete step-specific switches to the executor.
- Resolve placeholders centrally through `types.ExecuteStepConfig`.
- Keep journey configuration immutable during execution.
- Keep public client responses free of private context IDs and server-only details.
- Treat `ctx` as visible to the token holder; secrets belong in `encCtx` or `closedCtx`.
- Shared cache instances must be concurrency-safe.
- Any behavior that spans multiple invocations needs tests for start, pause, resume, invalid input retry, replay, and terminal completion.

Before merging execution or state changes, run:

```bash
go vet ./...
go test ./...
go test -race ./...
```
