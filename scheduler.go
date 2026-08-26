package gojourney

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	jcache "github.com/nitsugaro/go-journey/cache"
	"github.com/nitsugaro/go-journey/steps"
	"github.com/nitsugaro/go-journey/types"
	jsrun "github.com/nitsugaro/go-jsruntime/v2"
	"github.com/robfig/cron/v3"
)

type RESTScheduleStorage interface {
	Load(string) (*types.ScheduleConfiguration, error)
	Save(*types.ScheduleConfiguration) error
	Delete(string) error
	ListOfCache() []*types.ScheduleConfiguration
}

var ErrScheduleNoResult = errors.New("schedule produced no cacheable value")

type scheduleTargetResult struct {
	value    any
	hasValue bool
}

type Scheduler struct {
	manager *journeyManager
	storage RESTScheduleStorage
	cron    *cron.Cron

	mu        sync.Mutex
	cronJobs  map[string]cron.EntryID
	intervals map[string]context.CancelFunc
	running   map[string]*scheduleRun
	results   *scheduleResultCache
}

type scheduleRun struct {
	done   chan struct{}
	result *types.ScheduleRunResult
	err    error
}

type scheduleRunContextKey struct{}

type scheduleResultEntry struct {
	value    json.RawMessage
	storedAt time.Time
}

type scheduleResultCache struct {
	mu      sync.RWMutex
	entries map[string]scheduleResultEntry
}

func newScheduler(manager *journeyManager, storage RESTScheduleStorage) *Scheduler {
	return &Scheduler{
		manager: manager, storage: storage, cron: cron.New(),
		cronJobs: map[string]cron.EntryID{}, intervals: map[string]context.CancelFunc{},
		running: map[string]*scheduleRun{}, results: &scheduleResultCache{entries: map[string]scheduleResultEntry{}},
	}
}

func (scheduler *Scheduler) Start() error {
	if scheduler == nil || scheduler.storage == nil {
		return nil
	}
	for _, schedule := range scheduler.storage.ListOfCache() {
		if schedule != nil && schedule.Active {
			if err := scheduler.Schedule(schedule); err != nil {
				return err
			}
		}
	}
	scheduler.cron.Start()
	return nil
}

func (scheduler *Scheduler) Stop() {
	if scheduler == nil {
		return
	}
	scheduler.mu.Lock()
	for _, cancel := range scheduler.intervals {
		cancel()
	}
	scheduler.intervals = map[string]context.CancelFunc{}
	scheduler.cronJobs = map[string]cron.EntryID{}
	scheduler.mu.Unlock()
	if scheduler.cron != nil {
		ctx := scheduler.cron.Stop()
		<-ctx.Done()
	}
}

func (scheduler *Scheduler) Schedule(schedule *types.ScheduleConfiguration) error {
	if scheduler == nil || schedule == nil || schedule.Metadata == nil || schedule.ID == "" {
		return nil
	}
	scheduler.Unschedule(schedule.ID)
	if !schedule.Active || scheduleCompleted(schedule) {
		return nil
	}
	switch schedule.Kind {
	case types.ScheduleKindCron:
		id, err := scheduler.cron.AddFunc(scheduleCronSpec(schedule), func() {
			if schedule.StartAt > 0 && time.Now().Before(time.Unix(schedule.StartAt, 0)) {
				return
			}
			_, _ = scheduler.Run(context.Background(), schedule.ID, nil)
		})
		if err != nil {
			return err
		}
		scheduler.mu.Lock()
		scheduler.cronJobs[schedule.ID] = id
		entry := scheduler.cron.Entry(id)
		next := nextCronRunAt(schedule, time.Now())
		if next.IsZero() {
			next = entry.Next
		}
		if !next.IsZero() {
			schedule.NextRunAt = next.Unix()
			_ = scheduler.storage.Save(schedule)
		}
		scheduler.mu.Unlock()
	case types.ScheduleKindInterval:
		ctx, cancel := context.WithCancel(context.Background())
		scheduler.mu.Lock()
		scheduler.intervals[schedule.ID] = cancel
		if schedule.NextRunAt == 0 {
			schedule.NextRunAt = nextIntervalRunAt(schedule, time.Now()).Unix()
			_ = scheduler.storage.Save(schedule)
		}
		scheduler.mu.Unlock()
		go scheduler.runInterval(ctx, schedule.ID)
	}
	return nil
}

