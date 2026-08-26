package types_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/nitsugaro/go-journey/steps"
	"github.com/nitsugaro/go-journey/types"
)

func TestResolveStepConfigConvertsAllVariableTypes(t *testing.T) {
	state := types.NewJourneyState()
	state.GetCtx().Set("text", "Ada")
	state.GetCtx().Set("integer", 36)
	state.GetCtx().Set("decimal", 4.5)
	state.GetCtx().Set("enabled", true)
	state.GetCtx().Set("profile", map[string]any{"id": "1"})
	state.GetCtx().Set("roles", []any{"admin"})

	placeholder := func(template, variableType string) map[string]any {
		return map[string]any{
			"type": variableType,
			"placeholders": []any{map[string]any{
				"template": strings.TrimSuffix(strings.TrimPrefix(template, "${"), "}"), "starts_at": 0, "ends_at": len(template),
			}},
		}
	}
	resolved, err := types.ResolveStepConfig(map[string]any{
		"textValue": "${ctx.text}", "integerValue": "${ctx.integer}", "floatValue": "${ctx.decimal}",
		"boolValue": "${ctx.enabled}", "objectValue": "${ctx.profile}", "arrayValue": "${ctx.roles}",
		"vars": map[string]any{
			"textValue":    placeholder("${ctx.text}", "string"),
			"integerValue": placeholder("${ctx.integer}", "int"),
			"floatValue":   placeholder("${ctx.decimal}", "float"),
			"boolValue":    placeholder("${ctx.enabled}", "bool"),
			"objectValue":  placeholder("${ctx.profile}", "object"),
			"arrayValue":   placeholder("${ctx.roles}", "array"),
		}}, state)
	if err != nil {
		t.Fatal(err)
	}
	if got := resolved.Get("textValue").AsStringOr(""); got != "Ada" {
		t.Fatalf("string = %q", got)
	}
	if got := resolved.Get("integerValue").AsIntOr(0); got != 36 {
		t.Fatalf("integer = %d", got)
	}
	if got := resolved.Get("floatValue").AsFloatOr(0); got != 4.5 {
		t.Fatalf("float = %v", got)
	}
	if got := resolved.Get("boolValue").AsBoolOr(false); !got {
		t.Fatal("bool was not preserved")
	}
	if got := resolved.Get("objectValue.id").AsStringOr(""); got != "1" {
		t.Fatalf("object id = %q", got)
	}
	if values, err := resolved.Get("arrayValue").AsSlice(); err != nil || len(values) != 1 {
		t.Fatalf("array = %v, %v", values, err)
	}
}

func TestAllRuntimeStepPropertyTypesAcceptAndResolvePlaceholders(t *testing.T) {
	step := &types.Step{
		Name: "Dynamic HTTP", StepType: types.HttpRequestStep,
		Config: map[string]any{
			"uri": "${ctx.uri}", "method": "${ctx.method}", "content_type": "JSON",
			"headers": "${ctx.headers}",
			"body":    "${ctx.body}", "response_output": "${ctx.output}",
			"parse_json": "${ctx.parse}", "re_execute_on_chain_step": "${ctx.reexecute}",
			"outcome": map[string]any{"true": "00000000-0000-0000-0000-000000000001"},
		},
	}
	if err := types.GenerateStepVariables(step); err != nil {
		t.Fatal(err)
	}
	if err := steps.GetDefaultStepRegistry().ValidateStep(step); err != nil {
		t.Fatalf("placeholder-bearing typed properties failed schema validation: %v", err)
	}
	state := types.NewJourneyState()
	state.GetCtx().Set("uri", "https://example.test")
	state.GetCtx().Set("method", "POST")
	state.GetCtx().Set("headers", map[string]any{"Authorization": "Bearer token"})
	state.GetCtx().Set("body", "payload")
	state.GetCtx().Set("output", "ctx.response")
	state.GetCtx().Set("parse", true)
	state.GetCtx().Set("reexecute", false)
	resolved, err := types.ResolveStepConfig(step.Config, state)
	if err != nil {
		t.Fatal(err)
	}
	if !resolved.Get("parse_json").AsBoolOr(false) || resolved.Get("re_execute_on_chain_step").AsBoolOr(true) {
		t.Fatal("boolean placeholders did not retain their native types")
	}
	if resolved.Get("headers.Authorization").AsStringOr("") != "Bearer token" {
		t.Fatal("object placeholder did not retain its native type")
	}
}

func TestStructuralStepPropertiesRejectPlaceholders(t *testing.T) {
	for _, step := range []*types.Step{
		{Config: map[string]any{"outcome": map[string]any{"true": "${ctx.next}"}}},
		{StepType: types.AsyncExecStep, Config: map[string]any{"steps": "${ctx.steps}"}},
	} {
		if err := types.GenerateStepVariables(step); err == nil {
			t.Fatalf("structural placeholder was accepted: %v", step.Config)
		}
	}
}

func TestCustomStructuredPropertyNamedStepsStillGeneratesVariables(t *testing.T) {
	step := &types.Step{StepType: "Custom", Config: map[string]any{
		"steps": []any{map[string]any{"value": "${ctx.value}"}},
	}}
	if err := types.GenerateStepVariables(step); err != nil {
		t.Fatal(err)
	}
	variables := step.Config.(map[string]any)["vars"].(map[string]any)
	if _, found := variables["steps.0.value"]; !found {
		t.Fatal("custom structured steps property was skipped")
	}
}

