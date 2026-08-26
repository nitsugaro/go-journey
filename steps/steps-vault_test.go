package steps

import (
	"testing"

	"github.com/nitsugaro/go-journey/inputs"
	"github.com/nitsugaro/go-journey/types"
)

func TestCloneTransactionSharesInteractionState(t *testing.T) {
	state := types.NewJourneyState()
	parent := &types.JourneyTransaction{
		State:               state,
		ClientInputsBuilder: inputs.NewClientInputBuilder(nil, state.GetCtx(), nil),
	}
	parent.InteractionState.Set("parent", "visible")

	child := cloneTransaction(parent)
	if value, found := child.InteractionState.Get("parent"); !found || value != "visible" {
		t.Fatalf("child value=%#v found=%v", value, found)
	}
	child.InteractionState.Set("child", "shared")
	if value, found := parent.InteractionState.Get("child"); !found || value != "shared" {
		t.Fatalf("parent value=%#v found=%v", value, found)
	}
}