func (scheduler *Scheduler) Unschedule(id string) {
	if scheduler == nil || id == "" {
		return
	}
	scheduler.mu.Lock()
	if entry, ok := scheduler.cronJobs[id]; ok {
		scheduler.cron.Remove(entry)
		delete(scheduler.cronJobs, id)
	}
	if cancel, ok := scheduler.intervals[id]; ok {
		cancel()
		delete(scheduler.intervals, id)
	}
	scheduler.mu.Unlock()
}

func (scheduler *Scheduler) Reload(schedule *types.ScheduleConfiguration) error {
	if scheduler == nil || schedule == nil || schedule.ID == "" {
		return nil
	}
	scheduler.Unschedule(schedule.ID)
	return scheduler.Schedule(schedule)
}

func (scheduler *Scheduler) runInterval(ctx context.Context, scheduleID string) {
	for {
		schedule, err := scheduler.storage.Load(scheduleID)
		if err != nil || schedule == nil || !schedule.Active || scheduleCompleted(schedule) {
			return
		}
		delay := time.Until(time.Unix(schedule.NextRunAt, 0))
		if delay <= 0 {
			delay = time.Duration(schedule.IntervalSeconds) * time.Second
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			_, _ = scheduler.Run(context.Background(), scheduleID, nil)
		}
	}
}

func (scheduler *Scheduler) Trigger(ctx context.Context, id string, request *types.ScheduleTriggerRequest) (*types.ScheduleRunResult, error) {
	storedSchedule, err := scheduler.storage.Load(id)
	if err != nil || storedSchedule == nil {
		return nil, ErrJourneyNotFound
	}
	schedule, err := cloneScheduleConfiguration(storedSchedule)
	if err != nil {
		return nil, err
	}
	if !schedule.TriggerEnabled {
		return nil, errors.New("schedule trigger is disabled")
	}
	wait := schedule.TriggerWait
	if request != nil && request.Wait != nil {
		wait = *request.Wait
	}
	if !wait {
		go func() { _, _ = scheduler.Run(context.Background(), id, request) }()
		return &types.ScheduleRunResult{ScheduleID: id, Status: types.ScheduleStatusRunning}, nil
	}
	return scheduler.Run(ctx, id, request)
}

func (scheduler *Scheduler) Run(ctx context.Context, id string, request *types.ScheduleTriggerRequest) (result *types.ScheduleRunResult, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	call, leader := scheduler.beginRun(id)
	if !leader {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-call.done:
			return cloneScheduleRunResult(call.result), call.err
		}
	}
	result, err = scheduler.runOnce(ctx, id, request)
	scheduler.endRun(id, call, result, err)
	return cloneScheduleRunResult(result), err
}

