# Composition and asynchronous work

Choose composition based on dependency and completion semantics. Do not use composition only to make a JSON file look organized; each composition step changes execution behavior.

## Chain: sequential dependency

Use Chain when operation B consumes context written by operation A.

Example: create a session, then audit its returned ID. The second request body can use a registered `${ctx...}` placeholder pointing at the first response. See journey `00000000-0000-0000-0000-000000000002`.

Do not use Chain merely to group unrelated slow calls; they will run one after another.

## AsyncWait: independent work required before continuing

Use AsyncWait when multiple independent operations can overlap but the journey needs their results.

Typical examples:

- Fetch profile and permissions concurrently.
- Run independent fraud and eligibility checks.
- Query multiple providers and accept the first successful result with `ANY`.

```json
{
  "wait_for": "ALL",
  "timeout": "2",
  "steps": [
    { "step_type": "HttpRequest", "config": { "uri": "...", "response_output": "ctx.profile" } },
    { "step_type": "HttpRequest", "config": { "uri": "...", "response_output": "ctx.permissions" } }
  ],
  "outcome": { "true": "verify", "false": "failure" }
}
```

The executable fixture performs two 300 ms calls in approximately 300 ms, proving parallel execution.

For `ANY`, losing tasks receive cancellation through their context, but their underlying operation must honor context cancellation.

## AsyncExec: work that must not delay the journey

Use AsyncExec for non-critical side effects:

- Audit webhooks.
- Analytics events.
- Cache warming.
- Best-effort notifications.

Never use it for work whose success determines the journey result. The journey can complete before background work finishes.

Always configure `OnAsyncError`:

```go
execute := &types.JourneyExecute{
    Payload: payload,
    OnAsyncError: func(step types.Step, err error) {
        logger.Error("async journey step failed", "step", step.Name, "error", err)
    },
}
```

The error handler must be concurrency-safe. Its panic is isolated from the engine.

## SubJourney: reusable business flow

Use SubJourney when the child is a meaningful reusable workflow with its own start, terminal result, and possible client interactions. The engine treats tracking frames as a LIFO call stack:

```text
parent return frame
child current frame  <- executed first
```

Child Success writes true to the parent step marker; child Failure writes false. The parent SubJourney consumes that marker and chooses its configured outcome.

## Concurrency rules

- Context maps used by asynchronous children are synchronized.
- Each child gets its own current step ID, request context, and input builder while sharing the concurrency-safe CacheManager.
- Cache instances are not cloned. Shared dependencies must be concurrency-safe.
- Async children cannot manipulate the journey tracking stack or request client interaction.
- A custom HTTP client should honor `RequestWithContext` cancellation.
