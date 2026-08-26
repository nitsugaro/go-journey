package steps

import (
	"fmt"
	"strings"

	"github.com/nitsugaro/go-journey/env"
	"github.com/nitsugaro/go-journey/inputs"
	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type Form struct {
	BasicStep

	_       struct{}                  `description:"Collects one or more typed client inputs and saves them into context."`
	Inputs  []inputs.ValueInputConfig `json:"inputs" required:"true" minItems:"1"`
	Context string                    `json:"context" enum:"ctx,encCtx,closedCtx,tempCtx" default:"tempCtx"`
	Object  string                    `json:"object,omitempty" description:"Optional parent object used to group all saved attributes."`
	Outcome struct {
		True string `json:"true" format:"uuid"`
	} `json:"outcome" required:"true"`
}

func (*Form) GetStepType() string { return types.FormStep }

func (*Form) VerifyConfig(stepName string, config goutils.TreeMapImpl) error {
	if strings.Contains(config.Get("inputs").AsStringOr(""), "${") {
		return nil
	}
	fields, err := config.Get("inputs").AsSlice()
	if err != nil || len(fields) == 0 {
		return types.StepInvalidConfig(stepName, "at least one input is required")
	}
	ids := map[string]struct{}{}
	externalIDs := map[string]struct{}{}
	for _, field := range fields {
		id := field.Get("id").AsStringOr("")
		externalID := field.Get("external_id").AsStringOr("")
		inputType := field.Get("type").AsStringOr("")
		dynamicType := strings.Contains(inputType, "${")
		if id == "" || externalID == "" {
			return types.StepInvalidConfig(stepName, "every input requires id and external_id")
		}
		if _, exists := ids[id]; exists {
			return types.StepInvalidConfig(stepName, "duplicate context id: "+id)
		}
		if _, exists := externalIDs[externalID]; exists {
			return types.StepInvalidConfig(stepName, "duplicate external_id: "+externalID)
		}
		ids[id] = struct{}{}
		externalIDs[externalID] = struct{}{}
		if !dynamicType && !isValueInputType(inputType) {
			return types.StepInvalidConfig(stepName, "unsupported input type: "+inputType)
		}
		if !dynamicType && field.Get("pattern").AsStringOr("") != "" && inputType != inputs.STRING_INPUT {
			return types.StepInvalidConfig(stepName, "pattern only applies to string input: "+externalID)
		}
		hasBounds := field.IsDefined("min") || field.IsDefined("max")
		if !dynamicType && hasBounds && inputType != inputs.STRING_INPUT && inputType != inputs.INT_INPUT && inputType != inputs.FLOAT_INPUT {
			return types.StepInvalidConfig(stepName, "min and max only apply to string or numeric input: "+externalID)
		}
		if !dynamicType && field.Get("user_name").AsBoolOr(false) && inputType != inputs.STRING_INPUT {
			return types.StepInvalidConfig(stepName, "user_name input must be a string")
		}
		if field.IsDefined("min") && field.IsDefined("max") &&
			!strings.Contains(field.Get("min").AsStringOr(""), "${") &&
			!strings.Contains(field.Get("max").AsStringOr(""), "${") &&
			field.Get("min").AsFloatOr(0) > field.Get("max").AsFloatOr(0) {
			return types.StepInvalidConfig(stepName, "input min cannot exceed max: "+externalID)
		}
	}
	return nil
}

func (*Form) Execute(transaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	fields, err := config.Get("inputs").AsSlice()
	if err != nil || len(fields) == 0 {
		return "", types.ErrInvalidStepConfig
	}

	type acceptedValue struct {
		config inputs.ValueInputConfig
		value  any
	}
	accepted := make([]acceptedValue, 0, len(fields))
	requested := false
	for _, field := range fields {
		fieldConfig, configErr := valueInputConfig(field)
		if configErr != nil {
			return "", configErr
		}
		clientInput := transaction.ClientInputsBuilder.GetFirstFromExternalID(fieldConfig.ExternalID)
		if clientInput == nil {
			if transaction.ClientInputsBuilder.WasRequested(fieldConfig.ExternalID, transaction.CurrentStepID) && !fieldConfig.Required {
				continue
			}
			if err := transaction.ClientInputsBuilder.AddValueInput(&fieldConfig); err != nil {
				return "", err
			}
			requested = true
			continue
		}
		if clientInput.Input == nil && !fieldConfig.Required {
			continue
		}
		accepted = append(accepted, acceptedValue{config: fieldConfig, value: clientInput.Input})
	}
	if requested {
		return "", nil
	}

	contextName := config.Get("context").AsStringOr(types.CtxKey)
	contextMap := transaction.State.Get(contextName)
	if contextMap == nil {
		return "", types.StepInvalidConfig(transaction.CurrentStepID, "invalid context: "+contextName)
	}
	object := config.Get("object").AsStringOr("")
	for _, item := range accepted {
		key := item.config.ID
		if object != "" {
			key = object + "." + key
		}
		contextMap.Set(key, item.value)
		if item.config.UserName {
			loginHint, ok := item.value.(string)
			if !ok {
				return "", fmt.Errorf("user_name input %s is not a string", item.config.ExternalID)
			}
			transaction.State.GetClosedCtx().Set(env.GetContextKey("user_name"), loginHint)
		}
	}
	return "true", nil
}

func valueInputConfig(field goutils.TreeMapImpl) (inputs.ValueInputConfig, error) {
	var config inputs.ValueInputConfig
	if err := field.AsStruct(&config); err != nil {
		return config, err
	}
	config.Required = field.Get("required").AsBoolOr(true)
	config.StepType = types.FormStep
	return config, nil
}

func isValueInputType(inputType string) bool {
	switch inputType {
	case inputs.STRING_INPUT, inputs.INT_INPUT, inputs.FLOAT_INPUT, inputs.BOOL_INPUT, inputs.OBJECT_INPUT:
		return true
	default:
		return false
	}
}

func init() {
	defaultStepRegistry.AddStep(&Form{}, map[string]map[string]any{
		".": {"x-category": types.FlowCategory, "x-order": []string{"inputs", "context", "object", "outcome"}},
	})
}