func (scheduler *Scheduler) runOnce(ctx context.Context, id string, request *types.ScheduleTriggerRequest) (result *types.ScheduleRunResult, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("schedule panic: %v", recovered)
			if result == nil {
				result = &types.ScheduleRunResult{ScheduleID: id}
			}
			result.Status = types.ScheduleStatusFailed
			result.Error = err.Error()
		}
	}()
	storedSchedule, err := scheduler.storage.Load(id)
	if err != nil || storedSchedule == nil {
		return nil, ErrJourneyNotFound
	}
	schedule, err := cloneScheduleConfiguration(storedSchedule)
	if err != nil {
		return nil, err
	}
	if scheduleCompleted(schedule) {
		return &types.ScheduleRunResult{ScheduleID: id, Status: schedule.Status, RunCount: schedule.RunCount}, nil
	}
	startedAt := time.Now()
	schedule.Running = true
	schedule.Status = types.ScheduleStatusRunning
	schedule.LastRunAt = startedAt.Unix()
	schedule.LastError = ""
	_ = scheduler.storage.Save(schedule)

	result = &types.ScheduleRunResult{ScheduleID: id, Status: types.ScheduleStatusRunning, StartedAt: startedAt.Unix()}
	ctx = context.WithValue(ctx, scheduleRunContextKey{}, id)
	targetResult, runErr := scheduler.runTarget(ctx, schedule, request)
	finishedAt := time.Now()
	result.FinishedAt = finishedAt.Unix()
	result.Value = targetResult.value
	result.HasValue = targetResult.hasValue
	if runErr == nil && targetResult.hasValue {
		if cacheErr := scheduler.results.set(schedule.Realm, id, targetResult.value); cacheErr != nil {
			runErr = fmt.Errorf("cache schedule result: %w", cacheErr)
		}
	}

	schedule.Running = false
	schedule.RunCount++
	result.RunCount = schedule.RunCount
	if runErr != nil {
		schedule.Status = types.ScheduleStatusFailed
		schedule.LastError = runErr.Error()
		result.Status = types.ScheduleStatusFailed
		result.Error = runErr.Error()
	} else {
		schedule.Status = types.ScheduleStatusSucceeded
		result.Status = types.ScheduleStatusSucceeded
	}
	if schedule.Kind == types.ScheduleKindInterval {
		schedule.NextRunAt = finishedAt.Add(time.Duration(schedule.IntervalSeconds) * time.Second).Unix()
	} else if schedule.Kind == types.ScheduleKindCron {
		next := nextCronRunAt(schedule, finishedAt)
		if !next.IsZero() {
			schedule.NextRunAt = next.Unix()
		}
	}
	if scheduleCompleted(schedule) {
		schedule.Active = false
		schedule.NextRunAt = 0
	} else if schedule.Active {
		schedule.Status = types.ScheduleStatusIdle
	}
	if err := scheduler.storage.Save(schedule); err != nil && runErr == nil {
		runErr = err
	}
	if !schedule.Active {
		scheduler.Unschedule(id)
	}
	return result, runErr
}

func (scheduler *Scheduler) beginRun(id string) (*scheduleRun, bool) {
	scheduler.mu.Lock()
	defer scheduler.mu.Unlock()
	if call, exists := scheduler.running[id]; exists {
		return call, false
	}
	call := &scheduleRun{done: make(chan struct{})}
	scheduler.running[id] = call
	return call, true
}

func (scheduler *Scheduler) endRun(id string, call *scheduleRun, result *types.ScheduleRunResult, err error) {
	scheduler.mu.Lock()
	call.result = cloneScheduleRunResult(result)
	call.err = err
	if scheduler.running[id] == call {
		delete(scheduler.running, id)
	}
	close(call.done)
	scheduler.mu.Unlock()
}

func (scheduler *Scheduler) Get(ctx context.Context, realm, scheduleName string, options types.ScheduleCacheOptions) (any, error) {
	schedule, err := scheduler.scheduleByName(realm, scheduleName)
	if err != nil {
		return nil, err
	}
	value, found, cacheErr := scheduler.results.get(schedule.Realm, schedule.ID, options.MaxAgeSeconds)
	if cacheErr != nil {
		return nil, cacheErr
	}
	if found {
		return value, nil
	}
	stale, staleFound, staleErr := scheduler.results.get(schedule.Realm, schedule.ID, 0)
	value, err = scheduler.refresh(ctx, schedule)
	if err != nil && options.StaleIfError && staleFound && staleErr == nil {
		return stale, nil
	}
	return value, err
}

func (scheduler *Scheduler) Refresh(ctx context.Context, realm, scheduleName string) (any, error) {
	schedule, err := scheduler.scheduleByName(realm, scheduleName)
	if err != nil {
		return nil, err
	}
	return scheduler.refresh(ctx, schedule)
}

func (scheduler *Scheduler) Clear(_ context.Context, realm, scheduleName string) error {
	schedule, err := scheduler.scheduleByName(realm, scheduleName)
	if err != nil {
		return err
	}
	scheduler.results.clear(schedule.Realm, schedule.ID)
	return nil
}

