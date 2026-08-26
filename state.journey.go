package gojourney

import (
	"sync"
	"time"

	"github.com/nitsugaro/go-journey/types"
	gocache "github.com/patrickmn/go-cache"
)

type JourneyStates interface {
	Store(val *types.JourneyState, exp time.Duration) bool
	Get(ID string) (*types.JourneyState, bool)
	GetAndDelete(ID string) (*types.JourneyState, bool)
	Delete(ID string)
}

type journeyStates struct {
	cache *gocache.Cache
	mu    sync.Mutex
}

func newDefaultJourneyStates(defaultTTL, cleanupInterval time.Duration) *journeyStates {
	return &journeyStates{
		cache: gocache.New(defaultTTL, cleanupInterval),
	}
}

func (sm *journeyStates) Store(val *types.JourneyState, exp time.Duration) bool {
	if val == nil || val.GetID() == "" {
		return false
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.cache.Set(val.GetID(), val, exp)
	return true
}

func (sm *journeyStates) Get(id string) (*types.JourneyState, bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	val, ok := sm.cache.Get(id)
	if !ok {
		return nil, false
	}

	state, ok := val.(*types.JourneyState)
	return state, ok
}

func (sm *journeyStates) GetAndDelete(id string) (*types.JourneyState, bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	val, found := sm.cache.Get(id)
	if !found {
		return nil, false
	}

	sm.cache.Delete(id)
	state, ok := val.(*types.JourneyState)
	return state, ok
}

func (sm *journeyStates) Delete(id string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.cache.Delete(id)
}
