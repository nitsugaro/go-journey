package steps_test

import (
	"testing"

	journeysteps "github.com/nitsugaro/go-journey/steps"
	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

func TestSetCtxPropertyCanReadRESTRequest(t *testing.T) {
	transaction := newStepTransaction()
	transaction.Request = &types.JourneyRequest{Method: "PATCH", Path: "/customers/42"}
	step := journeysteps.GetDefaultStepRegistry().GetStep(types.SetCtxPropertyStep)
	if step == nil {
		t.Fatal("SetCtxProperty is not registered")
	}
	outcome, err := step.Execute(transaction, goutils.NewTreeMap(map[string]any{
		"type": "ctx", "key": "requestMethod", "expression": "request.Method",
	}))
	if err != nil || outcome != "true" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
	if got := transaction.State.GetCtx().Get("requestMethod").AsStringOr(""); got != "PATCH" {
		t.Fatalf("request method = %q", got)
	}
}

func TestIfExpressionCanReadRequestBinding(t *testing.T) {
	transaction := newStepTransaction()
	transaction.Request = &types.JourneyRequest{Method: "POST", Path: "/login"}
	outcome, err := (&journeysteps.IfExpression{}).Execute(transaction, goutils.NewTreeMap(map[string]any{
		"expression": `request.Method == "POST" && request.Path == "/login"`,
	}))
	if err != nil || outcome != "true" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
}

func TestIfExpressionCanSafelyReadRequestQueryAndHeaders(t *testing.T) {
	transaction := newStepTransaction()
	transaction.Request = &types.JourneyRequest{
		QueryParameters: map[string][]string{"name": {"true"}, "empty": {}},
		Headers:         map[string][]string{"Content-Type": {"application/json"}},
	}
	outcome, err := (&journeysteps.IfExpression{}).Execute(transaction, goutils.NewTreeMap(map[string]any{
		"expression": `requestQuery.First("name", "false") == "true" &&
requestQuery.First("missing", "fallback") == "fallback" &&
requestQuery.First("empty", "fallback") == "fallback" &&
requestHeader.First("content-type", "") == "application/json" &&
requestHeader.Has("CONTENT-TYPE")`,
	}))
	if err != nil || outcome != "true" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
}

func TestSwitchExpressionCanReadRequestBinding(t *testing.T) {
	transaction := newStepTransaction()
	transaction.Request = &types.JourneyRequest{Method: "GET", Path: "/health"}
	outcome, err := (&journeysteps.SwitchExpression{}).Execute(transaction, goutils.NewTreeMap(map[string]any{
		"expressions": []any{
			map[string]any{"name": "mutating", "exp": `request.Method == "POST"`},
			map[string]any{"name": "health", "exp": `request.Path == "/health"`},
		},
	}))
	if err != nil || outcome != "health" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
}

func TestAssertCanReadRequestBinding(t *testing.T) {
	transaction := newStepTransaction()
	transaction.Request = &types.JourneyRequest{Method: "PATCH", Path: "/customers/42"}
	outcome, err := (&journeysteps.Assert{}).Execute(transaction, goutils.NewTreeMap(map[string]any{
		"mode":           "all",
		"errors_context": "ctx",
		"errors_target":  "errors",
		"rules": []any{
			map[string]any{"id": "method", "expression": `request.Method == "PATCH"`, "message": "invalid method"},
			map[string]any{"id": "path", "expression": `request.Path == "/customers/42"`, "message": "invalid path"},
		},
	}))
	if err != nil || outcome != "valid" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
}
