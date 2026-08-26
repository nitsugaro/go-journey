package steps

import (
	"strings"

	"github.com/PaesslerAG/gval"
	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type AssertRule struct {
	ID         string `json:"id" required:"true" minLength:"1"`
	Expression string `json:"expression" required:"true" minLength:"1"`
	Message    string `json:"message,omitempty"`
}

type Assert struct {
	BasicStep

	_             struct{}     `description:"Evaluates business invariants and records structured failures for conditional routing."`
	Mode          string       `json:"mode" enum:"all,any" default:"all"`
	Rules         []AssertRule `json:"rules" required:"true" minItems:"1"`
	ErrorsContext string       `json:"errors_context" enum:"ctx,encCtx,closedCtx,tempCtx" default:"ctx"`
	ErrorsTarget  string       `json:"errors_target" default:"validationErrors" minLength:"1"`
	Outcome       struct {
		Valid   string `json:"valid" format:"uuid"`
		Invalid string `json:"invalid" format:"uuid"`
		Error   string `json:"error" format:"uuid"`
	} `json:"outcome" required:"true"`
}

func (*Assert) GetStepType() string { return types.AssertStep }

func (*Assert) VerifyConfig(stepName string, config goutils.TreeMapImpl) error {
	if strings.Contains(config.Get("rules").AsStringOr(""), "${") {
		return nil
	}
	rules, err := config.Get("rules").AsSlice()
	if err != nil || len(rules) == 0 {
		return types.StepInvalidConfig(stepName, "at least one assertion rule is required")
	}
	seen := map[string]struct{}{}
	for _, rule := range rules {
		id := rule.Get("id").AsStringOr("")
		if id == "" || rule.Get("expression").AsStringOr("") == "" {
			return types.StepInvalidConfig(stepName, "every assertion requires id and expression")
		}
		if _, exists := seen[id]; exists {
			return types.StepInvalidConfig(stepName, "duplicate assertion id: "+id)
		}
		seen[id] = struct{}{}
	}
	return nil
}

func (*Assert) Execute(transaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	rules, err := config.Get("rules").AsSlice()
	if err != nil {
		return "error", nil
	}
	failures := make([]map[string]any, 0)
	passed := 0
	bindings := transaction.ExpressionBindings()
	for _, rule := range rules {
		value, evaluateErr := gval.Evaluate(rule.Get("expression").AsStringOr(""), bindings)
		if evaluateErr != nil {
			return "error", nil
		}
		valid, ok := value.(bool)
		if !ok {
			return "error", nil
		}
		if valid {
			passed++
		} else {
			failures = append(failures, map[string]any{"id": rule.Get("id").AsStringOr(""), "message": rule.Get("message").AsStringOr("")})
		}
	}
	mode := config.Get("mode").AsStringOr("all")
	valid := (mode == "all" && passed == len(rules)) || (mode == "any" && passed > 0)
	errorsContext := transaction.State.Get(config.Get("errors_context").AsStringOr(types.CtxKey))
	if errorsContext == nil {
		return "error", nil
	}
	errorsTarget := config.Get("errors_target").AsStringOr("validationErrors")
	if valid {
		errorsContext.Delete(errorsTarget)
		return "valid", nil
	}
	errorsContext.Set(errorsTarget, failures)
	return "invalid", nil
}

func init() {
	defaultStepRegistry.AddStep(&Assert{}, map[string]map[string]any{
		".": {"x-category": types.FlowCategory, "x-order": []string{"mode", "rules", "errors_context", "errors_target", "outcome"}},
	})
}
