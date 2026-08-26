# Journey configuration

A journey is a directed graph stored as JSON. This page explains the file shape. Use [Journey storage lifecycle](journey-storage-lifecycle.md) for saving/validating definitions and [Invocation lifecycle](invocation-lifecycle.md) for running them.

Minimal mental model:

- top-level fields describe publication state, realm, expiry, and the start step;
- `steps` is a map of stable step IDs;
- each step chooses a registered `step_type`;
- each step's `outcome` maps returned outcome names to next step IDs;
- terminal steps finish the journey instead of pointing to another step.

## Top-level fields

| Field | Purpose |
|---|---|
| `id` | Stable journey identifier; normally matches the filename. |
| `name` | Human-readable name. |
| `description` | Optional documentation. |
| `active` | Inactive journeys cannot be invoked. |
| `confidential` | Prevents a new public invocation unless `IsConfidential` is set. Child/resumed execution remains possible. |
| `encrypted_client_inputs` | When true, client-input request definitions are stored in encrypted token context; otherwise they are stored in public token context. Form's private storage `id` and internal step correlation are excluded in both modes. |
| `debug` | Configuration flag kept for application metadata. Engine tracing is emitted through observers, not direct debug prints. |
| `default_exp` | Idle state lifetime in minutes. Each persisted pause uses this TTL unless suspended. |
| `realm` | Application-defined realm identifier. |
| `start_step_id` | First step ID for a new journey. |
| `sub_entries` | Application metadata for sub-entry relationships. |
| `steps` | Map of step IDs to step definitions. |
| `additional_properties` | Application-specific journey metadata available through `JourneyConfiguration.GetProp`. |

## Step definition

```json
"check-user": {
  "name": "Check user",
  "step_type": "IfExpression",
  "config": {
    "expression": "getClosedCtxProperty(\"journey_user_name\", \"\") != \"\"",
    "outcome": {
      "true": "success",
      "false": "failure"
    }
  }
}
```

`step_type` must exist in the manager's registry. `outcome` maps the string returned by a step to the next step ID. A missing mapping produces `ErrJourneyInvalidOutcome`.

Terminal steps do not require outcomes. Composite steps may contain nested step definitions in `config.steps`.

## Graph advice

- Give step IDs stable semantic names. Tokens and tracking frames store these IDs.
- Provide an explicit path to `Success` or `Failure` for every possible outcome.
- Do not rename a step while active tokens may still reference it.
- Do not remove or deactivate a child journey while a parent may have it on its tracking stack.
- Treat journey files as versioned application code. Review and test them with the same care as Go code.
- Prefer multiple focused journeys and `SubJourney` over one enormous graph.

## Registration versus constants

The constants in `types/step.type.go` describe known names across a wider ecosystem. A constant does not guarantee an implementation is registered. At runtime, inspect:

```go
registered := steps.GetDefaultStepRegistry().GetSteps()
```

The test runner fails if a registered default step lacks a journey fixture.

## Validation responsibility

`JourneyStorage.Save` automatically generates placeholder metadata and validates the complete journey before persistence. This includes step schemas, custom `VerifyConfig` rules, the start step, and outcome targets. Applications with custom persistence should call `types.PrepareJourneyConfiguration(journey, registry)` before inserting or updating. Manager startup loads existing files; it is not a substitute for publication-time validation.
