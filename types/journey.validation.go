package types

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	goutils "github.com/nitsugaro/go-utils/v2"
)

// PrepareJourneyConfiguration derives placeholder metadata and validates a
// journey before persistence. The journey is updated only after generation
// succeeds for each individual step configuration.
func PrepareJourneyConfiguration(journey *JourneyConfiguration, registry *Steps) error {
	if journey == nil {
		return fmt.Errorf("journey is nil")
	}
	if registry == nil {
		return fmt.Errorf("step registry is nil")
	}
	if journey.StartStepID == "" || journey.Steps[journey.StartStepID] == nil {
		return fmt.Errorf("start step %q does not exist", journey.StartStepID)
	}
	journey.JourneyType = NormalizeJourneyType(journey.JourneyType)
	for stepID, step := range journey.Steps {
		if step == nil {
			return fmt.Errorf("step %q is nil", stepID)
		}
		if err := GenerateStepVariables(step, registry); err != nil {
			return fmt.Errorf("step %q placeholders: %w", stepID, err)
		}
		if err := registry.ValidateStepForJourneyType(step, journey.JourneyType); err != nil {
			return fmt.Errorf("step %q: %w", stepID, err)
		}
		if err := validateNestedStepsForJourneyType(step, registry, journey.JourneyType); err != nil {
			return fmt.Errorf("step %q: %w", stepID, err)
		}
		config, ok := step.Config.(map[string]any)
		if !ok {
			continue
		}
		outcomes, _ := config["outcome"].(map[string]any)
		for outcome, rawTarget := range outcomes {
			target, ok := rawTarget.(string)
			if ok && target != "" && journey.Steps[target] == nil {
				return fmt.Errorf("step %q outcome %q targets missing step %q", stepID, outcome, target)
			}
		}
	}
	return nil
}

func NormalizeJourneyType(journeyType string) string {
	switch strings.TrimSpace(strings.ToLower(journeyType)) {
	case ResourceJourney, LegacyResourceJourney, LegacyProxyJourney:
		return ResourceJourney
	case WorkflowJourney, LegacyWorkflowJourney:
		return WorkflowJourney
	case AuthJourney, LegacyAuthJourney:
		return AuthJourney
	default:
		return AuthJourney
	}
}

func validateNestedStepsForJourneyType(step *Step, registry *Steps, journeyType string) error {
	if step == nil {
		return nil
	}
	config, ok := step.Config.(map[string]any)
	if !ok {
		return nil
	}
	rawSteps, ok := config["steps"].([]any)
	if !ok {
		return nil
	}
	for index, rawStep := range rawSteps {
		encoded, err := json.Marshal(rawStep)
		if err != nil {
			return fmt.Errorf("steps.%d: %w", index, err)
		}
		var child Step
		if err := json.Unmarshal(encoded, &child); err != nil {
			return fmt.Errorf("steps.%d: %w", index, err)
		}
		if err := registry.ValidateStepForJourneyType(&child, journeyType); err != nil {
			return fmt.Errorf("steps.%d: %w", index, err)
		}
		if err := validateNestedStepsForJourneyType(&child, registry, journeyType); err != nil {
			return fmt.Errorf("steps.%d: %w", index, err)
		}
	}
	return nil
}

// GenerateStepVariables scans configured string values and creates vars
// descriptors. Developers author only the actual property value containing
// ${prefix.path}; offsets and template paths are derived automatically.
func GenerateStepVariables(step *Step, registries ...*Steps) error {
	if step == nil || step.Config == nil {
		return nil
	}
	serialized, err := json.Marshal(step.Config)
	if err != nil {
		return err
	}
	var config map[string]any
	if err := json.Unmarshal(serialized, &config); err != nil {
		return err
	}
	composite := step.StepType == ChainStep || step.StepType == AsyncWaitStep || step.StepType == AsyncExecStep
	var registry *Steps
	if len(registries) != 0 {
		registry = registries[0]
	}
	if composite {
		if err := generateNestedStepVariables(config, registry); err != nil {
			return err
		}
	}
	existing, _ := config["vars"].(map[string]any)
	generated := map[string]any{}
	if err := discoverVariables(config, "", existing, generated, composite); err != nil {
		return err
	}
	var implementation IStep
	if registry != nil {
		implementation = registry.GetStep(step.StepType)
	}
	configMap := goutils.NewTreeMap(config)
	for path, rawVariable := range generated {
		variable, ok := rawVariable.(StepVariable)
		if !ok || variable.Type != "" {
			continue
		}
		variable.Type = inferStepVariableType(implementation, path)
		if variable.Type == "" && !isExactStepPlaceholder(configMap.Get(path).AsStringOr(""), &variable) {
			variable.Type = "string"
		}
		generated[path] = variable
	}
	if len(generated) == 0 {
		delete(config, "vars")
	} else {
		config["vars"] = generated
	}
	step.Config = config
	return nil
}

func isExactStepPlaceholder(source string, variable *StepVariable) bool {
	if variable == nil {
		return false
	}
	return len(variable.Placeholders) == 1 &&
		variable.Placeholders[0].StartsAt == 0 &&
		variable.Placeholders[0].EndsAt == len(source) &&
		"${"+variable.Placeholders[0].Template+"}" == source
}

