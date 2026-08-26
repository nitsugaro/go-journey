package journey_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gojourney "github.com/nitsugaro/go-journey"
	"github.com/nitsugaro/go-journey/types"
	"github.com/nitsugaro/go-nstore"
)

func TestJourneyStorageSaveGeneratesVariablesAndValidates(t *testing.T) {
	folder := t.TempDir()
	storage, err := gojourney.NewJourneyStorage(folder, nil)
	if err != nil {
		t.Fatal(err)
	}
	journey := &types.JourneyConfiguration{
		Metadata: &nstore.Metadata{ID: "generated-vars"}, Name: "generated-vars",
		Active: true, DefaultExp: 1, StartStepID: "00000000-0000-0000-0000-000000000101",
		Steps: map[string]*types.Step{
			"00000000-0000-0000-0000-000000000101": {
				Name: "Greeting", StepType: types.MetadataStep,
				Config: map[string]any{
					"metadata": "Hello ${tenant.profile.name} from ${ctx.realm}",
					"format":   "TEXT", "outcome": map[string]any{"true": "00000000-0000-0000-0000-000000000102"},
				},
			},
			"00000000-0000-0000-0000-000000000102": {Name: "Success", StepType: types.SuccessStep, Config: map[string]any{}},
		},
	}
	if err := storage.Save(journey); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(folder, journey.ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var persisted map[string]any
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatal(err)
	}
	config := persisted["steps"].(map[string]any)["00000000-0000-0000-0000-000000000101"].(map[string]any)["config"].(map[string]any)
	variables := config["vars"].(map[string]any)
	metadataVariable := variables["metadata"].(map[string]any)
	placeholders := metadataVariable["placeholders"].([]any)
	if len(placeholders) != 2 {
		t.Fatalf("generated placeholders = %v", placeholders)
	}
	first := placeholders[0].(map[string]any)
	if first["template"] != "tenant.profile.name" || first["starts_at"] != float64(6) || first["ends_at"] != float64(28) {
		t.Fatalf("first descriptor = %v", first)
	}
	journey.Steps["00000000-0000-0000-0000-000000000101"].Config.(map[string]any)["outcome"] = map[string]any{"true": "00000000-0000-0000-0000-000000000999"}
	if err := storage.Save(journey); err == nil {
		t.Fatal("journey with a missing outcome target was saved")
	}
}

func TestDeveloperSchemaStoragePersistsWithNStore(t *testing.T) {
	folder := t.TempDir()
	storage, err := gojourney.NewDeveloperSchemaStorage(folder)
	if err != nil {
		t.Fatal(err)
	}
	schema := &types.DeveloperSchema{
		Name:  "request-profile",
		Realm: "alpha",
		Schema: map[string]any{
			"type":     "object",
			"required": []any{"name"},
			"properties": map[string]any{
				"name": map[string]any{"type": "string", "minLength": 3},
			},
		},
	}
	if err := storage.Save(schema); err != nil {
		t.Fatal(err)
	}
	if schema.ID == "" {
		t.Fatal("schema id was not generated")
	}

	restarted, err := gojourney.NewDeveloperSchemaStorage(folder)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := restarted.Load(schema.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name != schema.Name || loaded.Realm != "alpha" {
		t.Fatalf("loaded schema mismatch: %+v", loaded)
	}
	if err := restarted.Validate(schema.ID, map[string]any{"name": "Ada"}); err != nil {
		t.Fatalf("persisted compiled schema did not validate: %v", err)
	}
}

func TestJourneyStorageValidatesStepFlowTypes(t *testing.T) {
	storage, err := gojourney.NewJourneyStorage(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}

	startID := "00000000-0000-0000-0000-000000000201"
	responseID := "00000000-0000-0000-0000-000000000202"
	finishID := "00000000-0000-0000-0000-000000000203"
	interactive := &types.JourneyConfiguration{
		Metadata:    &nstore.Metadata{ID: "resource-with-form"},
		Name:        "resource-with-form",
		JourneyType: types.ResourceJourney,
		Active:      true, DefaultExp: 1, StartStepID: startID, SubEntries: []string{},
		Steps: map[string]*types.Step{
			startID: {
				Name: "Collect user input", StepType: types.FormStep,
				Config: map[string]any{
					"context": "ctx",
					"inputs": []any{map[string]any{
						"id": "name", "external_id": "name", "label": "Name", "type": "string", "required": true,
					}},
					"outcome": map[string]any{"true": responseID},
				},
			},
			responseID: {
				Name:     "HTTP response",
				StepType: types.HTTPResponseStep,
				Config:   map[string]any{"status_code": 200, "outcome": map[string]any{"true": finishID}},
			},
			finishID: {Name: "Finish response", StepType: types.HTTPFinishResponseStep, Config: map[string]any{}},
		},
	}
	if err := storage.Save(interactive); err == nil || !strings.Contains(err.Error(), types.ResourceJourney) {
		t.Fatalf("resource journey accepted interactive form step: %v", err)
	}

	nonInteractive := &types.JourneyConfiguration{
		Metadata:    &nstore.Metadata{ID: "resource-with-set-ctx"},
		Name:        "resource-with-set-ctx",
		JourneyType: types.ResourceJourney,
		Active:      true, DefaultExp: 1, StartStepID: startID, SubEntries: []string{},
		Steps: map[string]*types.Step{
			startID: {
				Name: "Set property", StepType: types.SetCtxPropertyStep,
				Config: map[string]any{
					"type": "ctx", "key": "started", "expression": "true",
					"outcome": map[string]any{"true": responseID, "false": responseID},
				},
			},
			responseID: {
				Name:     "HTTP response",
				StepType: types.HTTPResponseStep,
				Config:   map[string]any{"status_code": 200, "outcome": map[string]any{"true": finishID}},
			},
			finishID: {Name: "Finish response", StepType: types.HTTPFinishResponseStep, Config: map[string]any{}},
		},
	}
	if err := storage.Save(nonInteractive); err != nil {
		t.Fatalf("resource journey rejected non-interactive step: %v", err)
	}
}
