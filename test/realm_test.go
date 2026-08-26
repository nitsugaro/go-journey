package main

import (
	"encoding/json"
	"testing"

	gojourney "github.com/nitsugaro/go-journey"
	"github.com/nitsugaro/go-journey/inputs"
	"github.com/nitsugaro/go-journey/types"
	"github.com/nitsugaro/go-nstore"
)

func TestRealmIsRestoredFromJourneyToken(t *testing.T) {
	const (
		formID    = "00000000-0000-0000-0000-000000000401"
		successID = "00000000-0000-0000-0000-000000000402"
	)
	storage, err := gojourney.NewJourneyStorage(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	journey := &types.JourneyConfiguration{
		Metadata: &nstore.Metadata{ID: "realm-token"}, Name: "realm-token", Active: true,
		DefaultExp: 1, Realm: "alpha", StartStepID: formID,
		Steps: map[string]*types.Step{
			formID: {Name: "Input", StepType: types.FormStep, Config: map[string]any{
				"inputs":  []any{map[string]any{"id": "name", "external_id": "profile.name", "type": "string", "required": true}},
				"outcome": map[string]any{"true": successID},
			}},
			successID: {Name: "Success", StepType: types.SuccessStep, Config: map[string]any{}},
		},
	}
	if err := storage.Save(journey); err != nil {
		t.Fatal(err)
	}
	manager := gojourney.NewManager(&gojourney.JourneyManagerConfig{
		JourneyStorage: storage, EncryptKey: []byte("0123456789abcdef0123456789abcdef"), Tokens: jsonJourneyTokens{},
	})
	realm := &types.Realm{ID: 42, Name: "alpha", Active: true, OAuthAlg: 7, OidcAlg: 9}
	firstPayload := (&types.JourneyPayloadReq{JourneyID: journey.ID}).SetRealm(realm)
	response, _, err := manager.InvokeJourney(&types.JourneyExecute{Payload: firstPayload})
	if err != nil || response == nil {
		t.Fatalf("response=%v err=%v", response, err)
	}
	var tokenState types.JourneyState
	if err := json.Unmarshal([]byte(response.Jwt), &tokenState); err != nil {
		t.Fatal(err)
	}
	if tokenState.GetRealm() != realm.Name {
		t.Fatalf("token realm = %q", tokenState.GetRealm())
	}

	data, _ := json.Marshal(response.ClientInputs)
	var submitted []*inputs.ClientInput
	if err := json.Unmarshal(data, &submitted); err != nil {
		t.Fatal(err)
	}
	submitted[0].Input = "Ada"
	resumePayload := &types.JourneyPayloadReq{Jwt: response.Jwt, ClientInputs: submitted}
	next, state, err := manager.InvokeJourney(&types.JourneyExecute{Payload: resumePayload})
	if err != nil || next != nil || state == nil {
		t.Fatalf("next=%v state=%v err=%v", next, state, err)
	}
	if resumePayload.GetRealm() == nil || resumePayload.GetRealm().Name != realm.Name || state.GetRealm() != realm.Name {
		t.Fatalf("payload realm=%+v state realm=%+v", resumePayload.GetRealm(), state.GetRealm())
	}
}
