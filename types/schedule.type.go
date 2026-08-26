package types

import (
	"context"
	"sync"
	"time"

	"github.com/nitsugaro/go-nstore"
)

type ScheduleResultContext struct {
	PreviousResult any

	mu        sync.Mutex
	result    any
	resultSet bool
}

func NewScheduleResultContext(previousResult any) *ScheduleResultContext {
	return &ScheduleResultContext{PreviousResult: previousResult}
}

func (result *ScheduleResultContext) SetResult(value any) {
	if result == nil {
		return
	}
	result.mu.Lock()
	result.result = value
	result.resultSet = true
	result.mu.Unlock()
}

func (result *ScheduleResultContext) Result() (any, bool) {
	if result == nil {
		return nil, false
	}
	result.mu.Lock()
	defer result.mu.Unlock()
	return result.result, result.resultSet
}

type ScheduleCacheOptions struct {
	MaxAgeSeconds int64
	StaleIfError  bool
}

// ScheduleCache provides transparent access to the last successful result of
// a schedule. Implementations resolve names inside the supplied realm and
// coordinate refreshes so concurrent callers share one execution.
type ScheduleCache interface {
	Get(context.Context, string, string, ScheduleCacheOptions) (any, error)
	Refresh(context.Context, string, string) (any, error)
	Clear(context.Context, string, string) error
}

const (
	ScheduleKindCron     = "cron"
	ScheduleKindInterval = "interval"

	ScheduleTargetScript          = "script"
	ScheduleTargetWorkflowJourney = WorkflowJourney
	LegacyScheduleTargetWorkflow  = LegacyWorkflowJourney

	ScheduleStatusIdle      = "idle"
	ScheduleStatusRunning   = "running"
	ScheduleStatusSucceeded = "succeeded"
	ScheduleStatusFailed    = "failed"
	ScheduleStatusDisabled  = "disabled"
)

type ScheduleConfiguration struct {
	*nstore.Metadata

	Name            string         `json:"name" binding:"required"`
	Description     string         `json:"description,omitempty"`
	Realm           string         `json:"realm,omitempty"`
	Active          bool           `json:"active"`
	Kind            string         `json:"kind" binding:"required"`
	Cron            string         `json:"cron,omitempty"`
	IntervalSeconds int64          `json:"interval_seconds,omitempty"`
	StartAt         int64          `json:"start_at,omitempty"`
	Timezone        string         `json:"timezone,omitempty"`
	MaxRuns         int64          `json:"max_runs,omitempty"`
	TriggerEnabled  bool           `json:"trigger_enabled"`
	TriggerWait     bool           `json:"trigger_wait"`
	TimeoutSeconds  int64          `json:"timeout_seconds,omitempty"`
	Target          ScheduleTarget `json:"target" binding:"required"`

	RunCount  int64  `json:"run_count,omitempty"`
	LastRunAt int64  `json:"last_run_at,omitempty"`
	NextRunAt int64  `json:"next_run_at,omitempty"`
	Status    string `json:"status,omitempty"`
	LastError string `json:"last_error,omitempty"`
	Running   bool   `json:"running,omitempty"`
}

type ScheduleTarget struct {
	Type        string         `json:"type" binding:"required"`
	ScriptID    string         `json:"script_id,omitempty"`
	ScriptType  string         `json:"script_type,omitempty"`
	Args        map[string]any `json:"args,omitempty"`
	JourneyID   string         `json:"journey_id,omitempty"`
	InitialData map[string]any `json:"initial_data,omitempty"`
}

type ScheduleTriggerRequest struct {
	Wait        *bool          `json:"wait,omitempty"`
	Args        map[string]any `json:"args,omitempty"`
	InitialData map[string]any `json:"initial_data,omitempty"`
}

type ScheduleRunResult struct {
	ScheduleID string `json:"schedule_id,omitempty"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
	Value      any    `json:"value,omitempty"`
	HasValue   bool   `json:"has_value"`
	StartedAt  int64  `json:"started_at,omitempty"`
	FinishedAt int64  `json:"finished_at,omitempty"`
	RunCount   int64  `json:"run_count,omitempty"`
}

func (schedule *ScheduleConfiguration) NextRunTime() time.Time {
	if schedule == nil || schedule.NextRunAt == 0 {
		return time.Time{}
	}
	return time.Unix(schedule.NextRunAt, 0)
}
