package gojourney

import (
	"errors"
	"fmt"

	"github.com/nitsugaro/go-journey/types"
)

/* ######## JOURNEY ERRORS ######## */

var (
	ErrJourney = errors.New("journey error")

	ErrJourneyInvalidPayload = fmt.Errorf("%w - invalid payload:", ErrJourney)
	ErrJourneyAlreadyExists  = fmt.Errorf("%w - already exists:", ErrJourney)
	ErrJourneyNotFound       = fmt.Errorf("%w - not found:", ErrJourney)
	ErrInvalidJourneyToken   = fmt.Errorf("invalid journey token: %w", ErrJourney)
	ErrJourneyInvalidOutcome = fmt.Errorf("invalid outcome: %w", ErrJourney)
	ErrJourneyFailure        = fmt.Errorf("failure: %w", ErrJourney)
	ErrJourneyStateStore     = fmt.Errorf("state persistence failed: %w", ErrJourney)
)

func JourneyInfiniteLoopErr(stepName string) error {
	return fmt.Errorf("%w: starting an infinite loop from step %s", ErrJourney, stepName)
}

func JourneyAlreadyExistsErr(journeyName string) error {
	return fmt.Errorf("%w %s", ErrJourney, journeyName)
}

func JourneyNotFoundErr(journeyName string) error {
	return fmt.Errorf("%w %s", ErrJourney, journeyName)
}

func JourneyInvalidPayloadErr(details string) error {
	return fmt.Errorf("%w %s", ErrJourneyInvalidPayload, details)
}

/* ######## STEPS ERRORS ######## */

var (
	ErrStep              = types.ErrStep
	ErrStepUnsupported   = types.ErrStepUnsupported
	ErrInvalidStepConfig = types.ErrInvalidStepConfig
	ErrInvalidOutcome    = types.ErrInvalidOutcome
	ErrStepNotFound      = types.ErrStepNotFound
)

func StepUnsupported(step string) error {
	return types.StepUnsupported(step)
}

func StepNotFound(step string) error {
	return types.StepNotFound(step)
}

func StepInvalidConfig(step string, detail string) error {
	return types.StepInvalidConfig(step, detail)
}

func StepInvalidOutcome(step string, outcome string) error {
	return types.StepInvalidOutcome(step, outcome)
}
