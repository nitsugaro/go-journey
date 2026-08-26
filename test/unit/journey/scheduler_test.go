package journey_test

import (
	"context"
	"testing"

	gojourney "github.com/nitsugaro/go-journey"
	"github.com/nitsugaro/go-journey/types"
	"github.com/nitsugaro/go-nstore"
)

const (
	schedulerWorkflowJourneyID = "00000000-0000-0000-0000-000000000901"
	schedulerEndStepID         = "00000000-0000-0000-0000-000000000902"
	schedulerID                = "00000000-0000-0000-0000-000000000903"
)

func TestSchedulerStoragePersistsAndRestoresSchedules(t *testing.T) {
	folder := t.TempDir()
	storage, err := gojourney.NewScheduleStorage(folder)
	if err != nil {
		t.Fatal(err)
	}
	schedule := workflowSchedule(schedulerID, schedulerWorkflowJourneyID)
	if err := storage.Save(schedule); err != nil {
		t.Fatal(err)
	}

	restored, err := gojourney.NewScheduleStorage(folder)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := restored.Load(schedulerID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name != "workflow-nightly" || loaded.Target.JourneyID != schedulerWorkflowJourneyID {
		t.Fatalf("schedule was not restored correctly: %#v", loaded)
	}
}

func TestSchedulerTriggerRunsWorkflowJourneyAndCompletesMaxRuns(t *testing.T) {
	scheduler, scheduleStorage := newSchedulerTestManager(t)
	if err := scheduleStorage.Save(workflowSchedule(schedulerID, schedulerWorkflowJourneyID)); err != nil {
		t.Fatal(err)
	}
	if err := scheduler.Reload(workflowSchedule(schedulerID, schedulerWorkflowJourneyID)); err != nil {
		t.Fatal(err)
	}

	wait := true
	result, err := scheduler.Trigger(context.Background(), schedulerID, &types.ScheduleTriggerRequest{
		Wait:        &wait,
		InitialData: map[string]any{"source": "manual"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != types.ScheduleStatusSucceeded || result.RunCount != 1 || !result.HasValue {
		t.Fatalf("unexpected run result: %#v", result)
	}
	value, ok := result.Value.(map[string]any)
	if !ok || value["status"] != "done" {
		t.Fatalf("unexpected workflow result: %#v", result.Value)
	}

	loaded, err := scheduleStorage.Load(schedulerID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Active || loaded.RunCount != 1 || loaded.Status != types.ScheduleStatusSucceeded {
		t.Fatalf("schedule max_runs was not applied: %#v", loaded)
	}
}

func newSchedulerTestManager(t *testing.T) (*gojourney.Scheduler, *gojourney.ScheduleStorage) {
	t.Helper()
	journeyStorage, err := gojourney.NewJourneyStorage(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	workflow := &types.JourneyConfiguration{
		Metadata:     &nstore.Metadata{ID: schedulerWorkflowJourneyID},
		Name:         "workflow-end",
		Realm:        "alpha",
		Active:       true,
		Confidential: true,
		DefaultExp:   1,
		JourneyType:  types.WorkflowJourney,
		StartStepID:  schedulerEndStepID,
		Steps: map[string]*types.Step{
			schedulerEndStepID: {
				Name:     "End",
				StepType: types.EndStep,
				Config: map[string]any{
					"result": map[string]any{"status": "done"},
				},
			},
		},
	}
	if err := journeyStorage.Save(workflow); err != nil {
		t.Fatal(err)
	}
	scheduleStorage, err := gojourney.NewScheduleStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	manager := gojourney.NewManager(&gojourney.JourneyManagerConfig{
		JourneyStorage:  journeyStorage,
		ScheduleStorage: scheduleStorage,
		EncryptKey:      []byte("0123456789abcdef0123456789abcdef"),
	})
	scheduler := manager.GetScheduler()
	if scheduler == nil {
		t.Fatal("scheduler was not configured")
	}
	return scheduler, scheduleStorage
}

func workflowSchedule(id, journeyID string) *types.ScheduleConfiguration {
	return &types.ScheduleConfiguration{
		Metadata:        &nstore.Metadata{ID: id},
		Name:            "workflow-nightly",
		Realm:           "alpha",
		Active:          true,
		Kind:            types.ScheduleKindInterval,
		IntervalSeconds: 3600,
		MaxRuns:         1,
		TriggerEnabled:  true,
		TriggerWait:     true,
		Target:          types.ScheduleTarget{Type: types.ScheduleTargetWorkflowJourney, JourneyID: journeyID},
	}
}
