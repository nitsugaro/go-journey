package steps_test

import (
	"testing"

	"github.com/nitsugaro/go-journey/env"
	journeysteps "github.com/nitsugaro/go-journey/steps"
	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

func TestSubJourneyPropsUseClosedContextStackByDefault(t *testing.T) {
	transaction := newStepTransaction()
	propsKey := env.GetContextKey("props")
	stackKey := env.GetContextKey("props_stack")
	transaction.State.GetClosedCtx().Set(propsKey, map[string]any{"parent": "value"})
	config := goutils.NewTreeMap(map[string]any{
		"journey_id": "child",
		"props":      map[string]any{"account_id": "acc-1", "attempts": 2},
	})

	outcome, err := (&journeysteps.SubJourney{}).Execute(transaction, config)
	if err != nil || outcome != "" {
		t.Fatalf("start outcome=%q err=%v", outcome, err)
	}
	if got := transaction.State.GetClosedCtx().Get(propsKey + ".account_id").AsStringOr(""); got != "acc-1" {
		t.Fatalf("active props account_id=%q", got)
	}
	if stack := transaction.State.GetClosedCtx().Get(stackKey).AsAnySlice(); len(stack) != 1 {
		t.Fatalf("stack length after start=%d", len(stack))
	}

	transaction.State.GetClosedCtx().Set(transaction.CurrentStepID, true)
	outcome, err = (&journeysteps.SubJourney{}).Execute(transaction, config)
	if err != nil || outcome != "true" {
		t.Fatalf("return outcome=%q err=%v", outcome, err)
	}
	if got := transaction.State.GetClosedCtx().Get(propsKey + ".parent").AsStringOr(""); got != "value" {
		t.Fatalf("restored parent props=%q", got)
	}
	if transaction.State.GetClosedCtx().IsDefined(stackKey) {
		t.Fatal("completed sub-journey props frame was not removed from stack")
	}
}

func TestSubJourneyPropsCanUseEncryptedContext(t *testing.T) {
	transaction := newStepTransaction()
	propsKey := env.GetContextKey("props")
	stackKey := env.GetContextKey("props_stack")
	config := goutils.NewTreeMap(map[string]any{
		"journey_id":    "child",
		"props_context": types.EncCtxKey,
		"props":         map[string]any{"secret": "value"},
	})

	if outcome, err := (&journeysteps.SubJourney{}).Execute(transaction, config); err != nil || outcome != "" {
		t.Fatalf("start outcome=%q err=%v", outcome, err)
	}
	if transaction.State.GetClosedCtx().IsDefined(propsKey) {
		t.Fatal("encrypted props leaked into closed context")
	}
	if got := transaction.State.GetEncryptedCtx().Get(propsKey + ".secret").AsStringOr(""); got != "value" {
		t.Fatalf("encrypted active props=%q", got)
	}

	transaction.State.GetClosedCtx().Set(transaction.CurrentStepID, false)
	outcome, err := (&journeysteps.SubJourney{}).Execute(transaction, config)
	if err != nil || outcome != "false" {
		t.Fatalf("return outcome=%q err=%v", outcome, err)
	}
	if transaction.State.GetEncryptedCtx().IsDefined(propsKey) || transaction.State.GetEncryptedCtx().IsDefined(stackKey) {
		t.Fatal("encrypted props or stack remained after completion")
	}
}

func TestSubJourneyPropsStackRestoresNestedFrames(t *testing.T) {
	transaction := newStepTransaction()
	propsKey := env.GetContextKey("props")
	stackKey := env.GetContextKey("props_stack")
	step := &journeysteps.SubJourney{}
	outer := goutils.NewTreeMap(map[string]any{"journey_id": "outer", "props": map[string]any{"level": "outer"}})
	inner := goutils.NewTreeMap(map[string]any{"journey_id": "inner", "props": map[string]any{"level": "inner"}})

	transaction.CurrentStepID = "outer-step"
	if outcome, err := step.Execute(transaction, outer); err != nil || outcome != "" {
		t.Fatalf("outer start outcome=%q err=%v", outcome, err)
	}
	transaction.CurrentStepID = "inner-step"
	if outcome, err := step.Execute(transaction, inner); err != nil || outcome != "" {
		t.Fatalf("inner start outcome=%q err=%v", outcome, err)
	}
	if got := transaction.State.GetClosedCtx().Get(propsKey + ".level").AsStringOr(""); got != "inner" {
		t.Fatalf("nested active props=%q", got)
	}
	if stack := transaction.State.GetClosedCtx().Get(stackKey).AsAnySlice(); len(stack) != 2 {
		t.Fatalf("nested stack length=%d", len(stack))
	}

	transaction.State.GetClosedCtx().Set("inner-step", true)
	if outcome, err := step.Execute(transaction, inner); err != nil || outcome != "true" {
		t.Fatalf("inner return outcome=%q err=%v", outcome, err)
	}
	if got := transaction.State.GetClosedCtx().Get(propsKey + ".level").AsStringOr(""); got != "outer" {
		t.Fatalf("restored outer props=%q", got)
	}
	if stack := transaction.State.GetClosedCtx().Get(stackKey).AsAnySlice(); len(stack) != 1 {
		t.Fatalf("stack length after inner return=%d", len(stack))
	}

	transaction.CurrentStepID = "outer-step"
	transaction.State.GetClosedCtx().Set("outer-step", true)
	if outcome, err := step.Execute(transaction, outer); err != nil || outcome != "true" {
		t.Fatalf("outer return outcome=%q err=%v", outcome, err)
	}
	if transaction.State.GetClosedCtx().IsDefined(propsKey) || transaction.State.GetClosedCtx().IsDefined(stackKey) {
		t.Fatal("props or stack remained after outer completion")
	}
}