func generateNestedStepVariables(config map[string]any, registry *Steps) error {
	rawSteps, exists := config["steps"]
	if !exists {
		return nil
	}
	steps, ok := rawSteps.([]any)
	if !ok {
		if value, isString := rawSteps.(string); isString {
			placeholders, err := discoverPlaceholders(value)
			if err != nil {
				return fmt.Errorf("steps: %w", err)
			}
			if len(placeholders) != 0 {
				return fmt.Errorf("steps cannot contain placeholders because it defines journey structure")
			}
		}
		return nil
	}
	for index, rawStep := range steps {
		encoded, err := json.Marshal(rawStep)
		if err != nil {
			return fmt.Errorf("steps.%d: %w", index, err)
		}
		var child Step
		if err := json.Unmarshal(encoded, &child); err != nil {
			return fmt.Errorf("steps.%d: %w", index, err)
		}
		if err := GenerateStepVariables(&child, registry); err != nil {
			return fmt.Errorf("steps.%d: %w", index, err)
		}
		encoded, err = json.Marshal(child)
		if err != nil {
			return fmt.Errorf("steps.%d: %w", index, err)
		}
		var normalized map[string]any
		if err := json.Unmarshal(encoded, &normalized); err != nil {
			return fmt.Errorf("steps.%d: %w", index, err)
		}
		steps[index] = normalized
	}
	config["steps"] = steps
	return nil
}

func inferStepVariableType(step IStep, path string) string {
	if step == nil {
		return ""
	}
	current := reflect.TypeOf(step)
	for _, part := range strings.Split(path, ".") {
		for current.Kind() == reflect.Pointer {
			current = current.Elem()
		}
		switch current.Kind() {
		case reflect.Struct:
			field, found := jsonStructField(current, part)
			if !found {
				return ""
			}
			current = field.Type
		case reflect.Slice, reflect.Array:
			if _, err := strconv.Atoi(part); err != nil {
				return ""
			}
			current = current.Elem()
		case reflect.Map:
			current = current.Elem()
		default:
			return ""
		}
	}
	for current.Kind() == reflect.Pointer {
		current = current.Elem()
	}
	switch current.Kind() {
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "bool"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "int"
	case reflect.Float32, reflect.Float64:
		return "float"
	case reflect.Map, reflect.Struct:
		return "object"
	case reflect.Slice, reflect.Array:
		return "array"
	default:
		return ""
	}
}

func jsonStructField(structType reflect.Type, property string) (reflect.StructField, bool) {
	for index := 0; index < structType.NumField(); index++ {
		field := structType.Field(index)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == property {
			return field, true
		}
		if field.Anonymous {
			nested := field.Type
			for nested.Kind() == reflect.Pointer {
				nested = nested.Elem()
			}
			if nested.Kind() == reflect.Struct {
				if match, found := jsonStructField(nested, property); found {
					return match, true
				}
			}
		}
	}
	return reflect.StructField{}, false
}

func discoverVariables(value any, path string, existing, generated map[string]any, composite bool) error {
	switch current := value.(type) {
	case map[string]any:
		for key, child := range current {
			if key == "vars" || (composite && path == "" && key == "steps") {
				continue
			}
			childPath := key
			if path != "" {
				childPath = path + "." + key
			}
			if err := discoverVariables(child, childPath, existing, generated, composite); err != nil {
				return err
			}
		}
	case []any:
		for index, child := range current {
			childPath := fmt.Sprintf("%s.%d", path, index)
			if err := discoverVariables(child, childPath, existing, generated, composite); err != nil {
				return err
			}
		}
	case string:
		placeholders, err := discoverPlaceholders(current)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if len(placeholders) == 0 {
			return nil
		}
		if staticPlaceholderPath(path) {
			return fmt.Errorf("%s cannot contain placeholders because it defines journey structure", path)
		}
		variable := StepVariable{Placeholders: placeholders}
		if raw, ok := existing[path]; ok {
			var previous StepVariable
			data, _ := json.Marshal(raw)
			if json.Unmarshal(data, &previous) == nil {
				variable.Type = previous.Type
			}
		}
		generated[path] = variable
	}
	return nil
}

func staticPlaceholderPath(path string) bool {
	parts := strings.Split(path, ".")
	if parts[0] == "outcome" {
		return true
	}
	for index, part := range parts {
		if part == "outcome" && index > 0 && parts[index-1] == "config" {
			return true
		}
		if (part == "step_type" || part == "name") && index >= 2 && parts[index-2] == "steps" {
			return true
		}
	}
	return false
}

func discoverPlaceholders(value string) ([]StepPlaceholder, error) {
	result := []StepPlaceholder{}
	for cursor := 0; cursor < len(value); {
		relativeStart := strings.Index(value[cursor:], "${")
		if relativeStart < 0 {
			break
		}
		start := cursor + relativeStart
		relativeEnd := strings.IndexByte(value[start+2:], '}')
		if relativeEnd < 0 {
			return nil, fmt.Errorf("placeholder at byte %d is not closed", start)
		}
		end := start + 2 + relativeEnd + 1
		path := value[start+2 : end-1]
		parts := strings.SplitN(path, ".", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid placeholder %q", value[start:end])
		}
		result = append(result, StepPlaceholder{Template: path, StartsAt: start, EndsAt: end})
		cursor = end
	}
	return result, nil
}
