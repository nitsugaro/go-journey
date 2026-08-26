package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	goutils "github.com/nitsugaro/go-utils/v2"
)

// ################ STEPS_TYPES #####################

const (
	AsyncExecStep             = "AsyncExec"
	AsyncWaitStep             = "AsyncWait"
	ScriptStep                = "Script"
	SubJourneyStep            = "SubJourney"
	ChainStep                 = "Chain"
	ChoiceStep                = "Choice"
	FormStep                  = "Form"
	TransformStep             = "Transform"
	WaitUntilStep             = "WaitUntil"
	AssertStep                = "Assert"
	VerifyJWTStep             = "VerifyJWT"
	SignJWTStep               = "SignJWT"
	OIDCAuthorizationCodeStep = "OIDCAuthorizationCode"
	LDAPSearchStep            = "LDAPSearch"
	LDAPBindStep              = "LDAPBind"
	LDAPCompareStep           = "LDAPCompare"
	LDAPModifyStep            = "LDAPModify"
	LDAPAddStep               = "LDAPAdd"
	LDAPDeleteStep            = "LDAPDelete"
	LDAPModifyDNStep          = "LDAPModifyDN"
	ConditionStep             = "Condition"
	RandomStep                = "Random"
	RetryStep                 = "Retry"
	SuccessStep               = "Success"
	FailureStep               = "Failure"
	HTTPResponseStep          = "HTTPResponse"
	HTTPFinishResponseStep    = "HTTPFinishResponse"
	HTTPProxyStep             = "HTTPProxy"
	SetCookieStep             = "SetCookie"
	ReadCookieStep            = "ReadCookie"
	SchemaValidatorStep       = "SchemaValidator"
	ScheduleCacheGetStep      = "ScheduleCacheGet"
	ScheduleCacheRefreshStep  = "ScheduleCacheRefresh"
	ScheduleCacheClearStep    = "ScheduleCacheClear"
	EndStep                   = "End"

	PASSWORD_STEP      = "Password"
	PASSWORD_AUTH_STEP = "PasswordAuth"

	GET_OAUTH_CTX_STEP   = "GetOAuthCtx"
	SuspendFlowStep      = "SuspendFlow"
	IfExpressionStep     = "IfExpression"
	SwitchExpressionStep = "SwitchExpression"
	NOT_EMPTY_STEP       = "NotEmpty"
	HttpRequestStep      = "HttpRequest"
	ExtendExpStep        = "ExtendExp"
	MetadataStep         = "Metadata"

	SetStaticPropertyStep = "SetStaticProperty"
	SetCtxPropertyStep    = "SetCtxProperty"
	RemoveCtxPropertyStep = "RemoveCtxProperty"
)

var (
	ErrStep = errors.New("step error")

	ErrStepUnsupported   = fmt.Errorf("unsupported step type: %w", ErrStep)
	ErrInvalidStepConfig = fmt.Errorf("invalid step config: %w", ErrStep)
	ErrInvalidOutcome    = fmt.Errorf("invalid outcome: %w", ErrStep)
	ErrStepNotFound      = fmt.Errorf("step not found: %w", ErrStep)
)

func StepUnsupported(step string) error {
	return fmt.Errorf("step '%s' is not supported: %w", step, ErrStep)
}

func StepNotFound(step string) error {
	return fmt.Errorf("step '%s' not found: %w", step, ErrStep)
}

func StepInvalidConfig(step string, detail string) error {
	return fmt.Errorf("invalid config in step '%s' (%s): %w", step, detail, ErrStep)
}

func StepInvalidOutcome(step string, outcome string) error {
	return fmt.Errorf("invalid outcome '%s' in step '%s': %w", outcome, step, ErrStep)
}

// ##########################

type StepConfig struct {
	Outcome map[string]any          `json:"outcome"`
	Vars    map[string]StepVariable `json:"vars"`
}

type StepVariable struct {
	Type         string            `json:"type"`
	Placeholders []StepPlaceholder `json:"placeholders"`
	_            struct{}          `additionalProperties:"false"`
}

type StepPlaceholder struct {
	Template string `json:"template"`
	StartsAt int    `json:"starts_at"`
	EndsAt   int    `json:"ends_at"`
}

type Step struct {
	Name     string `json:"name"`
	StepType string `json:"step_type" binding:"required"`
	Config   any    `json:"config"`
}

