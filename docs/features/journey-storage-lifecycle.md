# Journey storage lifecycle

`JourneyStorage` is the default filesystem-backed store for journey definitions. It handles creation, validation, update, list, read, and delete. Execution should receive the same storage instance so administrative changes and invocation use one source of truth.

## Initialize

Load config first, then create the registry, storage, cache manager, and journey manager:

```go
if err := goconf.LoadConfig(); err != nil {
    return err
}
env.SetEnvironment()

registry := steps.GetDefaultStepRegistry()

storage, err := gojourney.NewJourneyStorage("config/auth/journeys", registry)
if err != nil {
    return err
}

manager := gojourney.NewManager(&gojourney.JourneyManagerConfig{
    JourneyStorage: storage,
    EncryptKey:     []byte("0123456789abcdef0123456789abcdef"),
})
```

Use a stable encryption key from a secret manager in production.

## Save: create or update

Call `Save` for both creation and update.

```go
journey := &types.JourneyConfiguration{
    Name:        "welcome",
    Realm:       "alpha",
    Active:      true,
    DefaultExp:  10,
    StartStepID: "complete",
    Steps: map[string]*types.Step{
        "complete": {
            Name:     "Complete",
            StepType: types.SuccessStep,
            Config:   map[string]any{},
        },
    },
}

if err := storage.Save(journey); err != nil {
    return err
}

fmt.Println(journey.ID, journey.Rev)
```

If `journey.ID` is empty, storage assigns it. If `journey.ID` already exists, storage updates that file and writes a new revision.

`Save` does the important publication work:

- manages metadata fields such as ID, revision, creation time, and modification time;
- generates placeholder descriptors in `config.vars`;
- validates step schemas and custom `VerifyConfig` rules;
- validates `start_step_id`;
- validates outcome targets;
- validates nested steps;
- rejects broken graphs before persistence.

Keep `(realm, name)` unique at the API/application level so developers do not accidentally create multiple logical copies of the same journey.

## Read

```go
journey, err := storage.Load(journeyID)
if err != nil {
    return err
}

owner := journey.GetProp("editor.owner")
```

## List and lookup

```go
ids := storage.IDs()
journey, found := storage.GetJourneyByName("welcome")
```

REST list APIs add query filters such as `name` and `limit`; direct storage access stays intentionally simple.

## Delete

```go
if err := storage.Delete(journeyID); err != nil {
    return err
}
```

Deletion is immediate. Do not delete a journey while active tokens, suspended executions, or parent journeys may still reference it.

## Safe update rules

- Load the current journey, mutate it, and save it again.
- Do not rename or remove active step IDs while outstanding tokens may contain them.
- Do not remove a child journey while a parent may have it on its tracking stack.
- For incompatible changes, create a new journey version and retire the previous version after active executions expire.
- Treat journey files as application code: review, test, and deploy them intentionally.

## Invoke after saving

Storage only persists definitions. Invocation is done by the manager:

```go
response, state, err := manager.InvokeJourney(&types.JourneyExecute{
    Payload: (&types.JourneyPayloadReq{JourneyID: journey.ID}).
        SetRealm(&types.Realm{Name: journey.Realm}),
})
```

See [Invocation lifecycle](invocation-lifecycle.md) for tokens, client input, suspended resumes, and terminal results.

## Custom journey storage

The execution manager needs a read interface:

```go
type JourneyConfigurations interface {
    Load(id string) (*types.JourneyConfiguration, error)
}
```

Applications using a database, remote service, or embedded files own their mutation API. Before inserting or updating, call:

```go
err := types.PrepareJourneyConfiguration(journey, registry)
```

That applies the same placeholder generation and validation used by `JourneyStorage.Save`.
