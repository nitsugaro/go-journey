package steps

import (
	"encoding/json"

	"github.com/nitsugaro/go-journey/inputs"
	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type Metadata struct {
	BasicStep

	_        struct{} `description:"Sends metadata to client."`
	Metadata any      `json:"metadata" required:"true"`
	Format   string   `json:"format" enum:"TEXT,JSON" default:"JSON"`
	Outcome  struct {
		True string `json:"true" format:"uuid"`
	} `json:"outcome" required:"true"`
}

func (m *Metadata) GetStepType() string {
	return types.MetadataStep
}

func (m *Metadata) Execute(journeyTransaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	if _, err := journeyTransaction.ClientInputsBuilder.GetClientInput(journeyTransaction.CurrentStepID, m.GetStepType()); err == nil {
		return "true", nil
	}

	val := config.Get("metadata")
	var metadata any
	if config.Get("format").AsStringOr("TEXT") == "TEXT" {
		metadata = val.AsStringOr("")
	} else {
		var obj map[string]any
		err := json.Unmarshal([]byte(val.ToJsonString(false)), &obj)
		if err != nil {
			metadata = err.Error()
		} else {
			metadata = obj
		}
	}

	journeyTransaction.ClientInputsBuilder.AddMessageInput(&inputs.Message{
		ID:       journeyTransaction.CurrentStepID,
		StepType: m.GetStepType(),
		Value:    metadata,
	})
	return "", nil
}

func init() {
	defaultStepRegistry.AddStep(&Metadata{}, map[string]map[string]any{
		".": {"x-category": types.FlowCategory},
	})
}