func (sc *Step) GetOutcomeID(outcome string) (string, error) {
	if sc.StepType != ScriptStep {
		return goutils.NewTreeMap(sc.Config).Get("outcome." + outcome).AsString()
	}
	outcomes, err := goutils.NewTreeMap(sc.Config).Get("outcome").AsMap()
	if err != nil {
		return "", err
	}
	normalized := strings.ToLower(strings.TrimSpace(outcome))
	for name, target := range outcomes {
		if strings.ToLower(strings.TrimSpace(name)) != normalized {
			continue
		}
		return goutils.NewTreeMap(target).AsString()
	}
	return "", fmt.Errorf("outcome %q is not configured", outcome)
}

type IStep interface {
	GetStepType() string
	EndJourney() bool
	VerifyConfig(string, goutils.TreeMapImpl) error
	Execute(*JourneyTransaction, goutils.TreeMapImpl) (string, error)
}

// ExecuteStepConfig is the single dispatch boundary for step configuration.
// It resolves an isolated runtime view immediately before execution, ensuring
// composite children observe state produced by earlier children.
func ExecuteStepConfig(step IStep, transaction *JourneyTransaction, rawConfig any) (string, error) {
	resolved, err := ResolveStepConfig(rawConfig, transaction.State, transaction.PlaceholderResolvers)
	if err != nil {
		return "", err
	}
	return step.Execute(transaction, resolved)
}

// JourneyCompletion lets a terminal step describe how the journey ended.
// Steps that do not implement it are treated as successful. This keeps the
// executor independent from concrete step types.
type JourneyCompletion interface {
	JourneySucceeded() bool
}

type Steps struct {
	mu      sync.RWMutex
	mergeMu sync.Mutex
	steps   map[string]IStep
	schemas *SchemaForm
}

func NewStepRegistry() *Steps {
	return &Steps{
		steps:   map[string]IStep{},
		schemas: New(),
	}
}

func (s *Steps) GetSteps() map[string]IStep {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]IStep, len(s.steps))
	for name, step := range s.steps {
		result[name] = step
	}
	return result
}

func (s *Steps) GetSchemas() *SchemaForm {
	return s.schemas
}

func (s *Steps) AddStep(step IStep, extraProperties map[string]map[string]any) {
	if step == nil {
		return
	}
	extraProperties = withDefaultStepFlowTypes(step.GetStepType(), extraProperties)
	s.schemas.AddSchema(step, extraProperties)
	s.mu.Lock()
	defer s.mu.Unlock()
	(s.steps)[step.GetStepType()] = step
}

func withDefaultStepFlowTypes(stepType string, extraProperties map[string]map[string]any) map[string]map[string]any {
	result := make(map[string]map[string]any, len(extraProperties)+1)
	for key, value := range extraProperties {
		next := make(map[string]any, len(value))
		for property, propertyValue := range value {
			next[property] = propertyValue
		}
		result[key] = next
	}
	root := result["."]
	if root == nil {
		root = map[string]any{}
		result["."] = root
	}
	if _, exists := root["x-flow-type"]; !exists {
		root["x-flow-type"] = defaultStepFlowTypes(stepType)
	}
	return result
}

func defaultStepFlowTypes(stepType string) []string {
	switch stepType {
	case FormStep, ChoiceStep, MetadataStep, SuspendFlowStep, WaitUntilStep, SuccessStep, FailureStep, OIDCAuthorizationCodeStep:
		return []string{AuthJourney}
	case HTTPResponseStep, HTTPFinishResponseStep, HTTPProxyStep:
		return []string{ResourceJourney}
	case SetCookieStep, ReadCookieStep:
		return []string{AuthJourney, ResourceJourney}
	case EndStep:
		return []string{WorkflowJourney}
	default:
		return []string{AuthJourney, ResourceJourney, WorkflowJourney}
	}
}

func (s *Steps) GetStep(stepID string) IStep {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return (s.steps)[stepID]
}

