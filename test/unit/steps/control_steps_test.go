package steps_test

import (
	"bytes"
	"testing"

	journeysteps "github.com/nitsugaro/go-journey/steps"
	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

func TestConditionSupportsTypedPlaceholderOperations(t *testing.T) {
	tests := []struct {
		name      string
		value     any
		valueType string
		operation string
		compare   any
		want      string
	}{
		{name: "numeric minimum", value: 21, valueType: "int", operation: "min", compare: 18, want: "true"},
		{name: "unicode string length maximum", value: "ábc", valueType: "string", operation: "max", compare: 3, want: "true"},
		{name: "contains", value: "journey-engine", valueType: "string", operation: "contains", compare: "engine", want: "true"},
		{name: "object equality", value: map[string]any{"active": true}, valueType: "object", operation: "equal", compare: map[string]any{"active": true}, want: "true"},
		{name: "empty is not present", value: "", valueType: "string", operation: "present", want: "false"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			transaction := newStepTransaction()
			transaction.State.GetCtx().Set("candidate", test.value)
			transaction.State.GetCtx().Set("expected", test.compare)
			config := map[string]any{
				"value": "${ctx.candidate}", "type": test.valueType, "operation": test.operation,
				"outcome": map[string]any{"true": "yes", "false": "no"},
			}
			if test.operation != "present" && test.operation != "not_present" {
				config["compare_value"] = "${ctx.expected}"
			}
			step := &types.Step{StepType: types.ConditionStep, Config: config}
			if err := types.GenerateStepVariables(step, journeysteps.GetDefaultStepRegistry()); err != nil {
				t.Fatal(err)
			}
			outcome, err := types.ExecuteStepConfig(&journeysteps.Condition{}, transaction, step.Config)
			if err != nil || outcome != test.want {
				t.Fatalf("outcome=%q want=%q err=%v", outcome, test.want, err)
			}
		})
	}
}

func TestRandomValidatesTotalAndSelectsWeightedOutcome(t *testing.T) {
	step := &journeysteps.Random{Reader: bytes.NewReader([]byte{0x0b, 0x71, 0xb0})} // 75.0000
	config := map[string]any{
		"probabilities": map[string]any{"control": 25.0, "variant": 75.0},
		"outcome":       map[string]any{"control": "a", "variant": "b"},
	}
	if err := step.VerifyConfig("random", goutils.NewTreeMap(config)); err != nil {
		t.Fatal(err)
	}
	outcome, err := types.ExecuteStepConfig(step, newStepTransaction(), config)
	if err != nil || outcome != "variant" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
	config["probabilities"] = map[string]any{"control": 25.0, "variant": 74.0}
	if err := step.VerifyConfig("random", goutils.NewTreeMap(config)); err == nil {
		t.Fatal("probability total other than 100 was accepted")
	}
}

func TestRetryCountsPlaceholderLimitAndRoutes(t *testing.T) {
	transaction := newStepTransaction()
	transaction.State.GetCtx().Set("limit", "2")
	step := &types.Step{StepType: types.RetryStep, Config: map[string]any{
		"max_attempts": "${ctx.limit}", "context": "closedCtx", "counter": "login.attempts",
		"outcome": map[string]any{"retry": "again", "exhausted": "stop"},
	}}
	if err := types.GenerateStepVariables(step, journeysteps.GetDefaultStepRegistry()); err != nil {
		t.Fatal(err)
	}
	first, err := types.ExecuteStepConfig(&journeysteps.Retry{}, transaction, step.Config)
	if err != nil || first != "retry" {
		t.Fatalf("first outcome=%q err=%v", first, err)
	}
	second, err := types.ExecuteStepConfig(&journeysteps.Retry{}, transaction, step.Config)
	if err != nil || second != "exhausted" {
		t.Fatalf("second outcome=%q err=%v", second, err)
	}
	if attempts := transaction.State.GetClosedCtx().Get("login.attempts").AsIntOr(0); attempts != 2 {
		t.Fatalf("stored attempts=%d", attempts)
	}
}

func TestControlStepSchemasAcceptTheirPublicContracts(t *testing.T) {
	target := "00000000-0000-0000-0000-000000000001"
	configured := []*types.Step{
		{StepType: types.ConditionStep, Config: map[string]any{
			"value": "${ctx.value}", "type": "string", "operation": "equal", "compare_value": "yes",
			"outcome": map[string]any{"true": target, "false": target},
		}},
		{StepType: types.RandomStep, Config: map[string]any{
			"probabilities": map[string]any{"a": 40, "b": 60}, "outcome": map[string]any{"a": target, "b": target},
		}},
		{StepType: types.RetryStep, Config: map[string]any{
			"max_attempts": "${ctx.limit}", "context": "ctx", "counter": "attempts",
			"outcome": map[string]any{"retry": target, "exhausted": target},
		}},
	}
	registry := journeysteps.GetDefaultStepRegistry()
	for _, step := range configured {
		if err := types.GenerateStepVariables(step, registry); err != nil {
			t.Fatalf("%s vars: %v", step.StepType, err)
		}
		if err := registry.ValidateStep(step); err != nil {
			t.Fatalf("%s schema: %v", step.StepType, err)
		}
	}
}
