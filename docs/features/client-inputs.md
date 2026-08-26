# Generic client inputs

The `Form` step collects typed fields in one client interaction. It replaces separate field-specific steps such as username, email, age, or preferences.

Developer contract:

- `id` is private and controls where the accepted value is saved in context.
- `external_id` is public and is the only field identity the frontend should depend on.
- the engine stores private validation metadata server-side or encrypted according to journey configuration;
- no form values are written until the complete submitted form is valid.

## Example

```json
{
  "name": "Collect profile",
  "step_type": "Form",
  "config": {
    "context": "ctx",
    "object": "formData",
    "inputs": [
      {
        "id": "userName",
        "external_id": "profile.username",
        "label": "User name",
        "prompt": "Choose your user name",
        "type": "string",
        "required": true,
        "pattern": "^[A-Za-z][A-Za-z0-9_]+$",
        "min": 3,
        "max": 30,
        "user_name": true
      },
      {
        "id": "email",
        "external_id": "profile.email",
        "label": "Email",
        "type": "string",
        "required": true,
        "pattern": "^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$",
        "max": 254
      },
      {
        "id": "age",
        "external_id": "profile.age",
        "type": "int",
        "required": true,
        "min": 18,
        "max": 120
      },
      {
        "id": "newsletter",
        "external_id": "profile.newsletter",
        "type": "bool",
        "required": false
      }
    ],
    "outcome": { "true": "next" }
  }
}
```

This saves:

```json
{
  "formData": {
    "userName": "Ada_Dev",
    "email": "ada@example.com",
    "age": 36,
    "newsletter": true
  }
}
```

Without `object`, the attributes are saved directly as `ctx.userName`, `ctx.email`, and so on.

## Field properties

| Property | Required | Meaning |
|---|---:|---|
| `id` | Yes | Attribute path used when saving into context. |
| `external_id` | Yes | Stable ID exposed to and returned by the client. It is separate from storage layout. |
| `label` | No | Human-readable UI label. |
| `prompt` | No | Additional instruction for the user. |
| `type` | Yes | `string`, `int`, `float`, `bool`, or `object`. |
| `required` | No | Defaults to true. Missing required fields reject the whole submission. |
| `pattern` | No | Go regular expression for strings. |
| `min` / `max` | No | Numeric value bounds, or Unicode character-length bounds for strings. |
| `user_name` | No | For a string field, also stores the login hint at `<context-prefix>user_name`. |

`context` accepts `ctx`, `encCtx`, `closedCtx`, or `tempCtx`. Use encrypted or closed context for confidential form values. Temporary context lasts only for the current execution and is cleared across token pauses.

## Response contract

Each requested `ClientInput` returns both identities:

```json
{
  "external_id": "profile.email",
  "step_type": "Form",
  "type": "string",
  "send_back": true,
  "output": {
    "label": "Email",
    "required": true,
    "pattern": "...",
    "max": 254
  }
}
```

The client adds only `input` and returns the same metadata. `external_id` is the Form field's sole public identity. The private `id` used as the context storage target and the request's step correlation remain in server-only state. The engine restores that mapping after verifying the external ID, step type, and value type against the stored request.

## Validation and resume safety

Validation happens after token/state authentication but before the tracking frame is popped or the step executes. The engine checks:

- Every required field is present.
- No external ID appears twice.
- No unexpected field is submitted.
- Context ID, external ID, step type, and type match the original request.
- Runtime value has the declared type.
- Strings satisfy pattern and character-length bounds.
- Integers contain no fractional part.
- Numeric values satisfy min/max.
- Objects are maps or structs.

If any check fails, `InvokeJourney` returns `ClientError`, restores the consumed server state, and does not execute a step. The corrected submission can retry with the same token. No values are written until the complete form is valid.

After a valid Form step completes, its stored request metadata is removed before the journey advances.

## Login hints

`user_name: true` does not change validation or storage. It copies the accepted string into closed context under `env.GetContextKey("user_name")` so authentication integrations can use it as a login hint.
