package inputs

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"unicode/utf8"

	jcache "github.com/nitsugaro/go-journey/cache"
	goutils "github.com/nitsugaro/go-utils/v2"
)

const (
	STRING_INPUT = "string"
	INT_INPUT    = "int"
	FLOAT_INPUT  = "float"
	BOOL_INPUT   = "bool"
	OBJECT_INPUT = "object"
)

type ValueInputConfig struct {
	ID         string   `json:"id" required:"true" description:"Attribute name used when saving the value."`
	ExternalID string   `json:"external_id" required:"true" description:"Stable identifier exposed to the client."`
	StepType   string   `json:"step_type,omitempty"`
	Label      string   `json:"label,omitempty"`
	Prompt     string   `json:"prompt,omitempty"`
	Type       string   `json:"type" enum:"string,int,float,bool,object" required:"true"`
	Required   bool     `json:"required" default:"true"`
	Pattern    string   `json:"pattern,omitempty"`
	Min        *float64 `json:"min,omitempty"`
	Max        *float64 `json:"max,omitempty"`
	UserName   bool     `json:"user_name,omitempty"`
}

type ValueInputOutput struct {
	Label    string   `json:"label,omitempty"`
	Prompt   string   `json:"prompt,omitempty"`
	Required bool     `json:"required"`
	Pattern  string   `json:"pattern,omitempty"`
	Min      *float64 `json:"min,omitempty"`
	Max      *float64 `json:"max,omitempty"`
}

func (cib *ClientInputsBuilder) AddValueInput(config *ValueInputConfig) error {
	if config == nil {
		return fmt.Errorf("input configuration is nil")
	}
	clientInput := &ClientInput{
		ExternalID: config.ExternalID,
		StepType:   config.StepType,
		Type:       config.Type,
		SendBack:   true,
		Output: ValueInputOutput{
			Label: config.Label, Prompt: config.Prompt, Required: config.Required,
			Pattern: config.Pattern, Min: config.Min, Max: config.Max,
		},
	}
	storedConfig := map[string]any{
		"external_id": config.ExternalID,
		"step_type":   config.StepType,
		"type":        config.Type,
		"required":    config.Required,
	}
	if config.Label != "" {
		storedConfig["label"] = config.Label
	}
	if config.Prompt != "" {
		storedConfig["prompt"] = config.Prompt
	}
	if config.Pattern != "" {
		storedConfig["pattern"] = config.Pattern
	}
	if config.Min != nil {
		storedConfig["min"] = config.Min
	}
	if config.Max != nil {
		storedConfig["max"] = config.Max
	}
	if config.UserName {
		storedConfig["user_name"] = true
	}
	if err := cib.Add(clientInput, storedConfig); err != nil {
		return err
	}
	return nil
}

func init() {
	for _, inputType := range []string{STRING_INPUT, INT_INPUT, FLOAT_INPUT, BOOL_INPUT, OBJECT_INPUT} {
		RegisterManagedValidator(inputType, validateValueInput)
	}
}

func validateValueInput(config goutils.TreeMapImpl, clientInput *ClientInput, cacheManager *jcache.Manager) *ClientError {
	var inputConfig ValueInputConfig
	if err := config.AsStruct(&inputConfig); err != nil {
		return &ClientError{Error: "cannot serialize input configuration: " + err.Error()}
	}
	if clientInput.Input == nil {
		if inputConfig.Required {
			return valueError(inputConfig.ExternalID, "value is required")
		}
		return nil
	}

	switch inputConfig.Type {
	case STRING_INPUT:
		value, ok := clientInput.Input.(string)
		if !ok {
			return valueError(inputConfig.ExternalID, "must be a string")
		}
		length := float64(utf8.RuneCountInString(value))
		if err := validateBounds(&inputConfig, length, "length"); err != nil {
			return err
		}
		if inputConfig.Pattern != "" {
			pattern, err := jcache.GetRegexp(cacheManager, inputConfig.Pattern)
			if err != nil {
				return valueError(inputConfig.ExternalID, "has an invalid configured pattern")
			}
			if !pattern.MatchString(value) {
				return valueError(inputConfig.ExternalID, "does not match the required pattern")
			}
		}
	case INT_INPUT:
		value, ok := numericValue(clientInput.Input)
		if !ok || math.Trunc(value) != value {
			return valueError(inputConfig.ExternalID, "must be an integer")
		}
		if err := validateBounds(&inputConfig, value, "value"); err != nil {
			return err
		}
	case FLOAT_INPUT:
		value, ok := numericValue(clientInput.Input)
		if !ok || math.IsInf(value, 0) || math.IsNaN(value) {
			return valueError(inputConfig.ExternalID, "must be a number")
		}
		if err := validateBounds(&inputConfig, value, "value"); err != nil {
			return err
		}
	case BOOL_INPUT:
		if _, ok := clientInput.Input.(bool); !ok {
			return valueError(inputConfig.ExternalID, "must be a boolean")
		}
	case OBJECT_INPUT:
		kind := reflect.Invalid
		if clientInput.Input != nil {
			kind = reflect.TypeOf(clientInput.Input).Kind()
		}
		if kind != reflect.Map && kind != reflect.Struct {
			return valueError(inputConfig.ExternalID, "must be an object")
		}
	default:
		return valueError(inputConfig.ExternalID, "uses an unsupported type")
	}
	return nil
}

func numericValue(value any) (float64, bool) {
	switch number := value.(type) {
	case int:
		return float64(number), true
	case int8:
		return float64(number), true
	case int16:
		return float64(number), true
	case int32:
		return float64(number), true
	case int64:
		return float64(number), true
	case uint:
		return float64(number), true
	case uint8:
		return float64(number), true
	case uint16:
		return float64(number), true
	case uint32:
		return float64(number), true
	case uint64:
		return float64(number), true
	case float32:
		return float64(number), true
	case float64:
		return number, true
	case json.Number:
		parsed, err := number.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

func validateBounds(config *ValueInputConfig, value float64, subject string) *ClientError {
	if config == nil {
		return valueError("", "input configuration is nil")
	}
	if config.Min != nil && value < *config.Min {
		return valueError(config.ExternalID, fmt.Sprintf("%s must be at least %v", subject, *config.Min))
	}
	if config.Max != nil && value > *config.Max {
		return valueError(config.ExternalID, fmt.Sprintf("%s must be at most %v", subject, *config.Max))
	}
	return nil
}

func valueError(externalID, message string) *ClientError {
	return &ClientError{Error: "invalid client input", Details: map[string]interface{}{"external_id": externalID, "message": message}}
}
