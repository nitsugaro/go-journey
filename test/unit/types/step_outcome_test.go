package types_test

import (
	"testing"

	"github.com/nitsugaro/go-journey/types"
)

func TestStepOutcomeLookupIsCaseInsensitive(t *testing.T) {
	step := &types.Step{
		StepType: types.ScriptStep,
		Config: map[string]any{
			"outcome": map[string]any{"SuCcEsS": "next-step"},
		},
	}
	target, err := step.GetOutcomeID(" success ")
	if err != nil || target != "next-step" {
		t.Fatalf("target=%q err=%v", target, err)
	}
}

func TestNonScriptOutcomeLookupRemainsCaseSensitive(t *testing.T) {
	step := &types.Step{
		StepType: "Condition",
		Config: map[string]any{
			"outcome": map[string]any{"Success": "next-step"},
		},
	}
	if _, err := step.GetOutcomeID("success"); err == nil {
		t.Fatal("non-Script outcome lookup unexpectedly ignored case")
	}
}
