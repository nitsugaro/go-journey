package types_test

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	journeytypes "github.com/nitsugaro/go-journey/types"
)

type interactionTestValue struct {
	Name string
}

func TestTransactionValuesAreZeroValueSafeAndTyped(t *testing.T) {
	transaction := &journeytypes.JourneyTransaction{}
	expected := &interactionTestValue{Name: "native"}
	transaction.InteractionState.Set("test.value", expected)

	raw, found := transaction.InteractionState.Get("test.value")
	if !found || raw != expected {
		t.Fatalf("raw value=%#v found=%v", raw, found)
	}
	typed, found := journeytypes.InteractionValue[*interactionTestValue](transaction, "test.value")
	if !found || typed != expected {
		t.Fatalf("typed value=%#v found=%v", typed, found)
	}
	if _, found := journeytypes.InteractionValue[string](transaction, "test.value"); found {
		t.Fatal("incompatible requested type must not match")
	}
	if _, found := transaction.InteractionState.Get("missing"); found {
		t.Fatal("missing key reported as found")
	}

	transaction.InteractionState.Delete("test.value")
	if transaction.InteractionState.Len() != 0 {
		t.Fatalf("len after delete=%d", transaction.InteractionState.Len())
	}
}

func TestTransactionValuesShareWithinInteractionAndIsolateNewTransactions(t *testing.T) {
	parent := &journeytypes.JourneyTransaction{}
	parent.InteractionState.Set("shared", "parent")
	childState := parent.InteractionState.Share()
	childState.Set("child", true)

	if value, found := childState.Get("shared"); !found || value != "parent" {
		t.Fatalf("child shared value=%#v found=%v", value, found)
	}
	if value, found := parent.InteractionState.Get("child"); !found || value != true {
		t.Fatalf("parent child value=%#v found=%v", value, found)
	}

	other := &journeytypes.JourneyTransaction{}
	if _, found := other.InteractionState.Get("shared"); found {
		t.Fatal("a new interaction inherited native state")
	}
}

func TestTransactionValuesLoadOrStoreIsConcurrent(t *testing.T) {
	transaction := &journeytypes.JourneyTransaction{}
	var stores atomic.Int32
	var group sync.WaitGroup
	for index := 0; index < 64; index++ {
		group.Add(1)
		go func(value int) {
			defer group.Done()
			actual, loaded := transaction.InteractionState.LoadOrStore("singleton", value)
			if !loaded {
				stores.Add(1)
			}
			if actual == nil {
				t.Error("LoadOrStore returned nil")
			}
		}(index)
	}
	group.Wait()
	if stores.Load() != 1 {
		t.Fatalf("successful stores=%d, want 1", stores.Load())
	}
}

func TestInteractionStateIsNotPersistedOrExposedToExpressions(t *testing.T) {
	transaction := &journeytypes.JourneyTransaction{State: journeytypes.NewJourneyState()}
	transaction.InteractionState.Set("native.secret", "must-not-leak")

	storage, err := transaction.State.MarshalStorageJSON()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(storage), "must-not-leak") || strings.Contains(string(storage), "native.secret") {
		t.Fatalf("interaction state leaked into journey storage: %s", storage)
	}
	bindings := transaction.ExpressionBindings()
	if _, exposed := bindings["interactionState"]; exposed {
		t.Fatal("interaction state was exposed as an expression binding")
	}
}
