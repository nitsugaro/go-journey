# Testing and release checks

Use this page as the release checklist. Journey behavior crosses several calls, so unit tests alone are not enough.

## Test layers

### Unit tests

Use unit tests for behavior difficult to represent as a successful journey:

- Placeholder offset validation.
- Unregistered placeholders.
- Promise error aggregation.
- Store atomic consumption.
- Panic isolation.
- Input validators.

### Configuration-driven scenarios

`test/config/auth/journeys` contains real journey files. `test/main.go` provides local HTTP endpoints, supplies simulated client inputs, resumes tokens and suspended states, verifies timing, and prints complete traces.

Current scenarios cover:

- Sequential branching and dependent HTTP Chain calls.
- Registered placeholder replacement.
- Typed grouped Form validation (including a rejected submission), Choice, and Metadata resumes.
- Parallel slow HTTP calls with AsyncWait.
- Fire-and-forget webhooks with AsyncExec.
- SuspendFlow resume IDs.
- Expected business failure.
- Parent/child journey tracking.

`verifyEveryDefaultStepHasFixture` compares all nested fixture step types with the default registry. Adding a default implementation without a fixture fails the runner.

## Commands

Fast package verification:

```bash
go vet ./...
go test ./...
```

Readable full execution traces:

```bash
go test -v ./test
```

Race verification:

```bash
go test -race ./...
cd test
go run -race .
```

## Adding a step

1. Implement and register the step.
2. Add config validation tests.
3. Add a meaningful journey JSON fixture—not a trivial outcome-only example.
4. Extend the local scenario server when the use case needs HTTP behavior.
5. Add client-input simulation when the step pauses.
6. Assert timing when concurrency is the behavior being proved.
7. Run vet, unit tests, verbose scenarios, and both race commands.

## Changing execution or state

Explicitly test:

- New journey start.
- Ordinary token pause and resume.
- Invalid client input followed by retry.
- Suspend and resume ID.
- Token replay rejection.
- SubJourney success and failure return.
- State-store failure.
- Encrypted-context resume with a stable key.
- Callback success, failure, and panic.
- Concurrent custom steps and cancellation.

## Release review

The root Go package embeds `ui/dist`. Commit the complete production UI build
with each release so a fresh Go module download compiles without Node.js or a
frontend build step. Do not replace the assets with a placeholder file.

When UI sources or dependencies change, regenerate and commit the assets:

```bash
cd ui
npm ci
npm run build
cd ..
git add ui/dist
```

Before tagging a release, verify `go build ./...` and `go test ./...` from a
clean checkout without running the UI build first. Publish the packaging fix as
`v0.1.1`; keep the existing `v0.1.0` tag unchanged.

- Document exported API changes.
- Avoid removing extension points without a replacement and migration note.
- Verify old outstanding tokens when serialization changes.
- Verify all replicas can decrypt and consume shared state.
- Check fixture logs for unexpected extra invocations or tracking frames.
- Ensure no test or production log accidentally publishes secrets or tokens.
