package gojourney

import (
	"reflect"
	"testing"

	"github.com/nitsugaro/go-journey/types"
	"github.com/nitsugaro/go-nstore"
)

func TestRuntimeJourneyCompilesEnvOnceAndKeepsDynamicPlaceholders(t *testing.T) {
	const (
		transformID = "00000000-0000-0000-0000-000000000101"
		successID   = "00000000-0000-0000-0000-000000000102"
		failureID   = "00000000-0000-0000-0000-000000000103"
	)
	journey := &types.JourneyConfiguration{
		Metadata: &nstore.Metadata{ID: "runtime-static-env"}, Name: "runtime-static-env",
		Active: true, DefaultExp: 1, StartStepID: transformID,
		Steps: map[string]*types.Step{
			transformID: {
				Name: "Resolve configuration", StepType: types.TransformStep,
				Config: map[string]any{
					"target_context": "ctx",
					"fields": []any{
						map[string]any{"target": "endpoint", "value": "https://${env.host}/${ctx.initial_data.path}"},
						map[string]any{"target": "routes", "value": "${env.routes}"},
						map[string]any{"target": "token", "value": "${secrets.token}"},
					},
					"outcome": map[string]any{"true": successID, "error": failureID},
				},
			},
			successID: {Name: "Success", StepType: types.SuccessStep, Config: map[string]any{}},
			failureID: {Name: "Failure", StepType: types.FailureStep, Config: map[string]any{}},
		},
	}
	storage, err := NewJourneyStorage(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.Save(journey); err != nil {
		t.Fatal(err)
	}
	envCalls := map[string]int{}
	secretCalls := 0
	manager := NewManager(&JourneyManagerConfig{
		JourneyStorage: storage,
		EncryptKey:     []byte("0123456789abcdef0123456789abcdef"),
		PlaceholderResolvers: map[string]types.PlaceholderResolver{
			"env": func(path string) (any, error) {
				envCalls[path]++
				switch path {
				case "host":
					return "api.example.test", nil
				case "routes":
					return []any{"/one", "/two"}, nil
				default:
					return nil, nil
				}
			},
			"secrets": func(path string) (any, error) {
				secretCalls++
				return "secret-" + path, nil
			},
		},
	})

	compiled, err := manager.loadJourney(journey.ID)
	if err != nil {
		t.Fatal(err)
	}
	again, err := manager.loadJourney(journey.ID)
	if err != nil {
		t.Fatal(err)
	}
	if compiled != again {
		t.Fatal("unchanged journey was compiled more than once")
	}
	if envCalls["host"] != 1 || envCalls["routes"] != 1 {
		t.Fatalf("env resolver calls = %v", envCalls)
	}

	compiledConfig := compiled.Steps[transformID].Config.(map[string]any)
	compiledFields := compiledConfig["fields"].([]any)
	if got := compiledFields[0].(map[string]any)["value"]; got != "https://api.example.test/${ctx.initial_data.path}" {
		t.Fatalf("compiled mixed value = %v", got)
	}
	if got := compiledFields[1].(map[string]any)["value"]; !reflect.DeepEqual(got, []any{"/one", "/two"}) {
		t.Fatalf("compiled typed array = %#v", got)
	}
	variables := compiledConfig["vars"].(map[string]any)
	if _, found := variables["fields.1.value"]; found {
		t.Fatal("static env array retained a runtime variable descriptor")
	}
	if _, found := variables["fields.0.value"]; !found {
		t.Fatal("mixed value lost its ctx variable descriptor")
	}
	if _, found := variables["fields.2.value"]; !found {
		t.Fatal("non-env custom placeholder lost its variable descriptor")
	}

	for _, path := range []string{"first", "second"} {
		state := types.NewJourneyState()
		state.GetCtx().Set("initial_data.path", path)
		resolved, err := types.ResolveStepConfig(compiledConfig, state, manager.placeholderResolvers)
		if err != nil {
			t.Fatal(err)
		}
		fields, err := resolved.Get("fields").AsSlice()
		if err != nil {
			t.Fatal(err)
		}
		if got := fields[0].Get("value").AsStringOr(""); got != "https://api.example.test/"+path {
			t.Fatalf("dynamic ctx value = %q", got)
		}
		if got := fields[2].Get("value").AsStringOr(""); got != "secret-token" {
			t.Fatalf("dynamic custom value = %q", got)
		}
	}
	if secretCalls != 2 {
		t.Fatalf("dynamic resolver calls = %d, want 2", secretCalls)
	}
	if envCalls["host"] != 1 || envCalls["routes"] != 1 {
		t.Fatalf("env was resolved during step execution: %v", envCalls)
	}

	persisted, err := storage.Load(journey.ID)
	if err != nil {
		t.Fatal(err)
	}
	persistedFields := persisted.Steps[transformID].Config.(map[string]any)["fields"].([]any)
	if got := persistedFields[0].(map[string]any)["value"]; got != "https://${env.host}/${ctx.initial_data.path}" {
		t.Fatalf("persisted placeholder was modified: %v", got)
	}
	if got := persistedFields[1].(map[string]any)["value"]; got != "${env.routes}" {
		t.Fatalf("persisted typed placeholder was modified: %v", got)
	}

	journey.Description = "new revision"
	if err := storage.Save(journey); err != nil {
		t.Fatal(err)
	}
	recompiled, err := manager.loadJourney(journey.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recompiled == compiled {
		t.Fatal("new journey revision reused the old compiled journey")
	}
	if envCalls["host"] != 1 || envCalls["routes"] != 1 {
		t.Fatalf("new revision re-resolved deployment-static env values: %v", envCalls)
	}
}

func TestRuntimeJourneyRequiresEnvResolverOnlyWhenUsed(t *testing.T) {
	const successID = "00000000-0000-0000-0000-000000000201"
	journey := &types.JourneyConfiguration{
		Metadata: &nstore.Metadata{ID: "missing-env-resolver"}, Name: "missing-env-resolver",
		Active: true, DefaultExp: 1, StartStepID: successID,
		Steps: map[string]*types.Step{
			successID: {Name: "Success", StepType: types.SuccessStep, Config: map[string]any{"data": "${env.required}"}},
		},
	}
	storage, err := NewJourneyStorage(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.Save(journey); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(&JourneyManagerConfig{JourneyStorage: storage})
	if _, err := manager.loadJourney(journey.ID); err == nil {
		t.Fatal("journey using env compiled without an env resolver")
	}
}
