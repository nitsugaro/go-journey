package steps_test

import (
	"testing"

	gojourney "github.com/nitsugaro/go-journey"
	jcache "github.com/nitsugaro/go-journey/cache"
	journeysteps "github.com/nitsugaro/go-journey/steps"
	"github.com/nitsugaro/go-journey/types"
	"github.com/nitsugaro/go-nstore"
	goutils "github.com/nitsugaro/go-utils/v2"
)

func TestSchemaValidatorValidatesRequestBodyJSON(t *testing.T) {
	transaction := newSchemaValidatorTransaction(t)
	transaction.Request = &types.JourneyRequest{
		Body: types.JourneyRequestBody{Data: []byte(`{"name":"Ada","age":37}`)},
	}
	config := goutils.NewTreeMap(map[string]any{
		"validations": []any{map[string]any{
			"source":         "body_json",
			"schema_id":      "profile",
			"output_context": "ctx",
			"output_path":    "validated.profile",
		}},
	})
	outcome, err := (&journeysteps.SchemaValidator{}).Execute(transaction, config)
	if err != nil || outcome != "valid" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
	if got := transaction.State.GetCtx().Get("validated.profile.name").AsStringOr(""); got != "Ada" {
		t.Fatalf("validated body was not saved, name=%q", got)
	}
}

func TestSchemaValidatorInvalidContextDoesNotNeedRequest(t *testing.T) {
	transaction := newSchemaValidatorTransaction(t)
	transaction.State.GetCtx().Set("profile", map[string]any{"name": "Al"})
	config := goutils.NewTreeMap(map[string]any{
		"validations": []any{map[string]any{
			"source":    "context",
			"schema_id": "profile",
			"context":   "ctx",
			"path":      "profile",
		}},
	})
	outcome, err := (&journeysteps.SchemaValidator{}).Execute(transaction, config)
	if err != nil || outcome != "invalid" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
	if !transaction.State.GetClosedCtx().IsDefined("schema_validation_error") {
		t.Fatal("validation error was not stored")
	}
}

func TestSchemaValidatorValidatesRequestHeaders(t *testing.T) {
	transaction := newSchemaValidatorTransaction(t)
	transaction.Request = &types.JourneyRequest{
		Headers: map[string][]string{
			"Content-Type": {"application/json"},
			"X-Trace":      {"abc", "def"},
		},
	}
	config := goutils.NewTreeMap(map[string]any{
		"validations": []any{map[string]any{
			"source":         "headers",
			"schema_id":      "headers",
			"output_context": "ctx",
			"output_path":    "validated.headers",
		}},
	})
	outcome, err := (&journeysteps.SchemaValidator{}).Execute(transaction, config)
	if err != nil || outcome != "valid" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
	if got := transaction.State.GetCtx().Get("validated.headers.Content-Type.0").AsStringOr(""); got != "application/json" {
		t.Fatalf("validated headers were not normalized/saved, content-type=%q", got)
	}
}

func newSchemaValidatorTransaction(t *testing.T) *types.JourneyTransaction {
	t.Helper()
	storage, err := gojourney.NewDeveloperSchemaStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.Save(&types.DeveloperSchema{
		Metadata: &nstore.Metadata{ID: "profile"},
		Name:     "profile",
		Schema: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []any{"name", "age"},
			"properties": map[string]any{
				"name": map[string]any{"type": "string", "minLength": 3},
				"age":  map[string]any{"type": "integer", "minimum": 18},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := storage.Save(&types.DeveloperSchema{
		Metadata: &nstore.Metadata{ID: "headers"},
		Name:     "headers",
		Schema: map[string]any{
			"type":                 "object",
			"additionalProperties": true,
			"required":             []any{"Content-Type"},
			"properties": map[string]any{
				"Content-Type": map[string]any{
					"type":     "array",
					"minItems": 1,
					"items":    map[string]any{"type": "string", "pattern": "^application/json$"},
				},
				"X-Trace": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "string"},
				},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	cacheManager, err := jcache.NewManager(&jcache.ManagerConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if err := cacheManager.UpdateRuntimeCacheInstance(journeysteps.SchemaStorageCacheKey, jcache.DefaultInstanceID, storage, 0); err != nil {
		t.Fatal(err)
	}
	transaction := newStepTransaction()
	transaction.CacheManager = cacheManager
	return transaction
}
