package types_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/nitsugaro/go-journey/types"
)

func TestJourneyStateStorageJSONRoundTripPreservesClosedContext(t *testing.T) {
	state := types.NewJourneyState()
	state.SetID("state-id")
	state.SetRealm("alpha")
	state.Exp = 7 * time.Minute
	state.PushTracking("journey-id", "step-id")
	state.GetCtx().Set("public.name", "Ada")
	state.GetEncryptedCtx().Set("secret.level", 3)
	state.GetClosedCtx().Set("server.only", true)
	state.GetTempCtx().Set("request.once", "value")

	data, err := state.MarshalStorageJSON()
	if err != nil {
		t.Fatal(err)
	}
	restored, err := types.UnmarshalJourneyStateStorageJSON(data)
	if err != nil {
		t.Fatal(err)
	}

	if restored.GetID() != "state-id" || restored.GetRealm() != "alpha" || restored.Exp != 7*time.Minute {
		t.Fatalf("metadata not restored: id=%q realm=%q exp=%s", restored.GetID(), restored.GetRealm(), restored.Exp)
	}
	journeyID, stepID := restored.GetTracking()
	if journeyID != "journey-id" || stepID != "step-id" {
		t.Fatalf("tracking not restored: %q %q", journeyID, stepID)
	}
	if got := restored.GetCtx().Get("public.name").AsStringOr(""); got != "Ada" {
		t.Fatalf("ctx not restored: %q", got)
	}
	if got := restored.GetEncryptedCtx().Get("secret.level").AsIntOr(0); got != 3 {
		t.Fatalf("encrypted ctx not restored: %d", got)
	}
	if got := restored.GetClosedCtx().Get("server.only").AsBoolOr(false); !got {
		t.Fatal("closed ctx not restored")
	}
	if got := restored.GetTempCtx().Get("request.once").AsStringOr(""); got != "value" {
		t.Fatalf("temp ctx not restored: %q", got)
	}
}

func TestJourneyStateDefaultJSONDoesNotExposeClosedContext(t *testing.T) {
	state := types.NewJourneyState()
	state.GetClosedCtx().Set("server.only", true)

	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if json.Valid(data) && string(data) != "" {
		var payload map[string]any
		if err := json.Unmarshal(data, &payload); err != nil {
			t.Fatal(err)
		}
		if _, found := payload["closed_ctx"]; found {
			t.Fatal("default JSON must not expose closed_ctx")
		}
	}
}
