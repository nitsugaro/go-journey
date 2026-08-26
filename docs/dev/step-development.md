# Step development

This page is for adding or changing step implementations. A step owns its business behavior; the executor must stay generic and should not learn concrete step types.

## Contract

Every step implements:

```go
type IStep interface {
    GetStepType() string
    EndJourney() bool
    VerifyConfig(stepName string, config goutils.TreeMapImpl) error
    Execute(transaction *JourneyTransaction, config goutils.TreeMapImpl) (string, error)
}
```

Embed `steps.BasicStep` for non-terminal defaults.

## Outcome behavior

- Return a configured outcome such as `true`, `false`, or a dynamic name to continue in the current journey.
- Return empty outcome after pushing tracking frames to yield control.
- Add client inputs and return empty outcome to pause.
- Return an error for an execution failure.
- Terminal steps return `EndJourney() == true`.
- A failed terminal step implements `JourneyCompletion` and returns false from `JourneySucceeded`.

Do not make the executor recognize your concrete step type.

## Configuration validation

Use struct tags to generate schemas. Override `VerifyConfig` for relationships JSON Schema cannot express, such as prohibited nested steps.

Schema resource identifiers use `journey.base_url` from `go-conf`:

```json
{
  "journey": {
    "base_url": "https://journey.example.com"
  }
}
```

Resources are registered as `<base_url>/schemas/<StepType>.json`. The default is `https://localhost:3000`. Registries created before `LoadConfig` recompile their existing resources after configuration loads.

Return typed errors:

```go
return types.StepInvalidConfig(stepName, "output path is required")
return types.StepInvalidOutcome(stepName, outcome)
return types.StepNotFound(stepType)
```

Never assume map conversion or nested fields succeed. Missing configuration should return an error, not panic.

## Context access

- Use state getters; do not access private restored fields.
- Check `GetCtxPath` results before writing.
- Put secrets in encrypted or closed context.
- Avoid writing temporary values needed after a pause.
- Read values directly from the configuration passed to `Execute`; placeholders have already been resolved centrally.

## Client input steps

Use `ClientInputsBuilder` to add a typed request. The builder stores the validation configuration in the selected context. On resume, the engine checks ID, type, step type, duplication, and the registered validator before the step executes.

An interactive step should:

1. Look for its input by current step ID.
2. Add a new input and return empty outcome when absent.
3. Parse and store the accepted value when present.
4. Return a configured outcome.

Interactive steps cannot run inside AsyncWait or AsyncExec.

Prefer the built-in generic Form step over adding a new step for each primitive form field. A specialized input step is justified only when interaction semantics differ substantially from typed value collection. Form separates the context `id` from the client-facing `external_id`, validates the entire requested form before execution, and supports grouped context output.

## Composite steps

Always resolve nested implementations through `transaction.Steps`. This preserves custom registries.

Restore any transaction fields you temporarily modify, including on error. Chain uses `defer` for this reason.

Concurrent children need isolated transactions. Use the established clone helper so current step IDs and input builders are not raced. Synchronized context maps and the CacheManager are shared.

## Registration and fixtures

Register a schema and implementation together with `Steps.AddStep`. Add at least:

- Unit tests for config and edge cases.
- A meaningful JSON journey fixture.
- Scenario-runner support when client input or external dependencies are required.

The fixture coverage check deliberately fails when a registered default step has no JSON configuration example.

A registry passed to `JourneyManagerConfig.Steps` extends rather than replaces the defaults. The manager copies missing built-ins with their original schemas. Registering the same step type before manager construction is the supported way to override a default.
