package utils_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nitsugaro/go-journey/utils"
)

func TestPromiseAnyReturnsErrorWhenEveryTaskFails(t *testing.T) {
	wantA := errors.New("a")
	wantB := errors.New("b")
	_, err := utils.PromiseAny(context.Background(), time.Second, nil, []func(context.Context) (string, error){
		func(context.Context) (string, error) { return "", wantA },
		func(context.Context) (string, error) { return "", wantB },
	})
	if !errors.Is(err, wantA) || !errors.Is(err, wantB) {
		t.Fatalf("PromiseAny error = %v, want both task errors", err)
	}
}