// AddMissingFrom adds implementations and their existing schemas without
// replacing entries already registered in the receiver.
func (s *Steps) AddMissingFrom(source *Steps) error {
	if source == nil || source == s {
		return nil
	}
	s.mergeMu.Lock()
	defer s.mergeMu.Unlock()
	for stepType, step := range source.GetSteps() {
		if s.GetStep(stepType) != nil {
			if _, hasSchema := s.schemas.GetSchema(stepType); !hasSchema {
				if schema, ok := source.schemas.GetSchema(stepType); ok {
					if err := s.schemas.AddRawSchema(stepType, schema); err != nil {
						return err
					}
				}
			}
			continue
		}
		if schema, ok := source.schemas.GetSchema(stepType); ok {
			if err := s.schemas.AddRawSchema(stepType, schema); err != nil {
				return err
			}
		}
		s.mu.Lock()
		if s.steps[stepType] == nil {
			s.steps[stepType] = step
		}
		s.mu.Unlock()
	}
	return nil
}

func (s *Steps) GetStepSchemasJSON() map[string]json.RawMessage {
	schemas := map[string]json.RawMessage{}
	for _, step := range s.GetSteps() {
		if schemaBytes, ok := s.schemas.GetSchema(step.GetStepType()); ok {
			schemas[step.GetStepType()] = json.RawMessage(schemaBytes)
		}
	}

	return schemas
}

func (s *Steps) ValidateStep(currentStep *Step) error {
	if len(currentStep.Name) == 0 {
		currentStep.Name = currentStep.StepType
	}

	if _, ok := s.schemas.GetSchema(currentStep.StepType); !ok {
		return StepUnsupported(currentStep.StepType)
	} else if currentStep.Config != nil {
		bytes, _ := json.Marshal(currentStep.Config)
		if err := s.schemas.Validate(currentStep.StepType, bytes); err != nil {
			return StepInvalidConfig(currentStep.Name, err.Error())
		}

		if err := s.GetStep(currentStep.StepType).VerifyConfig(currentStep.Name, goutils.NewTreeMap(currentStep.Config)); err != nil {
			return err
		}

		if err := validateStepVariables(currentStep.Name, goutils.NewTreeMap(currentStep.Config)); err != nil {
			return err
		}
	}

	return nil
}

func (s *Steps) ValidateStepForJourneyType(currentStep *Step, journeyType string) error {
	if err := s.ValidateStep(currentStep); err != nil {
		return err
	}
	if !s.StepSupportsJourneyType(currentStep.StepType, NormalizeJourneyType(journeyType)) {
		return StepInvalidConfig(currentStep.Name, fmt.Sprintf("step type %q is not supported by journey type %q", currentStep.StepType, NormalizeJourneyType(journeyType)))
	}
	return nil
}

func (s *Steps) StepSupportsJourneyType(stepType string, journeyType string) bool {
	flowTypes := s.StepFlowTypes(stepType)
	return goutils.Some(flowTypes, func(flowType string, _ int) bool { return flowType == NormalizeJourneyType(journeyType) })
}

func (s *Steps) StepFlowTypes(stepType string) []string {
	schemaBytes, ok := s.schemas.GetSchema(stepType)
	if !ok {
		return nil
	}
	var schema map[string]any
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		return nil
	}
	rawTypes, exists := schema["x-flow-type"]
	if !exists {
		return []string{AuthJourney}
	}
	switch value := rawTypes.(type) {
	case []any:
		result := make([]string, 0, len(value))
		for _, raw := range value {
			if text, ok := raw.(string); ok && text != "" {
				result = append(result, text)
			}
		}
		if len(result) != 0 {
			return result
		}
	case []string:
		if len(value) != 0 {
			return append([]string(nil), value...)
		}
	case string:
		if value != "" {
			return []string{value}
		}
	}
	return []string{AuthJourney}
}

