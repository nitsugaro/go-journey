# Execution engine

The implementation is centered in `execution.journey.go`.

This page describes runtime control flow. It is the reference for changes that affect start, resume, pause, sub-journey return, terminal completion, or state persistence.

## Invocation phases

### 1. Select and consume state

`GetJourneyState` requires exactly one selector:

- `JourneyID`: initialize a new state and push the configured start step.
- `Jwt`: verify the signature, validate tracking, atomically consume server state by JTI, and merge server-only fields.
- `ResumeID`: atomically consume suspended state and recover its journey from closed context.

Tokens and resume IDs are single-use because the corresponding state is removed before execution.

### 2. Initialize invocation dependencies

`InvokeJourney`:

- Restores or decrypts contexts using the manager key.
- Selects public or encrypted storage for client-input configuration.
- Validates every provided client input against its stored request configuration.
- Generates a fresh JTI.
- Creates `JourneyTransaction` with context, registry, payload, CacheManager, and async error handling.

### 3. Consume tracking frames

`executeJourney` treats `TrackingsID` as a LIFO stack. Each frame is encoded as `journeyID:stepID`.

```text
pop frame
load journey
use start step when step ID is empty
execute outcomes in that journey
repeat while frames remain
```

SubJourney pushes the parent return frame followed by the child entry frame. The child is popped first. A terminal child writes its boolean result into closed context for the parent step.

### 4. Execute steps in one frame

`executeSteps` repeatedly:

1. Resolves the configured step and implementation.
2. Calls `Execute`.
3. Handles errors.
4. Handles terminal completion.
5. Pauses when new client inputs exist.
6. Yields to the tracking stack when outcome is empty without inputs.
7. Maps a non-empty outcome to the next step ID.

An empty outcome has two distinct meanings determined by side effects:

- With new inputs: persist and return to the caller.
- Without inputs: stop the current frame so tracking changes can take control.

### 5. Persist a pause

`pauseJourney`:

- Consumes any expiration extension.
- Serializes public context.
- Encrypts encrypted context.
- Creates either a signed token or resume-ID response.
- Clears restored in-memory maps from the persisted representation.
- Stores state with its TTL.

State persistence failure is terminal and returns `ErrJourneyStateStore`; never return a token that has no corresponding server state.

## Invariants

- A token JTI must have one server-side state entry.
- State retrieval for execution must be atomic and destructive.
- A paused current step must be pushed before serialization.
- Tracking frames must contain exactly one journey and step separator.
- A terminal result must not be persisted as resumable state.
- Client input type, step type, and ID must match the stored request configuration.
- The manager encryption key must be set before encrypted context is restored.

Changes violating these invariants often appear to work on the first invocation and fail only on resume.