func (scheduler *Scheduler) refresh(ctx context.Context, schedule *types.ScheduleConfiguration) (any, error) {
	if runningID, _ := ctx.Value(scheduleRunContextKey{}).(string); runningID == schedule.ID {
		return nil, errors.New("a schedule cannot refresh its own cached result")
	}
	if scheduleCompleted(schedule) {
		return nil, errors.New("schedule has completed its maximum runs")
	}
	wait := true
	result, err := scheduler.Trigger(ctx, schedule.ID, &types.ScheduleTriggerRequest{Wait: &wait})
	if err != nil {
		return nil, err
	}
	if result == nil || result.Status != types.ScheduleStatusSucceeded {
		if result != nil && result.Error != "" {
			return nil, errors.New(result.Error)
		}
		return nil, errors.New("schedule refresh did not succeed")
	}
	if !result.HasValue {
		return nil, ErrScheduleNoResult
	}
	return cloneScheduleValue(result.Value)
}

func (scheduler *Scheduler) scheduleByName(realm, name string) (*types.ScheduleConfiguration, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("schedule name is required")
	}
	for _, schedule := range scheduler.storage.ListOfCache() {
		if schedule != nil && schedule.Realm == realm && schedule.Name == name {
			return schedule, nil
		}
	}
	return nil, ErrJourneyNotFound
}

func (cache *scheduleResultCache) set(realm, scheduleID string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	cache.mu.Lock()
	cache.entries[scheduleResultKey(realm, scheduleID)] = scheduleResultEntry{value: append(json.RawMessage(nil), raw...), storedAt: time.Now()}
	cache.mu.Unlock()
	return nil
}

func (cache *scheduleResultCache) get(realm, scheduleID string, maxAgeSeconds int64) (any, bool, error) {
	cache.mu.RLock()
	entry, found := cache.entries[scheduleResultKey(realm, scheduleID)]
	cache.mu.RUnlock()
	if !found || (maxAgeSeconds > 0 && time.Since(entry.storedAt) > time.Duration(maxAgeSeconds)*time.Second) {
		return nil, false, nil
	}
	var value any
	if err := json.Unmarshal(entry.value, &value); err != nil {
		return nil, false, err
	}
	return value, true, nil
}

func (cache *scheduleResultCache) clear(realm, scheduleID string) {
	cache.mu.Lock()
	delete(cache.entries, scheduleResultKey(realm, scheduleID))
	cache.mu.Unlock()
}

func scheduleResultKey(realm, scheduleID string) string {
	return realm + "\x00" + scheduleID
}

func cloneScheduleRunResult(result *types.ScheduleRunResult) *types.ScheduleRunResult {
	if result == nil {
		return nil
	}
	cloned := *result
	cloned.Value, _ = cloneScheduleValue(result.Value)
	return &cloned
}

func cloneScheduleConfiguration(schedule *types.ScheduleConfiguration) (*types.ScheduleConfiguration, error) {
	if schedule == nil {
		return nil, nil
	}
	raw, err := json.Marshal(schedule)
	if err != nil {
		return nil, err
	}
	var cloned types.ScheduleConfiguration
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return nil, err
	}
	return &cloned, nil
}

func cloneScheduleValue(value any) (any, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var cloned any
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return nil, err
	}
	return cloned, nil
}

func (scheduler *Scheduler) runTarget(ctx context.Context, schedule *types.ScheduleConfiguration, request *types.ScheduleTriggerRequest) (scheduleTargetResult, error) {
	timeout := schedule.TimeoutSeconds
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
	}
	schedule.Target.Type = normalizeScheduleTargetType(schedule.Target.Type)
	switch schedule.Target.Type {
	case types.ScheduleTargetScript:
		return scheduler.runScript(ctx, schedule, request)
	case types.ScheduleTargetWorkflowJourney:
		return scheduler.runWorkflow(ctx, schedule, request)
	default:
		return scheduleTargetResult{}, fmt.Errorf("unsupported schedule target %q", schedule.Target.Type)
	}
}