func TestMixedUnknownPlaceholderIsExplicitlyGeneratedAsString(t *testing.T) {
	step := &types.Step{StepType: types.SuccessStep, Config: map[string]any{
		"data": map[string]any{"key": []any{map[string]any{"item": "hi-${ctx.hola}"}}},
	}}
	if err := types.GenerateStepVariables(step, steps.GetDefaultStepRegistry()); err != nil {
		t.Fatal(err)
	}
	variable := step.Config.(map[string]any)["vars"].(map[string]any)["data.key.0.item"].(types.StepVariable)
	if variable.Type != "string" {
		t.Fatalf("mixed placeholder type=%q, want string", variable.Type)
	}
}

func TestNestedNumericAndBooleanStepPropertiesSupportPlaceholders(t *testing.T) {
	step := &types.Step{
		Name: "Dynamic form", StepType: types.FormStep,
		Config: map[string]any{
			"inputs": []any{map[string]any{
				"id": "age", "external_id": "profile.age", "type": "int",
				"min": "${ctx.minimumAge}", "required": "${ctx.ageRequired}",
			}},
			"context": "ctx",
			"outcome": map[string]any{"true": "00000000-0000-0000-0000-000000000001"},
		},
	}
	if err := types.GenerateStepVariables(step); err != nil {
		t.Fatal(err)
	}
	if err := steps.GetDefaultStepRegistry().ValidateStep(step); err != nil {
		t.Fatalf("nested typed placeholders failed schema validation: %v", err)
	}
	state := types.NewJourneyState()
	state.GetCtx().Set("minimumAge", 18)
	state.GetCtx().Set("ageRequired", true)
	resolved, err := types.ResolveStepConfig(step.Config, state)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Get("inputs.0.min").AsIntOr(0) != 18 || !resolved.Get("inputs.0.required").AsBoolOr(false) {
		t.Fatal("nested numeric or boolean placeholder lost its native type")
	}
}

func TestGeneratorInfersTypesFromRegisteredStepStructure(t *testing.T) {
	step := &types.Step{StepType: types.HttpRequestStep, Config: map[string]any{
		"uri": "${ctx.uri}", "method": "GET", "content_type": "JSON",
		"headers":         map[string]any{"X-Trace": "${ctx.trace}"},
		"response_output": "ctx.response",
		"parse_json":      "${ctx.parse}", "re_execute_on_chain_step": "${ctx.reexecute}",
		"outcome": map[string]any{"true": "00000000-0000-0000-0000-000000000001"},
	}}
	if err := types.GenerateStepVariables(step, steps.GetDefaultStepRegistry()); err != nil {
		t.Fatal(err)
	}
	variables := step.Config.(map[string]any)["vars"].(map[string]any)
	want := map[string]string{
		"uri": "string", "headers.X-Trace": "string", "parse_json": "bool", "re_execute_on_chain_step": "bool",
	}
	for path, expected := range want {
		variable, ok := variables[path].(types.StepVariable)
		if !ok || variable.Type != expected {
			t.Fatalf("vars.%s type=%q, want %q", path, variable.Type, expected)
		}
	}
	state := types.NewJourneyState()
	state.GetCtx().Set("uri", "https://example.test")
	state.GetCtx().Set("trace", "trace-id")
	state.GetCtx().Set("parse", "true")
	state.GetCtx().Set("reexecute", "false")
	resolved, err := types.ResolveStepConfig(step.Config, state)
	if err != nil {
		t.Fatal(err)
	}
	if !resolved.Get("parse_json").AsBoolOr(false) || resolved.Get("re_execute_on_chain_step").AsBoolOr(true) {
		t.Fatal("inferred boolean types were not applied during resolution")
	}
}

func TestResolveStepConfigCustomResolverAndErrors(t *testing.T) {
	template := "${secrets.api.key}"
	raw := map[string]any{
		"value": template,
		"vars": map[string]any{"value": map[string]any{
			"type": "string", "placeholders": []any{map[string]any{
				"template": "secrets.api.key", "starts_at": 0, "ends_at": len(template),
			}},
		}},
	}
	calledWith := ""
	resolved, err := types.ResolveStepConfig(raw, types.NewJourneyState(), map[string]types.PlaceholderResolver{
		"secrets": func(path string) (any, error) {
			calledWith = path
			return "token", nil
		},
	})
	if err != nil || calledWith != "api.key" || resolved.Get("value").AsStringOr("") != "token" {
		t.Fatalf("value=%q path=%q err=%v", resolved.Get("value").AsStringOr(""), calledWith, err)
	}
	if _, err := types.ResolveStepConfig(raw, types.NewJourneyState()); err == nil {
		t.Fatal("missing custom resolver was accepted")
	}
	resolverErr := errors.New("vault unavailable")
	if _, err := types.ResolveStepConfig(raw, types.NewJourneyState(), map[string]types.PlaceholderResolver{
		"secrets": func(string) (any, error) { return nil, resolverErr },
	}); !errors.Is(err, resolverErr) {
		t.Fatal("custom resolver error was not returned")
	}
}
