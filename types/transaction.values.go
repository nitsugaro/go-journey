package types

import (
	"fmt"
	"sync"
)

// TransactionValues is a concurrency-safe native scratchpad scoped to one
// JourneyTransaction interaction. It is intentionally absent from journey
// state serialization, expressions, placeholders, scripts, and responses.
//
// The zero value is ready to use. Share returns another handle to the same
// underlying values for child transactions created during the interaction.
type TransactionValues struct {
	once   sync.Once
	shared *transactionValuesShared
}

type transactionValuesShared struct {
	mu     sync.RWMutex
	values map[string]any
}

func NewTransactionValues() TransactionValues {
	return TransactionValues{}
}

func (values *TransactionValues) Set(key string, value any) {
	shared := values.sharedState()
	shared.mu.Lock()
	defer shared.mu.Unlock()
	shared.values[key] = value
}

func (values *TransactionValues) Get(key string) (any, bool) {
	shared := values.sharedState()
	shared.mu.RLock()
	defer shared.mu.RUnlock()
	value, found := shared.values[key]
	return value, found
}

func (values *TransactionValues) Delete(key string) {
	shared := values.sharedState()
	shared.mu.Lock()
	defer shared.mu.Unlock()
	delete(shared.values, key)
}

func (values *TransactionValues) LoadOrStore(key string, value any) (actual any, loaded bool) {
	shared := values.sharedState()
	shared.mu.Lock()
	defer shared.mu.Unlock()
	if actual, loaded = shared.values[key]; loaded {
		return actual, true
	}
	shared.values[key] = value
	return value, false
}

func (values *TransactionValues) Clear() {
	shared := values.sharedState()
	shared.mu.Lock()
	defer shared.mu.Unlock()
	shared.values = map[string]any{}
}

func (values *TransactionValues) Len() int {
	shared := values.sharedState()
	shared.mu.RLock()
	defer shared.mu.RUnlock()
	return len(shared.values)
}

func (values *TransactionValues) Share() TransactionValues {
	return TransactionValues{shared: values.sharedState()}
}

func (values *TransactionValues) sharedState() *transactionValuesShared {
	if values == nil {
		panic("gojourney: nil TransactionValues")
	}
	values.once.Do(func() {
		if values.shared == nil {
			values.shared = &transactionValuesShared{values: map[string]any{}}
		}
	})
	return values.shared
}

// InteractionValue returns a native interaction value only when it exists and
// is assignable to T.
func InteractionValue[T any](transaction *JourneyTransaction, key string) (T, bool) {
	var zero T
	if transaction == nil {
		return zero, false
	}
	value, found := transaction.InteractionState.Get(key)
	if !found {
		return zero, false
	}
	typed, ok := value.(T)
	if !ok {
		return zero, false
	}
	return typed, true
}

// MustInteractionValue returns a typed native interaction value or panics.
// Prefer InteractionValue when a missing value can be handled normally.
func MustInteractionValue[T any](transaction *JourneyTransaction, key string) T {
	value, ok := InteractionValue[T](transaction, key)
	if !ok {
		panic(fmt.Sprintf("gojourney: interaction value %q is missing or has an incompatible type", key))
	}
	return value
}
