package types

import (
	"context"
	"time"
)

type EventType string

const (
	EventStarted   EventType = "started"
	EventFinished  EventType = "finished"
	EventFailed    EventType = "failed"
	EventInfo      EventType = "info"
	EventDebug     EventType = "debug"
	EventSuspended EventType = "suspended"
)

type EventSubject struct {
	Type string `json:"type,omitempty"`
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type Event struct {
	Type     EventType      `json:"type"`
	At       time.Time      `json:"at"`
	Message  string         `json:"message,omitempty"`
	Duration time.Duration  `json:"duration,omitempty"`
	Error    error          `json:"-"`
	Subject  EventSubject   `json:"subject,omitempty"`
	Attrs    map[string]any `json:"attrs,omitempty"`
}

type JourneyExecutionStatus string

const (
	JourneyExecutionSucceeded JourneyExecutionStatus = "succeeded"
	JourneyExecutionFailed    JourneyExecutionStatus = "failed"
)

type JourneyExecutionEvent struct {
	Status  JourneyExecutionStatus `json:"status"`
	Journey *JourneyConfiguration  `json:"journey,omitempty"`
	State   *JourneyState          `json:"-"`
	Payload *JourneyPayloadReq     `json:"payload,omitempty"`
	Error   error                  `json:"-"`
}

type JourneyExecutionListener func(*JourneyExecutionEvent)

type Observer interface {
	OnEvent(context.Context, *Event)
}

type ObserverFunc func(context.Context, *Event)

func (fn ObserverFunc) OnEvent(ctx context.Context, event *Event) {
	if fn != nil {
		fn(ctx, event)
	}
}

func EmitEvent(ctx context.Context, observer Observer, event *Event) {
	if observer == nil || event == nil {
		return
	}
	defer func() {
		_ = recover()
	}()
	if event.At.IsZero() {
		event.At = time.Now().UTC()
	}
	if event.At.Location() != time.UTC {
		event.At = event.At.UTC()
	}
	observer.OnEvent(ctx, event)
}

func (transaction *JourneyTransaction) EmitEvent(event *Event) {
	if transaction == nil || event == nil {
		return
	}
	EmitEvent(transaction.Context, transaction.Observer, event)
}
