package gojourney

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"

	"github.com/nitsugaro/go-journey/types"
)

type JSONEventObserver struct {
	Writer io.Writer
	mu     sync.Mutex
}

type jsonEvent struct {
	Type       types.EventType    `json:"type"`
	At         time.Time          `json:"at"`
	Message    string             `json:"message,omitempty"`
	DurationMS int64              `json:"duration_ms,omitempty"`
	Error      string             `json:"error,omitempty"`
	Subject    types.EventSubject `json:"subject,omitempty"`
	Attrs      map[string]any     `json:"attrs,omitempty"`
}

func NewJSONEventObserver(writer io.Writer) *JSONEventObserver {
	if writer == nil {
		writer = os.Stdout
	}
	return &JSONEventObserver{Writer: writer}
}

func (observer *JSONEventObserver) OnEvent(_ context.Context, event *types.Event) {
	if observer == nil || event == nil {
		return
	}
	writer := observer.Writer
	if writer == nil {
		writer = os.Stdout
	}
	payload := jsonEvent{
		Type:       event.Type,
		At:         event.At.UTC(),
		Message:    event.Message,
		DurationMS: event.Duration.Milliseconds(),
		Subject:    event.Subject,
		Attrs:      event.Attrs,
	}
	if event.Error != nil {
		payload.Error = event.Error.Error()
	}
	observer.mu.Lock()
	defer observer.mu.Unlock()
	_ = json.NewEncoder(writer).Encode(payload)
}