func (scheduler *Scheduler) runWorkflow(ctx context.Context, schedule *types.ScheduleConfiguration, request *types.ScheduleTriggerRequest) (scheduleTargetResult, error) {
	journey, err := scheduler.manager.storage.Load(schedule.Target.JourneyID)
	if err != nil || journey == nil {
		return scheduleTargetResult{}, ErrJourneyNotFound
	}
	if types.NormalizeJourneyType(journey.JourneyType) != types.WorkflowJourney {
		return scheduleTargetResult{}, fmt.Errorf("schedule target journey %q is not a workflow journey", schedule.Target.JourneyID)
	}
	initialData := cloneAnyMap(schedule.Target.InitialData)
	if request != nil && len(request.InitialData) != 0 {
		initialData = cloneAnyMap(request.InitialData)
	}
	if initialData == nil {
		initialData = map[string]any{}
	}
	previousResult, found, cacheErr := scheduler.results.get(schedule.Realm, schedule.ID, 0)
	if cacheErr != nil {
		return scheduleTargetResult{}, cacheErr
	}
	if !found {
		previousResult = nil
	}
	initialData["previousResult"] = previousResult
	payload := (&types.JourneyPayloadReq{JourneyID: schedule.Target.JourneyID, InitialData: initialData}).SetRealm(&types.Realm{Name: schedule.Realm})
	_, state, err := scheduler.manager.InvokeJourney(&types.JourneyExecute{
		Context: ctx, IsConfidential: true, Payload: payload, Request: types.NewEmptyRequest(), Response: types.NewMemoryResponse(),
	})
	if err != nil {
		return scheduleTargetResult{}, err
	}
	if state == nil || !state.HasResult() {
		return scheduleTargetResult{}, nil
	}
	return scheduleTargetResult{value: state.GetResult(), hasValue: true}, nil
}

func (scheduler *Scheduler) runScript(ctx context.Context, schedule *types.ScheduleConfiguration, request *types.ScheduleTriggerRequest) (result scheduleTargetResult, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("schedule script panic: %v", recovered)
		}
	}()
	managerValue, hasManager := scheduler.manager.cacheManager.GetCacheInstance(steps.ScriptManagerCacheKey, jcache.DefaultInstanceID)
	storageValue, hasStorage := scheduler.manager.cacheManager.GetCacheInstance(steps.ScriptStorageCacheKey, jcache.DefaultInstanceID)
	manager, managerOK := managerValue.(*jsrun.ScriptManager)
	storage, storageOK := storageValue.(*jsrun.ScriptStorage)
	if !hasManager || !hasStorage || !managerOK || !storageOK {
		return scheduleTargetResult{}, errors.New("script runtime is not configured")
	}
	script, err := storage.Load(schedule.Target.ScriptID)
	if err != nil || script == nil {
		return scheduleTargetResult{}, jsrun.ErrScriptNotFound
	}
	if steps.NormalizeScriptType(script.Type) != steps.ScheduleScript {
		return scheduleTargetResult{}, fmt.Errorf("schedule target script %q must be type %q", schedule.Target.ScriptID, steps.ScheduleScript)
	}
	args := cloneAnyMap(schedule.Target.Args)
	if request != nil && len(request.Args) != 0 {
		args = cloneAnyMap(request.Args)
	}
	timeoutSeconds := int(schedule.TimeoutSeconds)
	if timeoutSeconds <= 0 {
		timeoutSeconds = schedulerScriptUnlimitedTimeoutSeconds
	}
	code, err := script.GetRawCode()
	if err != nil {
		return scheduleTargetResult{}, err
	}
	program, err := manager.CompileScript(script.Name, code)
	if err != nil {
		return scheduleTargetResult{}, fmt.Errorf("compile schedule script %s: %w", script.Name, err)
	}
	previousResult, found, cacheErr := scheduler.results.get(schedule.Realm, schedule.ID, 0)
	if cacheErr != nil {
		return scheduleTargetResult{}, cacheErr
	}
	if !found {
		previousResult = nil
	}
	resultContext := types.NewScheduleResultContext(previousResult)
	bindings, err := steps.ResolvedScheduleScriptBindings(
		ctx,
		scheduler.manager.cacheManager,
		scheduler.manager.observer,
		schedule.Realm,
		script,
		args,
		timeoutSeconds,
		resultContext,
	)
	if err != nil {
		return scheduleTargetResult{}, err
	}
	_, err = manager.ExecuteWithBindings(program, bindings, time.Duration(timeoutSeconds)*time.Second)
	if err != nil {
		return scheduleTargetResult{}, err
	}
	value, hasValue := resultContext.Result()
	if !hasValue {
		return scheduleTargetResult{}, nil
	}
	return scheduleTargetResult{value: value, hasValue: true}, nil
}

