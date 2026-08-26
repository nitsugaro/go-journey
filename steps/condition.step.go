package steps

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type Condition struct {
	BasicStep

	_            struct{} `description:"Evaluates a typed value with a simple comparison and routes to true or false."`
	Value        any      `json:"value" required:"true"`
	Type         string   `json:"type" enum:"int,float,bool,string,object" required:"true"`
	Operation    string   `json:"operation" enum:"present,not_present,equal,not_equal,min,max,starts_with,ends_with,contains" required:"true"`
	CompareValue any      `json:"compare_value,omitempty" description:"Comparison value required by operations other than present and not_present."`
	Outcome      struct {
		True  string `json:"true" required:"true" format:"uuid"`
		False string `json:"false" required:"true" format:"uuid"`
	} `json:"outcome" required:"true"`
}

func (*Condition) GetStepType() string { return types.ConditionStep }

func (*Condition) VerifyConfig(stepName string, config goutils.TreeMapImpl) error {
	operation := config.Get("operation").AsStringOr("")
	valueType := config.Get("type").AsStringOr("")
	if strings.Contains(operation, "${") || strings.Contains(valueType, "${") {
		return nil
	}
	if operation != "present" && operation != "not_present" && !config.IsDefined("compare_value") {
		return types.StepInvalidConfig(stepName, "compare_value is required for operation "+operation)
	}
	if (operation == "min" || operation == "max") && valueType != "int" && valueType != "float" && valueType != "string" {
		return types.StepInvalidConfig(stepName, operation+" supports only numbers or string length")
	}
	if (operation == "starts_with" || operation == "ends_with" || operation == "contains") && valueType != "string" {
		return types.StepInvalidConfig(stepName, operation+" supports only strings")
	}
	return nil
}

func (*Condition) Execute(_ *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	value := config.Get("value").AsAnyOr(nil)
	operation := config.Get("operation").AsStringOr("")
	present := conditionPresent(value)
	if operation == "present" {
		return conditionOutcome(present), nil
	}
	if operation == "not_present" {
		return conditionOutcome(!present), nil
	}
	valueType := config.Get("type").AsStringOr("")
	left, err := conditionValue(value, valueType)
	if err != nil {
		return "", err
	}
	rightRaw := config.Get("compare_value").AsAnyOr(nil)
	var matched bool
	switch operation {
	case "equal", "not_equal":
		right, convertErr := conditionValue(rightRaw, valueType)
		if convertErr != nil {
			return "", convertErr
		}
		matched = reflect.DeepEqual(left, right)
		if operation == "not_equal" {
			matched = !matched
		}
	case "min", "max":
		actual, limit, compareErr := conditionMagnitude(left, rightRaw, valueType)
		if compareErr != nil {
			return "", compareErr
		}
		matched = actual >= limit
		if operation == "max" {
			matched = actual <= limit
		}
	case "starts_with":
		matched = strings.HasPrefix(left.(string), fmt.Sprint(rightRaw))
	case "ends_with":
		matched = strings.HasSuffix(left.(string), fmt.Sprint(rightRaw))
	case "contains":
		matched = strings.Contains(left.(string), fmt.Sprint(rightRaw))
	default:
		return "", fmt.Errorf("unsupported condition operation %q", operation)
	}
	return conditionOutcome(matched), nil
}

func conditionPresent(value any) bool {
	if value == nil {
		return false
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
		return rv.Len() != 0
	case reflect.Pointer, reflect.Interface:
		return !rv.IsNil()
	default:
		return true
	}
}

func conditionValue(value any, valueType string) (any, error) {
	switch valueType {
	case "string":
		return fmt.Sprint(value), nil
	case "bool":
		return strconv.ParseBool(fmt.Sprint(value))
	case "int":
		number, err := strconv.ParseFloat(fmt.Sprint(value), 64)
		if err != nil || math.Trunc(number) != number {
			return nil, fmt.Errorf("%v is not an integer", value)
		}
		return int64(number), nil
	case "float":
		return strconv.ParseFloat(fmt.Sprint(value), 64)
	case "object":
		data, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		var object map[string]any
		if err := json.Unmarshal(data, &object); err != nil {
			return nil, err
		}
		return object, nil
	default:
		return nil, fmt.Errorf("unsupported condition type %q", valueType)
	}
}

func conditionMagnitude(value, limit any, valueType string) (float64, float64, error) {
	threshold, err := strconv.ParseFloat(fmt.Sprint(limit), 64)
	if err != nil {
		return 0, 0, err
	}
	if valueType == "string" {
		return float64(len([]rune(value.(string)))), threshold, nil
	}
	number, err := strconv.ParseFloat(fmt.Sprint(value), 64)
	return number, threshold, err
}

func conditionOutcome(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func init() {
	defaultStepRegistry.AddStep(&Condition{}, map[string]map[string]any{
		".": {"x-category": types.FlowCategory, "x-order": []string{"value", "type", "operation", "compare_value", "outcome"}},
	})
}
