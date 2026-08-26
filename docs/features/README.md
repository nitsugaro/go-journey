# Feature guide

This section is for application developers using Go Journey in a service.

Read in this order:

1. [Journey configuration](journey-configuration.md): JSON graph shape, step config, outcomes, realms, and validation rules.
2. [Journey storage lifecycle](journey-storage-lifecycle.md): create, validate, save, update, list, delete, and load journey definitions.
3. [Invocation lifecycle](invocation-lifecycle.md): start, pause for input, resume with token, suspend with resume ID, and complete.
4. [Contexts and placeholders](contexts-and-placeholders.md): `ctx`, `encCtx`, `closedCtx`, `tempCtx`, generated `vars`, typed placeholders, and extension values.
5. [Generic client inputs](client-inputs.md): `Form` behavior, private `id`, public `external_id`, validation, and response privacy.
6. [Built-in steps](built-in-steps.md): the registered step catalog and important configuration notes.
7. [Composition and asynchronous work](composition-and-async.md): `Chain`, `AsyncWait`, `AsyncExec`, and `SubJourney`.
8. [Optional Gin REST API](rest-api.md): REST CRUD, invocation, route customization, middleware, and response behavior.
9. [Extension points and production advice](extensions-and-production.md): custom steps, storage, tokens, cache-managed dependencies, callbacks, and deployment concerns.

The executable examples in `test/config/auth/journeys` are the canonical end-to-end references. Use them to confirm how a documented behavior looks in a complete journey file.

## Common tasks

| Task | Start here |
|---|---|
| Build or edit a journey JSON file | [Journey configuration](journey-configuration.md) |
| Save journeys through code | [Journey storage lifecycle](journey-storage-lifecycle.md) |
| Invoke from Go | [Invocation lifecycle](invocation-lifecycle.md) |
| Invoke through HTTP | [Optional Gin REST API](rest-api.md) |
| Collect client input | [Generic client inputs](client-inputs.md) |
| Use runtime values in config | [Contexts and placeholders](contexts-and-placeholders.md) |
| Choose a built-in step | [Built-in steps](built-in-steps.md) |
| Configure production dependencies | [Extension points and production advice](extensions-and-production.md) |