const schedulerScriptUnlimitedTimeoutSeconds = 10 * 365 * 24 * 60 * 60

func (scheduler *Scheduler) List() []*types.ScheduleConfiguration {
	if scheduler == nil || scheduler.storage == nil {
		return nil
	}
	items := scheduler.storage.ListOfCache()
	sort.Slice(items, func(i, j int) bool {
		if items[i] == nil || items[j] == nil {
			return items[i] != nil
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
	return items
}

func scheduleCompleted(schedule *types.ScheduleConfiguration) bool {
	return schedule != nil && schedule.MaxRuns > 0 && schedule.RunCount >= schedule.MaxRuns
}

func scheduleCronSpec(schedule *types.ScheduleConfiguration) string {
	if schedule == nil || strings.TrimSpace(schedule.Timezone) == "" || strings.HasPrefix(strings.TrimSpace(schedule.Cron), "CRON_TZ=") {
		if schedule == nil {
			return ""
		}
		return schedule.Cron
	}
	return "CRON_TZ=" + strings.TrimSpace(schedule.Timezone) + " " + strings.TrimSpace(schedule.Cron)
}

func nextIntervalRunAt(schedule *types.ScheduleConfiguration, now time.Time) time.Time {
	if schedule == nil || schedule.IntervalSeconds <= 0 {
		return now
	}
	interval := time.Duration(schedule.IntervalSeconds) * time.Second
	if schedule.StartAt <= 0 {
		return now.Add(interval)
	}
	start := time.Unix(schedule.StartAt, 0)
	if start.After(now) {
		return start
	}
	elapsed := now.Sub(start)
	steps := elapsed/interval + 1
	return start.Add(steps * interval)
}

func nextCronRunAt(schedule *types.ScheduleConfiguration, after time.Time) time.Time {
	if schedule == nil || strings.TrimSpace(schedule.Cron) == "" {
		return time.Time{}
	}
	parsed, err := cron.ParseStandard(scheduleCronSpec(schedule))
	if err != nil {
		return time.Time{}
	}
	base := after
	if schedule.StartAt > 0 {
		start := time.Unix(schedule.StartAt, 0)
		if start.After(base) {
			base = start.Add(-time.Second)
		}
	}
	return parsed.Next(base)
}

func cloneAnyMap(source map[string]any) map[string]any {
	if len(source) == 0 {
		return nil
	}
	target := make(map[string]any, len(source))
	for key, value := range source {
		target[key] = value
	}
	return target
}

func ensureScheduleStorage(config *JourneyManagerConfig) RESTScheduleStorage {
	if config == nil || config.FolderPath == "" {
		return nil
	}
	storage, err := NewScheduleStorage(config.FolderPath + "/schedules")
	if err != nil {
		panic("gojourney: cannot configure schedule storage: " + err.Error())
	}
	return storage
}

func ensureSchedulerRuntime(manager *journeyManager, storage RESTScheduleStorage) {
	if manager == nil || storage == nil {
		return
	}
	manager.scheduler = newScheduler(manager, storage)
	if err := manager.scheduler.Start(); err != nil {
		panic("gojourney: cannot start scheduler: " + err.Error())
	}
	_ = manager.cacheManager.UpdateRuntimeCacheInstance("scheduler", jcache.DefaultInstanceID, manager.scheduler, 0)
	_ = manager.cacheManager.UpdateRuntimeCacheInstance(steps.ScheduleCacheKey, jcache.DefaultInstanceID, manager.scheduler, 0)
}
