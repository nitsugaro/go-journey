package steps

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	jcache "github.com/nitsugaro/go-journey/cache"
	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

const SchemaStorageCacheKey = "schema_storage"

type SchemaValidator struct {
	BasicStep

	_           struct{}                 `description:"Validates request or context data against developer-defined JSON Schemas."`
	Validations []SchemaValidationConfig `json:"validations" required:"true" minItems:"1" description:"Validation rules to execute."`
	Outcome     struct {
		Valid   string `json:"valid" required:"true" format:"uuid"`
		Invalid string `json:"invalid" required:"true" format:"uuid"`
		Error   string `json:"error" required:"true" format:"uuid"`
	} `json:"outcome" required:"true"`
}

type SchemaValidationConfig struct {
	Source        string `json:"source" enum:"query_params,headers,body_json,context" required:"true" description:"Data source to validate."`
	SchemaID      string `json:"schema_id" required:"true" description:"Developer schema identifier."`
	Context       string `json:"context" enum:"ctx,encCtx,closedCtx,tempCtx" default:"ctx" description:"Context used when source is context."`
	Path          string `json:"path,omitempty" description:"Dot path used when source is context."`
	OutputContext string `json:"output_context,omitempty" enum:"ctx,encCtx,closedCtx,tempCtx" description:"Optional context where validated data will be saved."`
	OutputPath    string `json:"output_path,omitempty" description:"Optional dot path where validated data will be saved."`
}

func (*SchemaValidator) GetStepType() string {
	return types.SchemaValidatorStep
}

func (*SchemaValidator) Execute(transaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	provider, err := schemaProvider(transaction.CacheManager)
	if err != nil {
		return "error", err
	}
	var validations []SchemaValidationConfig
	if err := config.Get("validations").AsStruct(&validations); err != nil || len(validations) == 0 {
		return "error", errors.New("validations are required")
	}
	for index := range validations {
		validation := &validations[index]
		data, err := validationData(transaction, validation)
		if err != nil {
			return "error", fmt.Errorf("validation %d data: %w", index, err)
		}
		data, err = jsonSchemaCompatibleValue(data)
		if err != nil {
			return "error", fmt.Errorf("validation %d data: %w", index, err)
		}
		if err := provider.Validate(validation.SchemaID, data); err != nil {
			transaction.EmitEvent(&types.Event{
				Type:    types.EventFailed,
				Message: "failed validation",
				Error:   err,
				Subject: types.EventSubject{
					Type: "step", ID: transaction.CurrentStepID, Name: transaction.Journey.Steps[transaction.CurrentStepID].Name,
				},
			})

			transaction.State.GetClosedCtx().Set("schema_validation_error", err.Error())
			return "invalid", nil
		}
		if strings.TrimSpace(validation.OutputContext) != "" && strings.TrimSpace(validation.OutputPath) != "" {
			ctx := transaction.State.Get(validation.OutputContext)
			if ctx == nil {
				return "error", fmt.Errorf("validation %d output context is unsupported", index)
			}
			ctx.Set(validation.OutputPath, data)
		}
	}
	transaction.State.GetClosedCtx().TryDelete("schema_validation_error")
	return "valid", nil
}

func schemaProvider(cacheManager *jcache.Manager) (types.DeveloperSchemaProvider, error) {
	if cacheManager == nil {
		return nil, errors.New("schema storage is not configured")
	}
	value, found := cacheManager.GetCacheInstance(SchemaStorageCacheKey, jcache.DefaultInstanceID)
	if !found {
		return nil, errors.New("schema storage is not configured")
	}
	provider, ok := value.(types.DeveloperSchemaProvider)
	if !ok || provider == nil {
		return nil, errors.New("schema storage cache has an invalid type")
	}
	return provider, nil
}

