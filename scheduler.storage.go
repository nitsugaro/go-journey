package gojourney

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nitsugaro/go-journey/types"
	"github.com/nitsugaro/go-nstore"
	"github.com/robfig/cron/v3"
)

type ScheduleStorage struct {
	*nstore.NStorage[*types.ScheduleConfiguration]
}

func NewScheduleStorage(folder string) (*ScheduleStorage, error) {
	storage, err := nstore.New[*types.ScheduleConfiguration](folder)
	if err != nil {
		return nil, err
	}
	if err := storage.LoadFromDisk(); err != nil {
		return nil, err
	}

	return &ScheduleStorage{NStorage: storage}, nil
}

func (storage *ScheduleStorage) Save(schedule *types.ScheduleConfiguration) error {
	if err := ValidateScheduleConfiguration(schedule); err != nil {
		return err
	}
	return storage.NStorage.Save(schedule)
}

func ValidateScheduleConfiguration(schedule *types.ScheduleConfiguration) error {
	if schedule == nil {
		return errors.New("schedule is required")
	}
	if strings.TrimSpace(schedule.Name) == "" {
		return errors.New("schedule name is required")
	}
	switch schedule.Kind {
	case types.ScheduleKindCron:
		if strings.TrimSpace(schedule.Cron) == "" {
			return errors.New("schedule cron is required")
		}
		if _, err := cron.ParseStandard(scheduleCronSpec(schedule)); err != nil {
			return fmt.Errorf("invalid cron schedule: %w", err)
		}
	case types.ScheduleKindInterval:
		if schedule.IntervalSeconds <= 0 {
			return errors.New("schedule interval_seconds must be positive")
		}
	default:
		return fmt.Errorf("unsupported schedule kind %q", schedule.Kind)
	}
	if schedule.MaxRuns < 0 {
		return errors.New("schedule max_runs cannot be negative")
	}
	if schedule.StartAt < 0 {
		return errors.New("schedule start_at cannot be negative")
	}
	if schedule.TimeoutSeconds < 0 {
		return errors.New("schedule timeout_seconds cannot be negative")
	}
	schedule.Target.Type = normalizeScheduleTargetType(schedule.Target.Type)
	switch schedule.Target.Type {
	case types.ScheduleTargetScript:
		if strings.TrimSpace(schedule.Target.ScriptID) == "" {
			return errors.New("schedule target.script_id is required")
		}
	case types.ScheduleTargetWorkflowJourney:
		if strings.TrimSpace(schedule.Target.JourneyID) == "" {
			return errors.New("schedule target.journey_id is required")
		}
	default:
		return fmt.Errorf("unsupported schedule target type %q", schedule.Target.Type)
	}
	if schedule.Status == "" {
		if schedule.Active {
			schedule.Status = types.ScheduleStatusIdle
		} else {
			schedule.Status = types.ScheduleStatusDisabled
		}
	}
	return nil
}

func normalizeScheduleTargetType(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case types.ScheduleTargetWorkflowJourney, types.LegacyScheduleTargetWorkflow:
		return types.ScheduleTargetWorkflowJourney
	default:
		return value
	}
}
