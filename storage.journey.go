package gojourney

import (
	journeysteps "github.com/nitsugaro/go-journey/steps"
	"github.com/nitsugaro/go-journey/types"
	"github.com/nitsugaro/go-nstore"
)

// JourneyConfigurations is the read boundary for journey definitions.
// Implementations must be safe for concurrent Load calls.
type JourneyConfigurations interface {
	Load(id string) (*types.JourneyConfiguration, error)
}

type JourneyStorage struct {
	*nstore.NStorage[*types.JourneyConfiguration]
	Steps *types.Steps
}

func NewJourneyStorage(folder string, registry *types.Steps) (*JourneyStorage, error) {
	storage, err := nstore.New[*types.JourneyConfiguration](folder)
	if err != nil {
		return nil, err
	}
	if registry == nil {
		registry = journeysteps.GetDefaultStepRegistry()
	}
	return &JourneyStorage{NStorage: storage, Steps: registry}, nil
}

// Save generates placeholder descriptors and validates the complete journey
// before delegating to the filesystem store.
func (js *JourneyStorage) Save(journey *types.JourneyConfiguration) error {
	registry := js.Steps
	if registry == nil {
		registry = journeysteps.GetDefaultStepRegistry()
	}
	if err := types.PrepareJourneyConfiguration(journey, registry); err != nil {
		return err
	}
	return js.NStorage.Save(journey)
}

func (js *JourneyStorage) GetJourneyByName(name string) (*types.JourneyConfiguration, bool) {
	journeys, total := js.Query(func(t *types.JourneyConfiguration) bool {
		return t.Name == name
	}, 1)

	if total != 1 {
		return nil, false
	}

	return journeys[0], true
}

func (js *JourneyStorage) GetJourneyByNameRealm(name string, realm string) (*types.JourneyConfiguration, bool) {
	journeys, total := js.Query(func(t *types.JourneyConfiguration) bool {
		return t.Name == name && t.Realm == realm
	}, 1)

	if total != 1 {
		return nil, false
	}

	return journeys[0], true
}
