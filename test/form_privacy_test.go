package main

import (
	"encoding/json"
	"strings"
	"testing"

	gojourney "github.com/nitsugaro/go-journey"
	"github.com/nitsugaro/go-journey/inputs"
	"github.com/nitsugaro/go-journey/types"
	"github.com/nitsugaro/go-nstore"
)

type jsonJourneyTokens struct{}

func (jsonJourneyTokens) Sign(state *types.JourneyState) ([]byte, error) {
	return json.Marshal(state)
}

func (jsonJourneyTokens) Validate(token string) (*types.JourneyState, error) {
	var state types.JourneyState
	err := json.Unmarshal([]byte(token), &state)
	return &state, err
}

func TestFormKeepsPrivateIDsAndRequestDefinitionsOffClientContract(t *testing.T) {
	const (
		formID    = "00000000-0000-0000-0000-000000000301"
		successID = "00000000-0000-0000-0000-000000000302"
	)
	storage, err := gojourney.NewJourneyStorage(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	journey := &types.JourneyConfiguration{
		Metadata: &nstore.Metadata{ID: "form-privacy"}, Name: "form-privacy", Active: true,
		DefaultExp: 1, StartStepID: formID,
		Steps: map[string]*types.Step{
			formID: {Name: "Profile", StepType: types.FormStep, Config: map[string]any{
				"context": "ctx", "object": "profile",
				"inputs": []any{map[string]any{
					"id": "privateName", "external_id": "profile.name", "type": "string", "required": true,
				}},
				"outcome": map[string]any{"true": successID},
			}},
			successID: {Name: "Success", StepType: types.SuccessStep, Config: map[string]any{}},
		},
	}
	if err := storage.Save(journey); err != nil {
		t.Fatal(err)
	}
	manager := gojourney.NewManager(&gojourney.JourneyManagerConfig{
		JourneyStorage: storage,
		EncryptKey:     []byte("0123456789abcdef0123456789abcdef"),
		Tokens:         jsonJourneyTokens{},
	})

	response, _, err := manager.InvokeJourney(&types.JourneyExecute{Payload: &types.JourneyPayloadReq{JourneyID: journey.ID}})
	if err != nil || response == nil || len(response.ClientInputs) != 1 {
		t.Fatalf("response=%v err=%v", response, err)
	}
	if response.ClientInputs[0].ID != "" || response.ClientInputs[0].ExternalID != "profile.name" {
		t.Fatalf("public field identity = %+v", response.ClientInputs[0])
	}
	var claims map[string]any
	if err := json.Unmarshal([]byte(response.Jwt), &claims); err != nil {
		t.Fatal(err)
	}
	ctx, _ := claims["ctx"].(map[string]any)
	configs, ok := ctx["client_inputs"].([]any)
	if !ok || len(configs) != 1 {
		t.Fatalf("public client-input definitions = %#v", ctx["client_inputs"])
	}
	publicConfig := configs[0].(map[string]any)
	if _, leaked := publicConfig["id"]; leaked {
		t.Fatal("private Form id leaked into the journey token")
	}
	if _, leaked := publicConfig["journey_step_id"]; leaked {
		t.Fatal("journey_step_id leaked into the journey token")
	}

	// Model a real JSON round trip so the private ID cannot survive in memory.
	data, err := json.Marshal(response.ClientInputs)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"id"`) {
		t.Fatalf("private Form id leaked into response body: %s", data)
	}
	var submitted []*inputs.ClientInput
	if err := json.Unmarshal(data, &submitted); err != nil {
		t.Fatal(err)
	}
	submitted[0].Input = "Ada"
	next, state, err := manager.InvokeJourney(&types.JourneyExecute{Payload: &types.JourneyPayloadReq{Jwt: response.Jwt, ClientInputs: submitted}})
	if err != nil || next != nil || state == nil {
		var clientError any
		if next != nil {
			clientError = next.ClientError
		}
		t.Fatalf("resume=%v client_error=%+v state=%v err=%v", next, clientError, state, err)
	}
	if got := state.GetCtx().Get("profile.privateName").AsStringOr(""); got != "Ada" {
		t.Fatalf("private storage target value = %q", got)
	}

	// The privacy change must not disable encrypted_client_inputs.
	encodedJourney, err := json.Marshal(journey)
	if err != nil {
		t.Fatal(err)
	}
	var encryptedJourney types.JourneyConfiguration
	if err := json.Unmarshal(encodedJourney, &encryptedJourney); err != nil {
		t.Fatal(err)
	}
	encryptedJourney.ID = "form-privacy-encrypted"
	encryptedJourney.EncryptedClientInputs = true
	if err := storage.Save(&encryptedJourney); err != nil {
		t.Fatal(err)
	}
	encryptedResponse, _, err := manager.InvokeJourney(&types.JourneyExecute{Payload: &types.JourneyPayloadReq{JourneyID: encryptedJourney.ID}})
	if err != nil || encryptedResponse == nil {
		t.Fatalf("encrypted response=%v err=%v", encryptedResponse, err)
	}
	var encryptedClaims map[string]any
	if err := json.Unmarshal([]byte(encryptedResponse.Jwt), &encryptedClaims); err != nil {
		t.Fatal(err)
	}
	if encryptedClaims["encrypted_ctx"] == "" {
		t.Fatal("encrypted_client_inputs did not produce encrypted token context")
	}
	if encryptedCtx, ok := encryptedClaims["ctx"].(map[string]any); ok {
		if _, leaked := encryptedCtx["client_inputs"]; leaked {
			t.Fatal("encrypted client-input definitions leaked into public token context")
		}
	}
}
