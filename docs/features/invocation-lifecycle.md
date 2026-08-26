# Invocation lifecycle

This page explains how an already-saved journey runs. Storage and publication are covered in [Journey storage lifecycle](journey-storage-lifecycle.md).

Every invocation selects exactly one mode:

| Mode | Payload field | Meaning |
|---|---|---|
| Start | `JourneyID` | Start a new execution from the journey's `start_step_id`. |
| Token resume | `Jwt` / `journey_token` | Continue after a normal pause such as `Form`, `Choice`, or `Metadata`. |
| Suspended resume | `ResumeID` | Continue after `SuspendFlow` external callback state. |

## Start a journey

```go
response, state, err := manager.InvokeJourney(&types.JourneyExecute{
    Context: request.Context(),
    Payload: &types.JourneyPayloadReq{JourneyID: journeyID},
})
```

## Interpret the result

### Completed successfully

```text
response == nil
state    != nil
err      == nil
```

The final state contains public, encrypted, closed, and temporary contexts through its getters.

### Completed with failure

```text
response == nil
state    != nil
errors.Is(err, gojourney.ErrJourneyFailure) == true
```

Inspect server-controlled failure details in closed context when your Failure step configured them.

### Waiting for client input

```text
response != nil
response.Jwt != ""
state == nil
err == nil
```

Return `response.Jwt` and `response.ClientInputs` to the client. Submit the token and the requested inputs on the next call. Tokens are single-use: the corresponding server state is atomically consumed.

```go
response.ClientInputs[0].Input = submittedValue
next, state, err := manager.InvokeJourney(&types.JourneyExecute{
    Payload: &types.JourneyPayloadReq{
        Jwt:          response.Jwt,
        ClientInputs: response.ClientInputs,
    },
})
```

Unknown, duplicated, mismatched, or unsupported inputs are rejected through `ClientError`. The state is restored so a valid retry can use the token.

### Suspended for an external callback

```text
response != nil
response.Jwt == ""
state != nil
state.GetID() contains the resume ID
```

Store or deliver the callback URI returned by `SuspendFlow`. Resume with:

```go
response, state, err = manager.InvokeJourney(&types.JourneyExecute{
    Payload: &types.JourneyPayloadReq{ResumeID: resumeID},
})
```

Resume IDs are single-use. `SuspendFlow.exp` controls their TTL in seconds.

## Request-scoped integration

REST handlers can pass both the cancellation context and a typed, transport-neutral request snapshot. Read the body according to the host application's size limit, then provide those bytes to the adapter; it never consumes or closes `r.Body`:

```go
response, state, err := manager.InvokeJourney(&types.JourneyExecute{
    Context: r.Context(),
    Request: types.NewJourneyRequest(r, bodyBytes),
    Payload: payload,
})
```

`Context` controls cancellation and deadlines for operations such as HTTP calls. `Request` is an optional `*types.JourneyRequest` available to expression-driven and Script steps. It includes method, URI/path/query, multi-value headers, raw body bytes and media metadata, origin/base URL/host/port/protocol, HTTP and TLS versions, remote address, cookies, and peer-certificate summaries. For example, `SetCtxProperty` can store the incoming method with:

```json
{
  "type": "ctx",
  "key": "requestMethod",
  "expression": "request.Method",
  "outcome": {
    "true": "00000000-0000-0000-0000-000000000002",
    "false": "00000000-0000-0000-0000-000000000003"
  }
}
```

The snapshot pointer is propagated to chained and asynchronous child transactions. Body bytes, headers, query values, certificates, and cookies are copied by the adapter so later transport mutations do not alter journey-visible request data. Treat the snapshot as read-only after invocation. Journey definitions and scripts must still be trusted before exposing sensitive headers, cookies, certificates, or body content.

`JourneyExecute` supports:

- `Context`: cancellation and deadlines used by HTTP and waiting steps.
- `OnAsyncError`: errors from fire-and-forget work.
- `IsConfidential`: permission to start a confidential journey.

Terminal success and failure integration is registered on the manager with `OnJourneySuccess` and `OnJourneyFailure`. These listeners run asynchronously only when a journey finishes successfully or fails; suspended journeys do not trigger them.

The manager-owned `CacheManager` is attached automatically to every transaction and asynchronous child; it is not passed on each `JourneyExecute` call. Cache capacity is configured independently per category—for example, HTTP clients can have a different instance and byte limit from account services. See [Cache-managed dependencies](extensions-and-production.md#cache-managed-dependencies).

## Realm data

Attach application realm information to the payload before invocation:

```go
payload := (&types.JourneyPayloadReq{JourneyID: id}).SetRealm(realm)
```

The signed token stores only `"realm": "<realm-name>"`; mutable realm metadata such as active state and algorithms is not copied into journey state. Custom steps can read the name with `transaction.State.GetRealm()` or `transaction.Payload.GetRealm().Name`. Token and resume-ID continuations restore it automatically, so callers do not attach it again. The initial payload realm name overrides the journey configuration's `realm` when provided.
