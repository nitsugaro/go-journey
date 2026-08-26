package inputs

import (
	"fmt"

	jcache "github.com/nitsugaro/go-journey/cache"
	goutils "github.com/nitsugaro/go-utils/v2"
)

const STR_INPUT = "str"

type StrInputConfig struct {
	ID           string `json:"id,omitempty"`
	StepType     string `json:"step_type,omitempty"`
	DefaultValue string `json:"default_value,omitempty"`
	Prompt       string `json:"prompt,omitempty"`
	Required     bool   `json:"required,omitempty"`
	Pattern      string `json:"pattern,omitempty"`
	Type         string `json:"type,omitempty"`
	Value        any    `json:"value,omitempty"`
}

func (cib *ClientInputsBuilder) AddStrInput(config *StrInputConfig) error {
	clientinput := &ClientInput{
		StepType: config.StepType,
		Type:     STR_INPUT,
		ID:       config.ID,
		SendBack: true,
		Output:   &StrInputConfig{Required: config.Required, Prompt: config.Prompt, Pattern: config.Pattern, Value: config.Value, DefaultValue: config.DefaultValue},
		Input:    nil,
	}

	config.Type = STR_INPUT

	return cib.Add(clientinput, config)
}

// STR INPUT VALIDATOR
func init() {
	RegisterManagedValidator(STR_INPUT, func(config goutils.TreeMapImpl, clientInput *ClientInput, cacheManager *jcache.Manager) *ClientError {
		var strConfig StrInputConfig
		err := config.AsStruct(&strConfig)
		if err != nil {
			return &ClientError{Error: "cannot serialize config: " + err.Error()}
		}

		val := clientInput.GetInput().AsStringOr(strConfig.DefaultValue)

		if strConfig.Required && len(val) == 0 {
			return &ClientError{Error: fmt.Sprintf("'%s' value is required", strConfig.ID)}
		}

		if len(strConfig.Pattern) != 0 {
			regex, err := jcache.GetRegexp(cacheManager, strConfig.Pattern)
			if err != nil || !regex.MatchString(val) {
				return &ClientError{Error: fmt.Sprintf("'%s' value doesn't match pattern: %s", strConfig.ID, strConfig.Pattern)}
			}
		}

		return nil
	})
}
