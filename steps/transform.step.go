package steps

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type TransformField struct {
	Target string `json:"target" required:"true" minLength:"1"`
	Value  any    `json:"value" required:"true"`
	Type   string `json:"type,omitempty" enum:"string,int,float,bool,object"`
}

type Transform struct {
	BasicStep

	_             struct{}         `description:"Builds or reshapes context data from static values and context placeholders."`
	TargetContext string           `json:"target_context" enum:"ctx,encCtx,closedCtx,tempCtx" default:"ctx"`
	Target        string           `json:"target,omitempty" description:"Optional parent path for transformed fields."`
	Fields        []TransformField `json:"fields" required:"true" minItems:"1"`
	Merge         bool             `json:"merge" default:"true"`
	Outcome       struct {
		True  string `json:"true" required:"true" format:"uuid"`
		Error string `json:"error" required:"true" format:"uuid"`
	} `json:"outcome" required:"true"`
}

func (*Transform) GetStepType() string { return types.TransformStep }

func (*Transform) VerifyConfig(stepName string, config goutils.TreeMapImpl) error {
	if strings.Contains(config.Get("fields").AsStringOr(""), "${") {
		return nil
	}
	fields, err := config.Get("fields").AsSlice()
	if err != nil || len(fields) == 0 {
		return types.StepInvalidConfig(stepName, "at least one transform field is required")
	}
	seen := map[string]struct{}{}
	for _, field := range fields {
		target := field.Get("target").AsStringOr("")
		if target == "" {
			return types.StepInvalidConfig(stepName, "every transform field requires a target")
		}
		if _, exists := seen[target]; exists {
			return types.StepInvalidConfig(stepName, "duplicate transform target: "+target)
		}
		seen[target] = struct{}{}
	}
	return nil
}

func (*Transform) Execute(transaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	contextMap := transaction.State.Get(config.Get("target_context").AsStringOr(types.CtxKey))
	if contextMap == nil {
		return "error", nil
	}
	fields, err := config.Get("fields").AsSlice()
	if err != nil {
		return "error", nil
	}
	target := config.Get("target").AsStringOr("")
	if target != "" && !config.Get("merge").AsBoolOr(true) {
		contextMap.Delete(target)
	}
	for _, field := range fields {
		value := field.Get("value").AsAnyOr(nil)
		value, err = transformValue(value, field.Get("type").AsStringOr(""))
		if err != nil {
			return "error", nil
		}
		path := field.Get("target").AsStringOr("")
		if target != "" {
			path = target + "." + path
		}
		contextMap.Set(path, value)
	}
	return "true", nil
}

func transformValue(value any, targetType string) (any, error) {
	if targetType == "" {
		return value, nil
	}
	switch targetType {
	case "string":
		return fmt.Sprint(value), nil
	case "int":
		number, err := strconv.ParseInt(fmt.Sprint(value), 10, 64)
		return number, err
	case "float":
		number, err := strconv.ParseFloat(fmt.Sprint(value), 64)
		return number, err
	case "bool":
		boolean, err := strconv.ParseBool(fmt.Sprint(value))
		return boolean, err
	case "object":
		if _, ok := value.(map[string]any); ok {
			return value, nil
		}
		var object map[string]any
		err := json.Unmarshal([]byte(fmt.Sprint(value)), &object)
		return object, err
	default:
		return nil, fmt.Errorf("unsupported transform type %q", targetType)
	}
}

func init() {
	defaultStepRegistry.AddStep(&Transform{}, map[string]map[string]any{
		".": {"x-category": types.ContextCategory, "x-order": []string{"target_context", "target", "fields", "merge", "outcome"}},
	})
}
