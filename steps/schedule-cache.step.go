package steps

import (
	"context"

	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type ScheduleCacheGet struct {
	BasicStep

	_             struct{} `description:"Returns a shared schedule result, executing the schedule once when the cached value is absent or too old."`
	Schedule      string   `json:"schedule" required:"true" minLength:"1" description:"Schedule name in the current realm."`
	MaxAgeSeconds int64    `json:"max_age_seconds,omitempty" minimum:"0" default:"0" description:"Maximum accepted result age. Zero accepts any cached result."`
	StaleIfError  bool     `json:"stale_if_error" default:"false" description:"Return the last cached result when a refresh fails."`
	Output        string   `json:"output" required:"true" pattern:"^(ctx|encCtx|closedCtx|tempCtx)(\\.\\w+)+$" description:"Context path where the schedule result is stored."`
	Outcome       struct {
		True  string `json:"true" required:"true" format:"uuid"`
		Error string `json:"error" required:"true" format:"uuid"`
	} `json:"outcome" required:"true"`
}

type ScheduleCacheRefresh struct {
	BasicStep

	_        struct{} `description:"Executes a schedule and atomically replaces its shared cached result."`
	Schedule string   `json:"schedule" required:"true" minLength:"1" description:"Schedule name in the current realm."`
	Output   string   `json:"output" required:"true" pattern:"^(ctx|encCtx|closedCtx|tempCtx)(\\.\\w+)+$" description:"Context path where the refreshed result is stored."`
	Outcome  struct {
		True  string `json:"true" required:"true" format:"uuid"`
		Error string `json:"error" required:"true" format:"uuid"`
	} `json:"outcome" required:"true"`
}

type ScheduleCacheClear struct {
	BasicStep

	_        struct{} `description:"Removes the shared cached result for a schedule without executing it."`
	Schedule string   `json:"schedule" required:"true" minLength:"1" description:"Schedule name in the current realm."`
	Outcome  struct {
		True  string `json:"true" required:"true" format:"uuid"`
		Error string `json:"error" required:"true" format:"uuid"`
	} `json:"outcome" required:"true"`
}

func (*ScheduleCacheGet) GetStepType() string     { return types.ScheduleCacheGetStep }
func (*ScheduleCacheRefresh) GetStepType() string { return types.ScheduleCacheRefreshStep }
func (*ScheduleCacheClear) GetStepType() string   { return types.ScheduleCacheClearStep }

func (*ScheduleCacheGet) Execute(transaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	cache, err := transactionScheduleCache(transaction)
	if err != nil {
		return "error", nil
	}
	value, err := cache.Get(scheduleCacheContext(transaction), transaction.State.GetRealm(), config.Get("schedule").AsStringOr(""), types.ScheduleCacheOptions{
		MaxAgeSeconds: config.Get("max_age_seconds").AsIntOr(0),
		StaleIfError:  config.Get("stale_if_error").AsBoolOr(false),
	})
	if err != nil || setScheduleCacheOutput(transaction, config.Get("output").AsStringOr(""), value) != nil {
		return "error", nil
	}
	return "true", nil
}

func (*ScheduleCacheRefresh) Execute(transaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	cache, err := transactionScheduleCache(transaction)
	if err != nil {
		return "error", nil
	}
	value, err := cache.Refresh(scheduleCacheContext(transaction), transaction.State.GetRealm(), config.Get("schedule").AsStringOr(""))
	if err != nil || setScheduleCacheOutput(transaction, config.Get("output").AsStringOr(""), value) != nil {
		return "error", nil
	}
	return "true", nil
}

func (*ScheduleCacheClear) Execute(transaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	cache, err := transactionScheduleCache(transaction)
	if err != nil {
		return "error", nil
	}
	if err := cache.Clear(scheduleCacheContext(transaction), transaction.State.GetRealm(), config.Get("schedule").AsStringOr("")); err != nil {
		return "error", nil
	}
	return "true", nil
}

func transactionScheduleCache(transaction *types.JourneyTransaction) (types.ScheduleCache, error) {
	if transaction == nil {
		return scheduleCacheInstance(nil)
	}
	return scheduleCacheInstance(transaction.CacheManager)
}

func scheduleCacheContext(transaction *types.JourneyTransaction) context.Context {
	if transaction != nil && transaction.Context != nil {
		return transaction.Context
	}
	return context.Background()
}

func setScheduleCacheOutput(transaction *types.JourneyTransaction, output string, value any) error {
	ctx, path := transaction.State.GetCtxPath(output)
	if ctx == nil || path == "" {
		return types.StepInvalidConfig(transaction.CurrentStepID, "invalid schedule cache output context path")
	}
	ctx.Set(path, value)
	return nil
}

func init() {
	outputError := map[string]any{"x-error": "Value doesn't match pattern: '<CTX>.PATH.TO.KEY.CONTEXT'"}
	defaultStepRegistry.AddStep(&ScheduleCacheGet{}, map[string]map[string]any{
		".":      {"x-category": types.ContextCategory, "x-order": []string{"schedule", "max_age_seconds", "stale_if_error", "output", "outcome"}},
		"output": outputError,
	})
	defaultStepRegistry.AddStep(&ScheduleCacheRefresh{}, map[string]map[string]any{
		".":      {"x-category": types.ContextCategory, "x-order": []string{"schedule", "output", "outcome"}},
		"output": outputError,
	})
	defaultStepRegistry.AddStep(&ScheduleCacheClear{}, map[string]map[string]any{
		".": {"x-category": types.ContextCategory, "x-order": []string{"schedule", "outcome"}},
	})
}