func validateStepVariables(stepName string, config goutils.TreeMapImpl) error {
	vars, err := config.Get("vars").AsMap()
	if err != nil {
		return nil
	}
	for property, raw := range vars {
		var variable StepVariable
		if err := goutils.NewTreeMap(raw).AsStruct(&variable); err != nil {
			return StepInvalidConfig(stepName, fmt.Sprintf("invalid vars.%s: %v", property, err))
		}
		if !validVariableType(variable.Type) {
			return StepInvalidConfig(stepName, fmt.Sprintf("unsupported vars.%s type %q", property, variable.Type))
		}
		source := config.Get(property).AsStringOr("")
		placeholders := append([]StepPlaceholder(nil), variable.Placeholders...)
		sort.Slice(placeholders, func(i, j int) bool { return placeholders[i].StartsAt < placeholders[j].StartsAt })
		previousEnd := 0
		for _, placeholder := range placeholders {
			end := placeholder.EndsAt
			if placeholder.StartsAt < previousEnd || placeholder.StartsAt < 0 || end < placeholder.StartsAt || end > len(source) {
				return StepInvalidConfig(stepName, fmt.Sprintf("invalid placeholder offsets in vars.%s", property))
			}
			token := placeholderToken(placeholder.Template)
			if source[placeholder.StartsAt:end] != token {
				return StepInvalidConfig(stepName, fmt.Sprintf("stale placeholder %q in vars.%s", placeholder.Template, property))
			}
			path := placeholderPath(placeholder.Template)
			parts := strings.SplitN(path, ".", 2)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				return StepInvalidConfig(stepName, fmt.Sprintf("invalid placeholder %q in vars.%s", placeholder.Template, property))
			}
			previousEnd = end
		}
	}
	return nil
}

func validVariableType(variableType string) bool {
	switch variableType {
	case "", "string", "number", "float", "integer", "int", "boolean", "bool", "object", "array":
		return true
	default:
		return false
	}
}

// ResolveStepConfig returns an isolated step configuration with every vars entry
// resolved and converted to its declared type. The original journey configuration
// is never mutated, so concurrent executions remain independent.
func ResolveStepConfig(raw any, state *JourneyState, customResolvers ...map[string]PlaceholderResolver) (goutils.TreeMapImpl, error) {
	// Most step configurations have no runtime variables. Avoid serialization
	// and allocation entirely on that hot path; step configurations are immutable.
	if config, ok := raw.(map[string]any); ok {
		if _, hasVariables := config["vars"]; !hasVariables {
			return goutils.NewTreeMap(config), nil
		}
	}
	original, ok := raw.(map[string]any)
	if !ok {
		serialized, err := json.Marshal(raw)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(serialized, &original); err != nil {
			return nil, err
		}
	}
	config := goutils.NewTreeMap(original)
	variables, err := config.Get("vars").AsMap()
	if err != nil {
		return config, nil
	}
	updates := newConfigUpdateNode()
	for property, rawVariable := range variables {
		variable, err := decodeStepVariable(rawVariable)
		if err != nil {
			return nil, fmt.Errorf("resolve vars.%s: %w", property, err)
		}
		source := config.Get(property).AsStringOr("")
		var resolvers map[string]PlaceholderResolver
		if len(customResolvers) != 0 {
			resolvers = customResolvers[0]
		}
		value, err := resolveStepVariable(source, variable, state, resolvers)
		if err != nil {
			return nil, fmt.Errorf("resolve vars.%s: %w", property, err)
		}
		if err := updates.add(strings.Split(property, "."), value); err != nil {
			return nil, fmt.Errorf("resolve vars.%s: %w", property, err)
		}
	}
	resolved, err := updates.apply(original)
	if err != nil {
		return nil, err
	}
	return goutils.NewTreeMap(resolved.(map[string]any)), nil
}

func decodeStepVariable(raw any) (*StepVariable, error) {
	switch variable := raw.(type) {
	case StepVariable:
		return &variable, nil
	case *StepVariable:
		if variable != nil {
			return variable, nil
		}
	}
	var variable StepVariable
	err := goutils.NewTreeMap(raw).AsStruct(&variable)
	return &variable, err
}

func resolveStepVariable(source string, variable *StepVariable, state *JourneyState, resolvers map[string]PlaceholderResolver) (any, error) {
	if variable == nil {
		return nil, fmt.Errorf("step variable is nil")
	}
	value := source
	placeholders := append([]StepPlaceholder(nil), variable.Placeholders...)
	if len(placeholders) == 1 && placeholders[0].StartsAt == 0 && placeholders[0].EndsAt == len(value) && placeholderToken(placeholders[0].Template) == value {
		resolved, err := resolvePlaceholderValue(placeholders[0].Template, state, resolvers)
		if err != nil {
			return nil, err
		}
		if variable.Type == "" {
			return resolved, nil
		}
		if variable.Type == "object" {
			if object, ok := resolved.(map[string]any); ok {
				return object, nil
			}
		}
		if variable.Type == "array" {
			if array, ok := resolved.([]any); ok {
				return array, nil
			}
		}
	}
	sort.Slice(placeholders, func(i, j int) bool { return placeholders[i].StartsAt < placeholders[j].StartsAt })
	var builder strings.Builder
	builder.Grow(len(value))
	cursor := 0
	for _, placeholder := range placeholders {
		start, end := placeholder.StartsAt, placeholder.EndsAt
		if start < cursor || end < start || end > len(value) {
			return nil, fmt.Errorf("invalid placeholder offsets")
		}
		builder.WriteString(value[cursor:start])
		resolved, err := resolvePlaceholderValue(placeholder.Template, state, resolvers)
		if err != nil {
			return nil, err
		}
		if resolved != nil {
			builder.WriteString(fmt.Sprint(resolved))
		}
		cursor = end
	}
	builder.WriteString(value[cursor:])
	return convertStepVariable(builder.String(), variable.Type)
}

