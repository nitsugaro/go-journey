package inputs

import (
	goutils "github.com/nitsugaro/go-utils/v2"
)

const MSG_INPUT = "msg_input"

type Message struct {
	ID       string `json:"id"`
	StepType string `json:"step_type,omitempty"`
	Type     string `json:"type"`
	Value    any    `json:"value"`
}

func (cib *ClientInputsBuilder) AddMessageInput(message *Message) error {
	clientinput := &ClientInput{
		Type:     MSG_INPUT,
		ID:       message.ID,
		SendBack: false,
		StepType: message.StepType,
		Output:   message.Value,
		Input:    nil,
	}

	message.Type = MSG_INPUT

	cib.Add(clientinput, message)

	return nil
}

// MSG INPUT VALIDATOR
func init() {
	RegisterValidator(MSG_INPUT, func(config goutils.TreeMapImpl, clientInput *ClientInput) *ClientError {
		return nil
	})
}
