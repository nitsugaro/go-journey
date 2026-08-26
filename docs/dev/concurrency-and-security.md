# Concurrency and security

This page covers the places where ordinary single-invocation execution becomes shared or sensitive: async steps, state stores, cache-managed singletons, tokens, client input, HTTP calls, and logs.

## Concurrency model

Ordinary journey execution is synchronous within one invocation. Concurrency is introduced only by AsyncWait, AsyncExec, concurrent callers, shared registries, caches, and custom host dependencies.

Protected engine structures include:

- Default journey state store.
- Step registry.
- Schema registry.
- Manager-owned regular-expression LRU singleton.
- Tracking stack operations.
- Lazy additional-properties initialization.

Async containers replace state contexts with synchronized TreeMaps before launching children. Each child receives:

- Its own JourneyTransaction.
- Its own synthetic current step ID.
- Its own client-input builder.
- Its own JourneyState wrapper.
- The same concurrency-safe CacheManager pointer.
- Shared synchronized context maps.

Do not reintroduce the parent transaction into a goroutine. A race can occur even when the child only reads `CurrentStepID`, because the executor advances it immediately after AsyncExec returns.

## Async safety rules

- Async children must be non-interactive and non-terminal.
- They must not push tracking frames.
- Cache-managed singleton values must be concurrency-safe.
- Cache-manager lookups share a read lock; updates, removals, persistence, and per-category limit checks use the exclusive lock.
- Replacing or removing an entry prevents new lookups from observing the old entry. Code that already obtained its pointer may finish using it, so instance lifetime and shutdown remain the host application's responsibility.
- `OnAsyncError` must be concurrency-safe.
- AsyncExec errors cannot change an already-returned journey outcome.
- AsyncWait child transports must honor context cancellation.
- Do not convert synchronized contexts back to ordinary maps while losing AsyncWait tasks may still exit.

## Token security

- JWT signature verification precedes state use.
- Tracking claims are structurally validated.
- Server state is atomically consumed to prevent replay.
- JTI and resume IDs use cryptographically secure randomness.
- Exactly one invocation selector is accepted.
- Encrypted context failures reject the token.
- State-store failure prevents returning an unusable token.

## Input security

Submitted inputs are matched against server-stored request configuration. The engine rejects:

- Inputs that were never requested.
- Duplicate IDs.
- Nil or empty-ID inputs.
- Step-type mismatches.
- Input-type mismatches.
- Unsupported validators.
- Validator-specific failures such as required values or pattern mismatch.

Regex compilation is cached by the manager-owned `cache.RegexpCache` under its own mutex. Its bounded LRU capacity comes from `journey.cache.regexp.max` (default 1000). Do not allow untrusted parties to publish arbitrary patterns without considering regular-expression resource usage.

## HTTP security

HttpRequest can become an SSRF primitive when configuration or placeholder values are attacker-controlled. Production hosts should inject a restricted `steps.HTTPClient` that enforces:

- Scheme and hostname allowlists.
- Connection, TLS, header, and body timeouts.
- Response-size limits.
- Redirect policy.
- Private-network restrictions where appropriate.
- Observability without logging credentials.

## Logging

Debug execution logs identify journeys, steps, outcomes, and pauses. The test runner additionally prints full tokens and contexts. Never enable equivalent token logging in production.

## Required verification

Concurrency-related changes are incomplete until both pass:

```bash
go test -race ./...
cd test && go run -race .
```

The second command matters because the test package normally launches its scenario runner as a subprocess without automatically inheriting the outer race instrumentation.
