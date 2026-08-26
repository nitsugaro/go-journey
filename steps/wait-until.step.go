package steps

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/nitsugaro/go-journey/env"
	"github.com/nitsugaro/go-journey/inputs"
	"github.com/nitsugaro/go-journey/types"
	"github.com/nitsugaro/go-ndb"
	goutils "github.com/nitsugaro/go-utils/v2"
	"github.com/nitsugaro/go-utils/v2/crypto"
	"github.com/nitsugaro/go-utils/v2/encoding"
)

type WaitUntil struct {
	BasicStep

	_              struct{} `description:"Suspends execution until an absolute timestamp and rejects waits beyond an optional safety limit."`
	Timestamp      string   `json:"timestamp" required:"true" minLength:"1" description:"RFC 3339 timestamp; supports registered context placeholders."`
	Timezone       string   `json:"timezone,omitempty" default:"UTC" description:"Timezone for timestamps without an explicit offset."`
	MaxWaitSeconds string   `json:"max_wait_seconds,omitempty" description:"Optional maximum wait in seconds; supports registered context placeholders."`
	Outcome        struct {
		Resumed       string `json:"resumed" required:"true" format:"uuid"`
		LimitExceeded string `json:"limit_exceeded" required:"true" format:"uuid"`
		Invalid       string `json:"invalid" required:"true" format:"uuid"`
	} `json:"outcome" required:"true"`
}

func (*WaitUntil) GetStepType() string { return types.WaitUntilStep }

func (*WaitUntil) Execute(transaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	timestamp := config.Get("timestamp").AsStringOr("")
	timezone := config.Get("timezone").AsStringOr("UTC")
	target, err := parseWaitTimestamp(timestamp, timezone)
	if err != nil {
		return "invalid", nil
	}

	maxWaitText := config.Get("max_wait_seconds").AsStringOr("")
	maxWait := int64(0)
	if maxWaitText != "" {
		maxWait, err = strconv.ParseInt(maxWaitText, 10, 64)
		if err != nil || maxWait <= 0 {
			return "invalid", nil
		}
	}

	now := time.Now()
	remaining := target.Sub(now)
	if remaining <= 0 {
		transaction.State.GetClosedCtx().Delete(env.GetSuspendJourneyKey())
		return "resumed", nil
	}
	if maxWait > 0 && remaining > time.Duration(maxWait)*time.Second {
		return "limit_exceeded", nil
	}

	id, err := crypto.GetRandBytes(16)
	if err != nil {
		return "", err
	}
	resumeID := encoding.EncodeBase64URL(id)
	suspendKey := env.GetSuspendJourneyKey()
	closedCtx := transaction.State.GetClosedCtx()
	closedCtx.Set(suspendKey+".journey_id", transaction.Journey.ID)
	closedCtx.Set(suspendKey+".step_id", transaction.CurrentStepID)
	closedCtx.Set(suspendKey+".wait_until", target.Format(time.RFC3339Nano))

	// Keep the server-side state alive through the wait plus its normal journey TTL.
	ttlSeconds := int64(math.Ceil(remaining.Seconds())) + int64(math.Ceil(transaction.State.Exp.Seconds()))
	tempCtx := transaction.State.GetTempCtx()
	tempCtx.Set(suspendKey+".exp", ttlSeconds)
	tempCtx.Set(suspendKey+".resume_id", resumeID)
	if err := transaction.ClientInputsBuilder.AddMessageInput(&inputs.Message{
		ID: transaction.CurrentStepID, StepType: types.WaitUntilStep,
		Value: ndb.M{"resume_id": resumeID, "wait_until": target.Format(time.RFC3339Nano)},
	}); err != nil {
		return "", fmt.Errorf("add wait-until response: %w", err)
	}
	return "resumed", nil
}

func parseWaitTimestamp(value, timezone string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, nil
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, err
	}
	return time.ParseInLocation("2006-01-02T15:04:05", value, location)
}

func init() {
	defaultStepRegistry.AddStep(&WaitUntil{}, map[string]map[string]any{
		".":         {"x-category": types.FlowCategory, "x-order": []string{"timestamp", "timezone", "max_wait_seconds", "outcome"}},
		"timestamp": {"x-type": "scriptable"}, "timezone": {"x-type": "scriptable"}, "max_wait_seconds": {"x-type": "scriptable"},
	})
}
