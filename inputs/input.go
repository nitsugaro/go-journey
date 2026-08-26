package inputs

import (
	"encoding/json"
	"errors"
	"sync"

	jcache "github.com/nitsugaro/go-journey/cache"
	"github.com/nitsugaro/go-journey/env"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type Validator func(config goutils.TreeMapImpl, clientInput *ClientInput) *ClientError
type ManagedValidator func(config goutils.TreeMapImpl, clientInput *ClientInput, cacheManager *jcache.Manager) *ClientError

var inputsValidators = map[string]ManagedValidator{}
var inputsValidatorsMu sync.RWMutex

func RegisterValidator(inputType string, validator Validator) bool {
	if inputType == "" || validator == nil {
		return false
	}
	inputsValidatorsMu.Lock()
	defer inputsValidatorsMu.Unlock()
	inputsValidators[inputType] = func(config goutils.TreeMapImpl, input *ClientInput, _ *jcache.Manager) *ClientError {
		return validator(config, input)
	}
	return true
}

func RegisterManagedValidator(inputType string, validator ManagedValidator) bool {
	if inputType == "" || validator == nil {
		return false
	}
	inputsValidatorsMu.Lock()
	defer inputsValidatorsMu.Unlock()
	inputsValidators[inputType] = validator
	return true
}

type IClientInput interface {
	GetID() string
	GetInputType() string
	GetOutput() goutils.TreeMapImpl
	GetInput() goutils.TreeMapImpl
	GetSendBack() bool
	AsClientInput() *ClientInput
	Verify(config goutils.TreeMapImpl) bool
}

type ClientError struct {
	Error   string                 `json:"error"`
	Details map[string]interface{} `json:"details"`
}

type ClientInput struct {
	ID         string      `json:"id,omitempty"`
	ExternalID string      `json:"external_id,omitempty"`
	StepType   string      `json:"step_type"`
	Type       string      `json:"type"`
	SendBack   bool        `json:"send_back"`
	Output     interface{} `json:"output,omitempty"`
	Input      interface{} `json:"input,omitempty"`
}

func (ci ClientInputsBuilder) IsClientEmpty() bool {
	return len(ci.clientInputs) == 0
}

func (ci ClientInputsBuilder) IsNewEmpty() bool {
	return len(ci.newClientInputs) == 0
}

func (i *ClientInput) GetID() string        { return i.ID }
func (i *ClientInput) GetInputType() string { return i.Type }
func (i *ClientInput) GetOutput() goutils.TreeMapImpl {
	return goutils.NewTreeMap(i.Output)
}
func (i *ClientInput) GetInput() goutils.TreeMapImpl {
	return goutils.NewTreeMap(i.Input)
}
func (i *ClientInput) Verify(config goutils.TreeMapImpl) *ClientError {
	return i.verify(config, nil)
}

func (i *ClientInput) verify(config goutils.TreeMapImpl, cacheManager *jcache.Manager) *ClientError {
	if i == nil {
		return &ClientError{Error: "client input is nil"}
	}
	inputType := config.Get("type")
	if inputType.IsEmpty() {
		return &ClientError{Error: "input type not found"}
	}

	inputsValidatorsMu.RLock()
	validator, ok := inputsValidators[inputType.AsStringOr("")]
	inputsValidatorsMu.RUnlock()
	if !ok || validator == nil {
		return &ClientError{Error: "unsupported input type: " + inputType.AsStringOr("")}
	}
	return validator(config, i, cacheManager)
}

func (i *ClientInput) GetSendBack() bool { return i.SendBack }

func (i *ClientInput) AsClientInput() *ClientInput { return i }

type ClientInputsBuilder struct {
	clientInputs    []*ClientInput
	newClientInputs []*ClientInput
	ctxManager      goutils.TreeMapImpl
	cacheManager    *jcache.Manager
}

func NewClientInputBuilder(clientInputs []*ClientInput, ctxManager goutils.TreeMapImpl, managers ...*jcache.Manager) *ClientInputsBuilder {
	var cacheManager *jcache.Manager
	if len(managers) > 0 {
		cacheManager = managers[0]
	}
	return &ClientInputsBuilder{newClientInputs: []*ClientInput{}, ctxManager: ctxManager, clientInputs: clientInputs, cacheManager: cacheManager}
}

func (ci *ClientInputsBuilder) GetCtxManager() goutils.TreeMapImpl {
	return ci.ctxManager
}

func (ci *ClientInputsBuilder) GetProvidedInputs() []*ClientInput {
	return append([]*ClientInput(nil), ci.clientInputs...)
}

func (ci *ClientInputsBuilder) GetFromID(ID string) []*ClientInput {
	return goutils.Filter(ci.clientInputs, func(ci *ClientInput, i int) bool {
		return ci.GetID() == ID
	})
}

func (ci *ClientInputsBuilder) GetFirstFromID(ID string) *ClientInput {
	result := ci.GetFromID(ID)

	if len(result) == 0 {
		return nil
	}

	return result[0]
}

func (ci *ClientInputsBuilder) GetFirstFromExternalID(externalID string) *ClientInput {
	for _, input := range ci.clientInputs {
		if input != nil && input.ExternalID == externalID {
			return input
		}
	}
	return nil
}

func (ci *ClientInputsBuilder) GetByType(inputType string) []*ClientInput {
	return goutils.Filter(ci.clientInputs, func(ci *ClientInput, i int) bool {
		return ci.GetInputType() == inputType
	})
}

func (cib *ClientInputsBuilder) Add(clientInput *ClientInput, config interface{}) error {
	if clientInput == nil || (clientInput.ID == "" && clientInput.ExternalID == "") {
		return errors.New("client input identity is required")
	}

	cib.newClientInputs = append(cib.newClientInputs, clientInput)

	var clientInputConfigs []any
	err := cib.ctxManager.Get(env.GetClientInputsKey()).AsStruct(&clientInputConfigs)
	if err != nil {
		clientInputConfigs = []any{}
	}

	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	var storedConfig map[string]any
	if err := json.Unmarshal(data, &storedConfig); err != nil {
		return err
	}
	clientInputConfigs = append(clientInputConfigs, storedConfig)

	cib.ctxManager.Set(env.GetClientInputsKey(), clientInputConfigs)

	return nil
}

// ValidateProvided authenticates submitted client inputs against the request
// definitions stored by input producers. Execution code does not need to know
// how an input type chooses its public identity.
func (cib *ClientInputsBuilder) ValidateProvided(currentStepID string) *ClientError {
	provided := cib.GetProvidedInputs()
	configs, err := cib.ctxManager.Get(env.GetClientInputsKey()).AsSlice()
	if err != nil {
		if len(provided) == 0 {
			return nil
		}
		return &ClientError{Error: "client inputs were not requested"}
	}
	configured := make(map[string]goutils.TreeMapImpl, len(configs))
	for _, config := range configs {
		stepID := requestStepID(config)
		if stepID != "" && stepID != currentStepID {
			continue
		}
		identity := inputIdentity(config.Get("external_id").AsStringOr(""), config.Get("id").AsStringOr(""))
		if identity != "" {
			configured[identity] = config
		}
	}
	seen := make(map[string]struct{}, len(provided))
	for _, input := range provided {
		if input == nil {
			return &ClientError{Error: "invalid client input"}
		}
		identity := inputIdentity(input.ExternalID, input.ID)
		if identity == "" {
			return &ClientError{Error: "invalid client input"}
		}
		if _, duplicate := seen[identity]; duplicate {
			return &ClientError{Error: "duplicate client input: " + identity}
		}
		seen[identity] = struct{}{}
		config, ok := configured[identity]
		if !ok {
			return &ClientError{Error: "unexpected client input: " + identity}
		}
		configuredID := config.Get("id").AsStringOr("")
		if (input.ID != "" && configuredID != input.ID) || config.Get("step_type").AsStringOr("") != input.StepType || config.Get("type").AsStringOr("") != input.Type {
			return &ClientError{Error: "unexpected client input: " + identity}
		}
		if configuredID != "" {
			input.ID = configuredID
		}
		if validationError := input.verify(config, cib.cacheManager); validationError != nil {
			return validationError
		}
	}
	for identity, config := range configured {
		if config.Get("required").AsBoolOr(false) {
			if _, supplied := seen[identity]; !supplied {
				return &ClientError{Error: "missing required client input", Details: map[string]interface{}{"external_id": identity}}
			}
		}
	}
	return nil
}

func inputIdentity(externalID, id string) string {
	if externalID != "" {
		return externalID
	}
	return id
}

func (cib *ClientInputsBuilder) GetNewInputs() []*ClientInput {
	return cib.newClientInputs
}

func (cib *ClientInputsBuilder) GetClientInputs(inputsID string) []goutils.TreeMapImpl {
	if clientInputsConfig, err := cib.GetCtxManager().Get(env.GetClientInputsKey()).AsSlice(); err == nil {
		return goutils.Filter(clientInputsConfig, func(ci goutils.TreeMapImpl, _ int) bool { return ci.Get("id").AsStringOr("") == inputsID })
	}

	return []goutils.TreeMapImpl{}
}

func (cib *ClientInputsBuilder) GetClientInput(inputsID, stepType string) (*ClientInput, error) {
	if clientInputsConfig, err := cib.GetCtxManager().Get(env.GetClientInputsKey()).AsSlice(); err == nil {
		for _, client := range clientInputsConfig {
			if client.Get("id").AsStringOr("") == inputsID && client.Get("step_type").AsStringOr("") == stepType {
				var clientInput ClientInput
				err := client.AsStruct(&clientInput)
				return &clientInput, err
			}
		}
	}

	return nil, errors.New("not found")
}

func (cib *ClientInputsBuilder) WasRequested(externalID, journeyStepID string) bool {
	configs, err := cib.ctxManager.Get(env.GetClientInputsKey()).AsSlice()
	if err != nil {
		return false
	}
	for _, config := range configs {
		requestID := requestStepID(config)
		if config.Get("external_id").AsStringOr("") == externalID && (requestID == "" || requestID == journeyStepID) {
			return true
		}
	}
	return false
}

func (cib *ClientInputsBuilder) ClearRequestsForStep(journeyStepID string) {
	configs, err := cib.ctxManager.Get(env.GetClientInputsKey()).AsSlice()
	if err != nil {
		return
	}
	remaining := make([]any, 0, len(configs))
	for _, config := range configs {
		requestID := requestStepID(config)
		if requestID == "" || requestID == journeyStepID {
			continue
		}
		if value, mapErr := config.AsMap(); mapErr == nil {
			remaining = append(remaining, value)
		}
	}
	if len(remaining) == 0 {
		cib.ctxManager.Delete(env.GetClientInputsKey())
		return
	}
	cib.ctxManager.Set(env.GetClientInputsKey(), remaining)
}

func requestStepID(config goutils.TreeMapImpl) string {
	return config.Get("journey_step_id").AsStringOr(config.Get("id").AsStringOr(""))
}
