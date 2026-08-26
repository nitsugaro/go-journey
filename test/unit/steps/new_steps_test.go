package steps_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/nitsugaro/go-journey/env"
	"github.com/nitsugaro/go-journey/inputs"
	journeysteps "github.com/nitsugaro/go-journey/steps"
	"github.com/nitsugaro/go-journey/types"
	"github.com/nitsugaro/go-nstore"
	goutils "github.com/nitsugaro/go-utils/v2"
)

func newStepTransaction() *types.JourneyTransaction {
	state := types.NewJourneyState()
	return &types.JourneyTransaction{
		Journey: &types.JourneyConfiguration{
			Metadata: &nstore.Metadata{ID: "journey"},
			Steps: map[string]*types.Step{
				"step": {Name: "Step", StepType: "Test"},
			},
		},
		CurrentStepID:       "step",
		State:               state,
		Response:            types.NewMemoryResponse(),
		Steps:               journeysteps.GetDefaultStepRegistry(),
		ClientInputsBuilder: inputs.NewClientInputBuilder(nil, state.GetCtx()),
	}
}

func TestEndPublishesOnlyConfiguredResult(t *testing.T) {
	withoutResult := newStepTransaction()
	if _, err := types.ExecuteStepConfig(&journeysteps.End{}, withoutResult, map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if withoutResult.State.HasResult() {
		t.Fatal("End without result unexpectedly published a value")
	}

	withResult := newStepTransaction()
	if _, err := types.ExecuteStepConfig(&journeysteps.End{}, withResult, map[string]any{"result": []any{"one", float64(2)}}); err != nil {
		t.Fatal(err)
	}
	if !withResult.State.HasResult() {
		t.Fatal("End with result did not publish its value")
	}
	result, ok := withResult.State.GetResult().([]any)
	if !ok || len(result) != 2 {
		t.Fatalf("unexpected End result %#v", withResult.State.GetResult())
	}

	withNull := newStepTransaction()
	if _, err := types.ExecuteStepConfig(&journeysteps.End{}, withNull, map[string]any{"result": nil}); err != nil {
		t.Fatal(err)
	}
	if !withNull.State.HasResult() || withNull.State.GetResult() != nil {
		t.Fatalf("explicit null was not preserved: has=%v value=%#v", withNull.State.HasResult(), withNull.State.GetResult())
	}
}

func TestTransformPreservesAndConvertsPlaceholderValues(t *testing.T) {
	transaction := newStepTransaction()
	transaction.State.GetCtx().Set("source.age", float64(36))
	transaction.State.GetCtx().Set("source.enabled", true)
	raw := map[string]any{
		"target_context": "ctx", "target": "result", "merge": false,
		"fields": []any{
			map[string]any{"target": "age", "value": "${ctx.source.age}", "type": "int"},
			map[string]any{"target": "enabled", "value": "${ctx.source.enabled}"},
			map[string]any{"target": "label", "value": "age=${ctx.source.age}"},
		},
		"vars": map[string]any{
			"fields.0.value": map[string]any{"type": "int", "placeholders": []any{map[string]any{"template": "ctx.source.age", "starts_at": 0, "ends_at": 17}}},
			"fields.1.value": map[string]any{"type": "bool", "placeholders": []any{map[string]any{"template": "ctx.source.enabled", "starts_at": 0, "ends_at": 21}}},
			"fields.2.value": map[string]any{"type": "string", "placeholders": []any{map[string]any{"template": "ctx.source.age", "starts_at": 4, "ends_at": 21}}},
		},
	}
	config, err := types.ResolveStepConfig(raw, transaction.State)
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := (&journeysteps.Transform{}).Execute(transaction, config)
	if err != nil || outcome != "true" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
	if transaction.State.GetCtx().Get("result.age").AsIntOr(0) != 36 || !transaction.State.GetCtx().Get("result.enabled").AsBoolOr(false) {
		t.Fatal("typed placeholder values were not stored")
	}
	if got := transaction.State.GetCtx().Get("result.label").AsStringOr(""); got != "age=36" {
		t.Fatalf("embedded placeholder = %q", got)
	}
}

func TestChainResolvesEachStructuredChildImmediatelyBeforeExecution(t *testing.T) {
	chain := &types.Step{StepType: types.ChainStep, Config: map[string]any{
		"steps": []any{
			map[string]any{"name": "produce", "step_type": types.TransformStep, "config": map[string]any{
				"target_context": "ctx", "fields": []any{map[string]any{"target": "produced", "value": "Ada"}},
			}},
			map[string]any{"name": "consume", "step_type": types.TransformStep, "config": map[string]any{
				"target_context": "ctx", "fields": []any{map[string]any{"target": "consumed", "value": "${ctx.produced}"}},
			}},
		},
		"outcome": map[string]any{"true": "next"},
	}}
	if err := types.GenerateStepVariables(chain); err != nil {
		t.Fatal(err)
	}
	root := chain.Config.(map[string]any)
	if _, found := root["vars"]; found {
		t.Fatal("child placeholders were incorrectly registered in parent vars")
	}
	children := root["steps"].([]any)
	secondConfig := children[1].(map[string]any)["config"].(map[string]any)
	if _, found := secondConfig["vars"]; !found {
		t.Fatal("child placeholder metadata was not kept with child config")
	}

	transaction := newStepTransaction()
	outcome, err := types.ExecuteStepConfig(&journeysteps.Chain{}, transaction, chain.Config)
	if err != nil {
		t.Fatal(err)
	}
	if outcome != "true" || transaction.State.GetCtx().Get("consumed").AsStringOr("") != "Ada" {
		t.Fatalf("outcome=%q consumed=%q", outcome, transaction.State.GetCtx().Get("consumed").AsStringOr(""))
	}
}

func TestSuccessPreservesResolvedStructuredData(t *testing.T) {
	transaction := newStepTransaction()
	transaction.State.GetCtx().Set("result", map[string]any{"name": "Ada", "roles": []any{"admin"}})
	step := &types.Step{Config: map[string]any{"data": "${ctx.result}"}}
	if err := types.GenerateStepVariables(step); err != nil {
		t.Fatal(err)
	}
	if _, err := types.ExecuteStepConfig(&journeysteps.Success{}, transaction, step.Config); err != nil {
		t.Fatal(err)
	}
	response, ok := transaction.Response.(*types.MemoryResponse)
	if !ok || response.StatusCode != http.StatusOK || !response.BodySet {
		t.Fatalf("success did not write terminal response: %#v", transaction.Response)
	}
	var body map[string]any
	if err := json.Unmarshal(response.BodyBytesValue, &body); err != nil {
		t.Fatal(err)
	}
	stored := goutils.NewTreeMap(body).Get("data")
	if stored.Get("name").AsStringOr("") != "Ada" || stored.Get("roles.0").AsStringOr("") != "admin" {
		t.Fatalf("structured terminal data was not preserved: %v", stored.AsAnyOr(nil))
	}
}

func TestTransformRoutesConversionFailure(t *testing.T) {
	transaction := newStepTransaction()
	config := goutils.NewTreeMap(map[string]any{
		"fields": []any{map[string]any{"target": "age", "value": "not-a-number", "type": "int"}},
	})
	outcome, err := (&journeysteps.Transform{}).Execute(transaction, config)
	if err != nil || outcome != "error" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
}

func TestAssertRecordsBusinessFailures(t *testing.T) {
	transaction := newStepTransaction()
	transaction.State.GetCtx().Set("age", 16)
	config := goutils.NewTreeMap(map[string]any{
		"mode": "all", "errors_context": "ctx", "errors_target": "problems",
		"rules": []any{
			map[string]any{"id": "adult", "expression": `getCtxProperty("age", 0) >= 18`, "message": "Must be an adult"},
			map[string]any{"id": "positive", "expression": `getCtxProperty("age", 0) > 0`},
		},
	})
	outcome, err := (&journeysteps.Assert{}).Execute(transaction, config)
	if err != nil || outcome != "invalid" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
	failures, err := transaction.State.GetCtx().Get("problems").AsSlice()
	if err != nil || len(failures) != 1 || failures[0].Get("id").AsStringOr("") != "adult" {
		t.Fatalf("failures=%v err=%v", failures, err)
	}
}

func TestWaitUntilUsesNumericPlaceholderAndSuspends(t *testing.T) {
	transaction := newStepTransaction()
	transaction.State.Exp = time.Minute
	transaction.State.GetCtx().Set("limit", 5)
	target := time.Now().Add(2 * time.Second).UTC().Format(time.RFC3339)
	template := "${ctx.limit}"
	raw := map[string]any{
		"timestamp": target, "timezone": "UTC", "max_wait_seconds": template,
		"vars": map[string]any{"max_wait_seconds": map[string]any{
			"type":         "int",
			"placeholders": []any{map[string]any{"template": "ctx.limit", "starts_at": 0, "ends_at": len(template)}},
		}},
	}
	config, err := types.ResolveStepConfig(raw, transaction.State)
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := (&journeysteps.WaitUntil{}).Execute(transaction, config)
	if err != nil || outcome != "resumed" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
	if transaction.ClientInputsBuilder.IsNewEmpty() {
		t.Fatal("future timestamp did not suspend the journey")
	}
	if transaction.State.GetTempCtx().Get(env.GetSuspendJourneyKey()+".exp").AsIntOr(0) < 60 {
		t.Fatal("suspended state TTL does not cover the normal journey lifetime")
	}
}

func TestWaitUntilRoutesInvalidAndLimitExceeded(t *testing.T) {
	tests := []struct {
		name, timestamp, max, expected string
	}{
		{name: "invalid", timestamp: "tomorrow-ish", max: "10", expected: "invalid"},
		{name: "limit", timestamp: time.Now().Add(time.Hour).UTC().Format(time.RFC3339), max: "10", expected: "limit_exceeded"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			transaction := newStepTransaction()
			config := goutils.NewTreeMap(map[string]any{"timestamp": test.timestamp, "timezone": "UTC", "max_wait_seconds": test.max})
			outcome, err := (&journeysteps.WaitUntil{}).Execute(transaction, config)
			if err != nil || outcome != test.expected {
				t.Fatalf("outcome=%q err=%v", outcome, err)
			}
		})
	}
}