func validationData(transaction *types.JourneyTransaction, validation *SchemaValidationConfig) (any, error) {
	if validation == nil {
		return nil, errors.New("validation is nil")
	}
	switch validation.Source {
	case "query_params":
		if transaction.Request == nil {
			return nil, errors.New("request is not available")
		}
		return transaction.Request.QueryMap(), nil
	case "headers":
		if transaction.Request == nil {
			return nil, errors.New("request is not available")
		}
		return transaction.Request.HeaderMap(), nil
	case "body_json":
		return parseRequestJSONBody(transaction.Request)
	case "context":
		ctx := transaction.State.Get(validation.Context)
		if ctx == nil {
			return nil, errors.New("context is not supported")
		}
		if strings.TrimSpace(validation.Path) == "" {
			return ctx.AsMap()
		}
		return ctx.Get(validation.Path).AsAny()
	default:
		return nil, fmt.Errorf("unsupported source %q", validation.Source)
	}
}

func parseRequestJSONBody(request types.RequestAccessor) (any, error) {
	if request == nil {
		return nil, errors.New("request is not available")
	}
	body, err := request.BodyBytes()
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, errors.New("request body is empty")
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func jsonSchemaCompatibleValue(value any) (any, error) {
	switch typed := value.(type) {
	case nil, string, bool, float32, float64,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64:
		return typed, nil
	case json.Number:
		if intValue, err := typed.Int64(); err == nil {
			return intValue, nil
		}
		floatValue, err := typed.Float64()
		if err != nil {
			return nil, err
		}
		return floatValue, nil
	case []string:
		result := make([]any, len(typed))
		for index, item := range typed {
			result[index] = item
		}
		return result, nil
	case []any:
		result := make([]any, len(typed))
		for index, item := range typed {
			normalized, err := jsonSchemaCompatibleValue(item)
			if err != nil {
				return nil, err
			}
			result[index] = normalized
		}
		return result, nil
	case map[string]string:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			result[key] = item
		}
		return result, nil
	case map[string][]string:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			normalized, err := jsonSchemaCompatibleValue(item)
			if err != nil {
				return nil, err
			}
			result[key] = normalized
		}
		return result, nil
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			normalized, err := jsonSchemaCompatibleValue(item)
			if err != nil {
				return nil, err
			}
			result[key] = normalized
		}
		return result, nil
	}
	reflected := reflect.ValueOf(value)
	if !reflected.IsValid() {
		return nil, nil
	}
	switch reflected.Kind() {
	case reflect.Pointer, reflect.Interface:
		if reflected.IsNil() {
			return nil, nil
		}
		return jsonSchemaCompatibleValue(reflected.Elem().Interface())
	case reflect.Slice, reflect.Array:
		result := make([]any, reflected.Len())
		for index := 0; index < reflected.Len(); index++ {
			normalized, err := jsonSchemaCompatibleValue(reflected.Index(index).Interface())
			if err != nil {
				return nil, err
			}
			result[index] = normalized
		}
		return result, nil
	case reflect.Map:
		if reflected.Type().Key().Kind() != reflect.String {
			return nil, fmt.Errorf("map key type %s is not JSON-compatible", reflected.Type().Key())
		}
		result := make(map[string]any, reflected.Len())
		for _, key := range reflected.MapKeys() {
			normalized, err := jsonSchemaCompatibleValue(reflected.MapIndex(key).Interface())
			if err != nil {
				return nil, err
			}
			result[key.String()] = normalized
		}
		return result, nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var result any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&result); err != nil {
		return nil, err
	}
	return jsonSchemaCompatibleValue(result)
}

func init() {
	defaultStepRegistry.AddStep(&SchemaValidator{}, map[string]map[string]any{
		".": {"x-category": types.FlowCategory, "x-order": []string{"validations", "outcome"}},
		"validations": {
			"schema_id": map[string]any{
				"x-type": "selectable",
				"x-props": map[string]any{
					"resource":      "schemas",
					"nameProperty":  "name",
					"valueProperty": "id",
				},
			},
		},
	})
}
