package gojourney

import (
	"sync"

	"github.com/google/uuid"
	"github.com/nitsugaro/go-journey/types"
)

type journeyListenerRegistry struct {
	mu      sync.RWMutex
	success map[string]types.JourneyExecutionListener
	failure map[string]types.JourneyExecutionListener
}

func newJourneyListenerRegistry() *journeyListenerRegistry {
	return &journeyListenerRegistry{
		success: map[string]types.JourneyExecutionListener{},
		failure: map[string]types.JourneyExecutionListener{},
	}
}

func (registry *journeyListenerRegistry) addSuccess(listener types.JourneyExecutionListener) string {
	return registry.add(registry.success, listener)
}

func (registry *journeyListenerRegistry) addFailure(listener types.JourneyExecutionListener) string {
	return registry.add(registry.failure, listener)
}

func (registry *journeyListenerRegistry) add(target map[string]types.JourneyExecutionListener, listener types.JourneyExecutionListener) string {
	if registry == nil || listener == nil {
		return ""
	}
	id := uuid.NewString()
	registry.mu.Lock()
	defer registry.mu.Unlock()
	target[id] = listener
	return id
}

func (registry *journeyListenerRegistry) remove(id string) {
	if registry == nil || id == "" {
		return
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	delete(registry.success, id)
	delete(registry.failure, id)
}

func (registry *journeyListenerRegistry) snapshot(status types.JourneyExecutionStatus) []types.JourneyExecutionListener {
	if registry == nil {
		return nil
	}
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	source := registry.success
	if status == types.JourneyExecutionFailed {
		source = registry.failure
	}
	listeners := make([]types.JourneyExecutionListener, 0, len(source))
	for _, listener := range source {
		if listener != nil {
			listeners = append(listeners, listener)
		}
	}
	return listeners
}

// OnJourneySuccess registers a manager-level listener for terminal successful
// journeys. The listener runs asynchronously and cannot block the journey
// response path.
func (jm *journeyManager) OnJourneySuccess(listener types.JourneyExecutionListener) string {
	if jm == nil || jm.listeners == nil {
		return ""
	}
	return jm.listeners.addSuccess(listener)
}

// OnJourneyFailure registers a manager-level listener for terminal failed
// journeys. The listener runs asynchronously and cannot block the journey
// response path.
func (jm *journeyManager) OnJourneyFailure(listener types.JourneyExecutionListener) string {
	if jm == nil || jm.listeners == nil {
		return ""
	}
	return jm.listeners.addFailure(listener)
}

// RemoveJourneyListener unregisters a listener returned by OnJourneySuccess or
// OnJourneyFailure.
func (jm *journeyManager) RemoveJourneyListener(id string) {
	if jm == nil || jm.listeners == nil {
		return
	}
	jm.listeners.remove(id)
}

func (jm *journeyManager) emitJourneyExecutionEvent(event *types.JourneyExecutionEvent) {
	if jm == nil || jm.listeners == nil || event == nil {
		return
	}
	listeners := jm.listeners.snapshot(event.Status)
	for _, listener := range listeners {
		listener := listener
		eventCopy := *event
		go func() {
			defer func() {
				_ = recover()
			}()
			listener(&eventCopy)
		}()
	}
}
