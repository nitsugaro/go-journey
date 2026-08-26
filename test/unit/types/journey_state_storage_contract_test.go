package types_test

import (
	"testing"
	"time"

	"github.com/nitsugaro/go-journey/types"
)

type serializedJourneyStateStore struct {
	data map[string][]byte
}

func newSerializedJourneyStateStore() *serializedJourneyStateStore {
	return &serializedJourneyStateStore{data: map[string][]byte{}}
}

func (store *serializedJourneyStateStore) Store(state *types.JourneyState, _ time.Duration) bool {
	data, err := state.MarshalStorageJSON()
	if err != nil {
		return false
	}
	store.data[state.GetID()] = append([]byte(nil), data...)
	return true
}

func (store *serializedJourneyStateStore) Get(id string) (*types.JourneyState, bool) {
	data, found := store.data[id]
	if !found {
		return nil, false
	}
	state, err := types.UnmarshalJourneyStateStorageJSON(data)
	return state, err == nil
}

func (store *serializedJourneyStateStore) GetAndDelete(id string) (*types.JourneyState, bool) {
	state, found := store.Get(id)
	if found {
		delete(store.data, id)
	}
	return state, found
}

func (store *serializedJourneyStateStore) Delete(id string) {
	delete(store.data, id)
}

func TestSerializedJourneyStateStorePreservesClosedContext(t *testing.T) {
	store := newSerializedJourneyStateStore()
	state := types.NewJourneyState()
	state.SetID("resume-id")
	state.GetClosedCtx().Set("private.session", "server-only")

	if !store.Store(state, time.Minute) {
		t.Fatal("state was not stored")
	}
	restored, found := store.GetAndDelete("resume-id")
	if !found {
		t.Fatal("state was not restored")
	}
	if got := restored.GetClosedCtx().Get("private.session").AsStringOr(""); got != "server-only" {
		t.Fatalf("closed ctx not restored from serialized store: %q", got)
	}
	if _, found := store.Get("resume-id"); found {
		t.Fatal("GetAndDelete must remove restored state")
	}
}