func resolvePlaceholderValue(template string, state *JourneyState, resolvers map[string]PlaceholderResolver) (any, error) {
	path := placeholderPath(template)
	parts := strings.SplitN(path, ".", 2)
	if len(parts) != 2 || state == nil {
		return nil, fmt.Errorf("invalid placeholder path %q", path)
	}
	ctx := state.Get(parts[0])
	if ctx != nil {
		return ctx.Get(parts[1]).AsAnyOr(nil), nil
	}
	resolver := resolvers[parts[0]]
	if resolver == nil {
		return nil, fmt.Errorf("placeholder resolver %q is not registered", parts[0])
	}
	return resolver(parts[1])
}

func placeholderPath(template string) string {
	return strings.TrimSuffix(strings.TrimPrefix(template, "${"), "}")
}

func placeholderToken(template string) string {
	return "${" + placeholderPath(template) + "}"
}

type configUpdateNode struct {
	children map[string]*configUpdateNode
	value    any
	set      bool
}

func newConfigUpdateNode() *configUpdateNode {
	return &configUpdateNode{children: map[string]*configUpdateNode{}}
}

func (node *configUpdateNode) add(parts []string, value any) error {
	if len(parts) == 0 || parts[0] == "" {
		return fmt.Errorf("empty property path")
	}
	current := node
	for _, part := range parts {
		if part == "" {
			return fmt.Errorf("empty property path segment")
		}
		next := current.children[part]
		if next == nil {
			next = newConfigUpdateNode()
			current.children[part] = next
		}
		current = next
	}
	current.value = value
	current.set = true
	return nil
}

// apply performs a branch-only copy: unrelated maps, slices, and scalar values
// remain shared with the immutable stored configuration.
func (node *configUpdateNode) apply(current any) (any, error) {
	if node.set {
		if len(node.children) != 0 {
			return nil, fmt.Errorf("variable paths overlap")
		}
		return node.value, nil
	}
	switch container := current.(type) {
	case map[string]any:
		cloned := make(map[string]any, len(container))
		for key, value := range container {
			cloned[key] = value
		}
		for key, child := range node.children {
			value, exists := container[key]
			if !exists {
				return nil, fmt.Errorf("property path not found")
			}
			resolved, err := child.apply(value)
			if err != nil {
				return nil, err
			}
			cloned[key] = resolved
		}
		return cloned, nil
	case []any:
		cloned := append([]any(nil), container...)
		for rawIndex, child := range node.children {
			index, err := strconv.Atoi(rawIndex)
			if err != nil || index < 0 || index >= len(container) {
				return nil, fmt.Errorf("invalid array index %q", rawIndex)
			}
			resolved, err := child.apply(container[index])
			if err != nil {
				return nil, err
			}
			cloned[index] = resolved
		}
		return cloned, nil
	default:
		return nil, fmt.Errorf("property path crosses a non-container value")
	}
}

func convertStepVariable(value, variableType string) (any, error) {
	switch variableType {
	case "", "string":
		return value, nil
	case "number", "float":
		return strconv.ParseFloat(value, 64)
	case "integer", "int":
		return strconv.ParseInt(value, 10, 64)
	case "boolean", "bool":
		return strconv.ParseBool(value)
	case "object":
		var object map[string]any
		err := json.Unmarshal([]byte(value), &object)
		return object, err
	case "array":
		var array []any
		err := json.Unmarshal([]byte(value), &array)
		return array, err
	default:
		return nil, fmt.Errorf("unsupported variable type %q", variableType)
	}
}
