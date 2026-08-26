package inputs_test

import (
	"testing"

	"github.com/nitsugaro/go-journey/inputs"
	goutils "github.com/nitsugaro/go-utils/v2"
)

func TestRegisterCustomValidator(t *testing.T) {
	const inputType = "custom-test-input"
	called := false
	if !inputs.RegisterValidator(inputType, func(goutils.TreeMapImpl, *inputs.ClientInput) *inputs.ClientError {
		called = true
		return nil
	}) {
		t.Fatal("RegisterValidator returned false")
	}
	input := &inputs.ClientInput{ID: "custom", Type: inputType}
	if validationError := input.Verify(goutils.NewTreeMap(map[string]any{"type": inputType})); validationError != nil {
		t.Fatalf("custom validator returned %v", validationError)
	}
	if !called {
		t.Fatal("custom validator was not called")
	}
}

func TestValueInputValidation(t *testing.T) {
	minimum := 3.0
	maximum := 5.0
	tests := []struct {
		name      string
		inputType string
		value     any
		pattern   string
		wantError bool
	}{
		{name: "string", inputType: inputs.STRING_INPUT, value: "Ada", pattern: "^[A-Z][a-z]+$"},
		{name: "missing required", inputType: inputs.STRING_INPUT, value: nil, wantError: true},
		{name: "short string", inputType: inputs.STRING_INPUT, value: "Al", wantError: true},
		{name: "integer", inputType: inputs.INT_INPUT, value: float64(4)},
		{name: "fractional integer", inputType: inputs.INT_INPUT, value: 4.5, wantError: true},
		{name: "float", inputType: inputs.FLOAT_INPUT, value: 4.5},
		{name: "boolean", inputType: inputs.BOOL_INPUT, value: true},
		{name: "wrong boolean", inputType: inputs.BOOL_INPUT, value: "true", wantError: true},
		{name: "object", inputType: inputs.OBJECT_INPUT, value: map[string]any{"id": "1"}},
		{name: "wrong object", inputType: inputs.OBJECT_INPUT, value: []any{"1"}, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configMap := goutils.NewTreeMap(map[string]any{
				"id": "value", "external_id": "form.value", "type": test.inputType,
				"required": true, "pattern": test.pattern, "min": minimum, "max": maximum,
			})
			clientInput := &inputs.ClientInput{ID: "value", ExternalID: "form.value", Type: test.inputType, Input: test.value}
			validationError := clientInput.Verify(configMap)
			if (validationError != nil) != test.wantError {
				t.Fatalf("Verify() error = %v, wantError %v", validationError, test.wantError)
			}
		})
	}
}
