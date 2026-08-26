package steps

import (
	"errors"

	"github.com/nitsugaro/go-journey/env"
	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type SubJourney struct {
	BasicStep

	_            struct{}       `description:"Executes a Journey as a Sub-Journey."`
	JourneyID    string         `json:"journey_id" required:"true" description:"Journey which will be executed."`
	PassTag      bool           `json:"pass_tag" default:"false" description:"Bypass to outcome 'true' automatically if Journey is tagged on session."`
	SetTag       bool           `json:"set_tag" default:"false" description:"Tag Journey on session for future invokations."`
	TagName      string         `json:"tag_name" description:"Journey Tag on the session. If empty will use the Journey Name."`
	Props        map[string]any `json:"props,omitempty" description:"Values exposed to the sub-journey through the configured internal props context."`
	PropsContext string         `json:"props_context" enum:"closedCtx,encCtx" default:"closedCtx" description:"Internal context where sub-journey props and their stack are stored."`
	Outcome      struct {
		True  string `json:"true" required:"true" format:"uuid"`
		False string `json:"false" required:"true" format:"uuid"`
	} `json:"outcome" required:"true"`
}

type subJourneyPropsFrame struct {
	Props       map[string]any `json:"props"`
	HadPrevious bool           `json:"had_previous"`
	Previous    any            `json:"previous,omitempty"`
}

func (uns *SubJourney) GetStepType() string {
	return types.SubJourneyStep
}

func (uns *SubJourney) Execute(journeyTransaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	journeyId, err := config.Get("journey_id").AsString()
	if err != nil {
		return "", err
	}

	tagName := config.Get("tag_name").AsStringOr(journeyId)
	closedCtx := journeyTransaction.State.GetClosedCtx()
	tagsKey := env.GetContextKey("tags")
	if closedCtx.IsDefined(journeyTransaction.CurrentStepID) {
		if err := restoreSubJourneyProps(journeyTransaction, config); err != nil {
			return "", err
		}
		if closedCtx.Delete(journeyTransaction.CurrentStepID).AsBoolOr(false) {
			if config.Get("set_tag").AsBoolOr(false) {
				var currentTags []string
				if err := closedCtx.Get(tagsKey).AsStruct(&currentTags); err != nil {
					currentTags = []string{}
				}
				if !goutils.Some(currentTags, func(tag string, _ int) bool { return tag == tagName }) {
					currentTags = append(currentTags, tagName)
				}

				closedCtx.Set(tagsKey, currentTags)
			}

			return "true", nil
		} else {
			return "false", nil
		}
	}

	if config.Get("pass_tag").AsBoolOr(false) &&
		hasJourneyTag(closedCtx, tagsKey, tagName) {
		return "true", nil
	}

	if err := pushSubJourneyProps(journeyTransaction, config); err != nil {
		return "", err
	}
	journeyTransaction.State.PushTracking(journeyTransaction.Journey.ID, journeyTransaction.CurrentStepID)
	journeyTransaction.State.PushTracking(journeyId, "")

	return "", nil
}

func pushSubJourneyProps(journeyTransaction *types.JourneyTransaction, config goutils.TreeMapImpl) error {
	propsCtx, err := subJourneyPropsContext(journeyTransaction, config)
	if err != nil {
		return err
	}
	props, err := subJourneyConfigProps(config)
	if err != nil {
		return err
	}
	propsKey := env.GetContextKey("props")
	stackKey := env.GetContextKey("props_stack")
	stack := subJourneyPropsStack(propsCtx, stackKey)
	frame := subJourneyPropsFrame{
		Props:       props,
		HadPrevious: propsCtx.IsDefined(propsKey),
		Previous:    propsCtx.Get(propsKey).AsAnyOr(nil),
	}
	stack = append(stack, frame)
	propsCtx.Set(stackKey, stack)
	propsCtx.Set(propsKey, props)
	return nil
}

func restoreSubJourneyProps(journeyTransaction *types.JourneyTransaction, config goutils.TreeMapImpl) error {
	propsCtx, err := subJourneyPropsContext(journeyTransaction, config)
	if err != nil {
		return err
	}
	propsKey := env.GetContextKey("props")
	stackKey := env.GetContextKey("props_stack")
	stack := subJourneyPropsStack(propsCtx, stackKey)
	if len(stack) == 0 {
		return nil
	}
	frame := stack[len(stack)-1]
	stack = stack[:len(stack)-1]
	if frame.HadPrevious {
		propsCtx.Set(propsKey, frame.Previous)
	} else {
		propsCtx.TryDelete(propsKey)
	}
	if len(stack) == 0 {
		propsCtx.TryDelete(stackKey)
	} else {
		propsCtx.Set(stackKey, stack)
	}
	return nil
}

func subJourneyPropsContext(journeyTransaction *types.JourneyTransaction, config goutils.TreeMapImpl) (goutils.TreeMapImpl, error) {
	switch config.Get("props_context").AsStringOr(types.ClosedCtxKey) {
	case types.ClosedCtxKey:
		return journeyTransaction.State.GetClosedCtx(), nil
	case types.EncCtxKey:
		return journeyTransaction.State.GetEncryptedCtx(), nil
	default:
		return nil, errors.New("sub-journey props_context must be closedCtx or encCtx")
	}
}

func subJourneyConfigProps(config goutils.TreeMapImpl) (map[string]any, error) {
	if !config.IsDefined("props") {
		return map[string]any{}, nil
	}
	props, err := config.Get("props").AsMap()
	if err != nil {
		return nil, errors.New("sub-journey props must be an object")
	}
	return props, nil
}

func subJourneyPropsStack(propsCtx goutils.TreeMapImpl, stackKey string) []subJourneyPropsFrame {
	var stack []subJourneyPropsFrame
	if err := propsCtx.Get(stackKey).AsStruct(&stack); err != nil {
		return []subJourneyPropsFrame{}
	}
	return stack
}

func hasJourneyTag(closedCtx goutils.TreeMapImpl, tagsKey, tagName string) bool {
	var tags []string
	if err := closedCtx.Get(tagsKey).AsStruct(&tags); err != nil {
		return false
	}
	return goutils.Some(tags, func(tag string, _ int) bool { return tag == tagName })
}

func init() {
	defaultStepRegistry.AddStep(&SubJourney{}, map[string]map[string]any{
		".": {"x-category": types.FlowCategory, "x-order": []string{"journey_id", "props_context", "props", "pass_tag", "set_tag", "tag_name", "outcome"}},
		"journey_id": {
			"x-type": "selectable",
			"x-props": map[string]any{
				"resource":      "journeys",
				"nameProperty":  "name",
				"valueProperty": "id",
			},
		},
		"props": {"x-sub-journey-props": true},
	})
}
