package journey_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	gojourney "github.com/nitsugaro/go-journey"
	journeysteps "github.com/nitsugaro/go-journey/steps"
	"github.com/nitsugaro/go-journey/types"
	"github.com/nitsugaro/go-nstore"
	goutils "github.com/nitsugaro/go-utils/v2"
)

func TestScheduleCacheCoordinatesRefreshAndCarriesPreviousResult(t *testing.T) {
	producer := &BlockingScheduleResult{started: make(chan struct{}), release: make(chan struct{})}
	noResultProducer := &ScheduleNoResult{}
	registry := types.NewStepRegistry()
	registry.AddStep(producer, nil)
	registry.AddStep(noResultProducer, nil)
	journeyStorage, err := gojourney.NewJourneyStorage(t.TempDir(), registry)
	if err != nil {
		t.Fatal(err)
	}
	saveProducerJourney(t, journeyStorage, "00000000-0000-0000-0000-000000000911", "00000000-0000-0000-0000-000000000912", producer.GetStepType())
	saveProducerJourney(t, journeyStorage, "00000000-0000-0000-0000-000000000914", "00000000-0000-0000-0000-000000000915", noResultProducer.GetStepType())

	scheduleStorage, err := gojourney.NewScheduleStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	manager := gojourney.NewManager(&gojourney.JourneyManagerConfig{
		JourneyStorage: journeyStorage, ScheduleStorage: scheduleStorage, Steps: registry,
		EncryptKey: []byte("0123456789abcdef0123456789abcdef"),
	})
	resultSchedule := workflowSchedule("00000000-0000-0000-0000-000000000913", "00000000-0000-0000-0000-000000000911")
	resultSchedule.Name = "external-token"
	resultSchedule.MaxRuns = 0
	noResultSchedule := workflowSchedule("00000000-0000-0000-0000-000000000916", "00000000-0000-0000-0000-000000000914")
	noResultSchedule.Name = "no-result"
	noResultSchedule.MaxRuns = 0
	if err := scheduleStorage.Save(resultSchedule); err != nil {
		t.Fatal(err)
	}
	if err := scheduleStorage.Save(noResultSchedule); err != nil {
		t.Fatal(err)
	}

	cacheValue, found := manager.GetCacheManager().GetCacheInstance(journeysteps.ScheduleCacheKey, "default")
	if !found {
		t.Fatal("schedule cache runtime instance was not registered")
	}
	cache, ok := cacheValue.(types.ScheduleCache)
	if !ok {
		t.Fatalf("unexpected schedule cache type %T", cacheValue)
	}

	values := make(chan any, 2)
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			value, cacheErr := cache.Get(context.Background(), "alpha", "external-token", types.ScheduleCacheOptions{MaxAgeSeconds: 3600})
			values <- value
			errs <- cacheErr
		}()
	}
	<-producer.started
	time.Sleep(25 * time.Millisecond)
	close(producer.release)
	for range 2 {
		if cacheErr := <-errs; cacheErr != nil {
			t.Fatal(cacheErr)
		}
		assertScheduleCacheToken(t, <-values)
	}
	if calls := producer.calls.Load(); calls != 1 {
		t.Fatalf("concurrent misses executed producer %d times, want 1", calls)
	}

	if _, err := cache.Refresh(context.Background(), "alpha", "external-token"); err != nil {
		t.Fatal(err)
	}
	if err := cache.Clear(context.Background(), "alpha", "external-token"); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Get(context.Background(), "alpha", "external-token", types.ScheduleCacheOptions{}); err != nil {
		t.Fatal(err)
	}
	if calls := producer.calls.Load(); calls != 3 {
		t.Fatalf("Get, Refresh and Get-after-Clear calls=%d, want 3", calls)
	}
	producer.previousMu.Lock()
	previous := append([]any(nil), producer.previous...)
	producer.previousMu.Unlock()
	if len(previous) != 3 || previous[0] != nil || previous[2] != nil {
		t.Fatalf("unexpected previous results: %#v", previous)
	}
	assertScheduleCacheToken(t, previous[1])

	for range 2 {
		if _, err := cache.Get(context.Background(), "alpha", "no-result", types.ScheduleCacheOptions{}); !errors.Is(err, gojourney.ErrScheduleNoResult) {
			t.Fatalf("no-result Get error=%v", err)
		}
	}
	if calls := noResultProducer.calls.Load(); calls != 2 {
		t.Fatalf("producer without result was cached; calls=%d", calls)
	}
}

type BlockingScheduleResult struct {
	Value string `json:"value,omitempty"`

	started    chan struct{}
	release    chan struct{}
	once       sync.Once
	calls      atomic.Int32
	previousMu sync.Mutex
	previous   []any
}

func (*BlockingScheduleResult) GetStepType() string                            { return "BlockingScheduleResult" }
func (*BlockingScheduleResult) EndJourney() bool                               { return true }
func (*BlockingScheduleResult) VerifyConfig(string, goutils.TreeMapImpl) error { return nil }
func (step *BlockingScheduleResult) Execute(transaction *types.JourneyTransaction, _ goutils.TreeMapImpl) (string, error) {
	step.calls.Add(1)
	previous := transaction.State.GetCtx().Get("initial_data.previousResult").AsAnyOr(nil)
	step.previousMu.Lock()
	step.previous = append(step.previous, previous)
	step.previousMu.Unlock()
	step.once.Do(func() { close(step.started) })
	<-step.release
	transaction.State.SetResult(map[string]any{"access_token": "shared-token"})
	return "true", nil
}

type ScheduleNoResult struct {
	Value string `json:"value,omitempty"`
	calls atomic.Int32
}

func (*ScheduleNoResult) GetStepType() string                            { return "ScheduleNoResult" }
func (*ScheduleNoResult) EndJourney() bool                               { return true }
func (*ScheduleNoResult) VerifyConfig(string, goutils.TreeMapImpl) error { return nil }
func (step *ScheduleNoResult) Execute(*types.JourneyTransaction, goutils.TreeMapImpl) (string, error) {
	step.calls.Add(1)
	return "true", nil
}

func saveProducerJourney(t *testing.T, storage *gojourney.JourneyStorage, journeyID, stepID, stepType string) {
	t.Helper()
	if err := storage.Save(&types.JourneyConfiguration{
		Metadata: &nstore.Metadata{ID: journeyID}, Name: stepType, Realm: "alpha", Active: true,
		Confidential: true, DefaultExp: 1, JourneyType: types.WorkflowJourney, StartStepID: stepID,
		Steps: map[string]*types.Step{stepID: {Name: stepType, StepType: stepType, Config: map[string]any{}}},
	}); err != nil {
		t.Fatal(err)
	}
}

func assertScheduleCacheToken(t *testing.T, value any) {
	t.Helper()
	object, ok := value.(map[string]any)
	if !ok || object["access_token"] != "shared-token" {
		t.Fatalf("unexpected cached value %#v", value)
	}
}
