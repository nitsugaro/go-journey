package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync"
	"time"

	goconf "github.com/nitsugaro/go-conf"
	gojourney "github.com/nitsugaro/go-journey"
	"github.com/nitsugaro/go-journey/env"
	"github.com/nitsugaro/go-journey/steps"
	"github.com/nitsugaro/go-journey/types"
)

const journeyFolder = "config/auth/journeys"

type journeyInvoker interface {
	InvokeJourney(*types.JourneyExecute) (*types.JourneyPayloadReq, *types.JourneyState, error)
}

type scenario struct {
	name        string
	journeyID   string
	wantFailure bool
	validate    func(runResult) error
}

type runResult struct {
	state        *types.JourneyState
	duration     time.Duration
	clientErrors int
}

type requestLog struct {
	mu     sync.Mutex
	counts map[string]int
}

func (log *requestLog) record(path string) {
	log.mu.Lock()
	defer log.mu.Unlock()
	log.counts[path]++
}

func (log *requestLog) count(path string) int {
	log.mu.Lock()
	defer log.mu.Unlock()
	return log.counts[path]
}

func main() {
	if err := goconf.LoadConfig(); err != nil {
		fatal("load configuration", err)
	}
	env.SetEnvironment()

	requests, closeServer, err := startScenarioServer()
	if err != nil {
		fatal("start local scenario server", err)
	}
	defer closeServer()

	if err := verifyEveryDefaultStepHasFixture(); err != nil {
		fatal("fixture coverage", err)
	}

	manager := gojourney.NewManager(&gojourney.JourneyManagerConfig{FolderPath: journeyFolder})

	scenarios := []scenario{
		{name: "synchronous context and branching", journeyID: "00000000-0000-0000-0000-000000000002"},
		{
			name:      "typed grouped input validation and token resume",
			journeyID: "00000000-0000-0000-0000-000000000003",
			validate: func(result runResult) error {
				if result.clientErrors != 1 {
					return fmt.Errorf("client validation errors = %d, want 1", result.clientErrors)
				}
				if result.state == nil || result.state.GetCtx().Get("formData.age").AsIntOr(0) != 36 {
					return fmt.Errorf("validated grouped formData was not saved")
				}
				if result.state.GetCtx().Get("normalized.age").AsIntOr(0) != 36 || result.state.GetCtx().Get("normalized.displayName").AsStringOr("") != "User Ada_Dev" {
					return fmt.Errorf("transformed normalized profile was not saved")
				}
				if result.state.GetClosedCtx().Get(env.GetContextKey("user_name")).AsStringOr("") != "Ada_Dev" {
					return fmt.Errorf("user_name input did not set login hint")
				}
				return nil
			},
		},
		{
			name:      "two slow HTTP requests in parallel",
			journeyID: "00000000-0000-0000-0000-000000000004",
			validate: func(result runResult) error {
				if result.duration >= 550*time.Millisecond {
					return fmt.Errorf("took %s; two 300ms calls did not run concurrently", result.duration)
				}
				return nil
			},
		},
		{
			name:      "fire-and-forget audit and analytics webhooks",
			journeyID: "00000000-0000-0000-0000-000000000005",
			validate: func(result runResult) error {
				deadline := time.Now().Add(2 * time.Second)
				for time.Now().Before(deadline) {
					if requests.count("/background/audit") == 1 && requests.count("/background/analytics") == 1 {
						return nil
					}
					time.Sleep(10 * time.Millisecond)
				}
				return fmt.Errorf("background webhooks were not both delivered")
			},
		},
		{name: "external suspend and resume callback", journeyID: "00000000-0000-0000-0000-000000000006"},
		{name: "expected business failure", journeyID: "00000000-0000-0000-0000-000000000007", wantFailure: true},
		{name: "parent and child journey call stack", journeyID: "00000000-0000-0000-0000-000000000000", wantFailure: true},
	}

	failed := false
	for _, item := range scenarios {
		result, err := runScenario(manager, item.journeyID)
		if item.wantFailure && errors.Is(err, gojourney.ErrJourneyFailure) {
			err = nil
		}
		if err == nil && item.validate != nil {
			err = item.validate(result)
		}
		if err != nil {
			failed = true
			fmt.Printf("FAIL  %-48s %v\n", item.name, err)
		} else {
			fmt.Printf("PASS  %-48s (%s)\n", item.name, result.duration.Round(time.Millisecond))
		}
	}
	if failed {
		os.Exit(1)
	}
}

func runScenario(manager journeyInvoker, journeyID string) (runResult, error) {
	started := time.Now()
	payload := &types.JourneyPayloadReq{JourneyID: journeyID}
	result := runResult{}
	invalidGenericInputSubmitted := false
	fmt.Printf("\nEXEC  journey=%s\n", journeyID)

	for invocation := 0; invocation < 12; invocation++ {
		fmt.Printf("  INVOCATION %d request=%s\n", invocation+1, asJSON(payload))
		response, state, err := manager.InvokeJourney(&types.JourneyExecute{Payload: payload})
		result.duration = time.Since(started)
		result.state = state
		if response != nil && response.ClientError != nil {
			result.clientErrors++
		}
		fmt.Printf("  INVOCATION %d response=%s\n", invocation+1, asJSON(response))
		fmt.Printf("  INVOCATION %d state=%s error=%v\n", invocation+1, asJSON(state), err)
		if state != nil {
			fmt.Printf("  INVOCATION %d contexts=%s\n", invocation+1, stateContexts(state))
		}
		if err != nil {
			return result, err
		}
		if response == nil {
			if state == nil {
				return result, fmt.Errorf("journey returned neither response nor state")
			}
			return result, nil
		}

		if response.Jwt == "" && state != nil && state.GetID() != "" {
			payload = &types.JourneyPayloadReq{ResumeID: state.GetID()}
			continue
		}
		for _, input := range response.ClientInputs {
			switch input.StepType {
			case types.FormStep:
				switch input.ExternalID {
				case "profile.username":
					input.Input = "Ada_Dev"
				case "profile.email":
					input.Input = "ada@example.com"
				case "profile.age":
					if !invalidGenericInputSubmitted && response.ClientError == nil {
						input.Input = 12
					} else {
						input.Input = 36
					}
				case "profile.newsletter":
					input.Input = true
				}
			case types.ChoiceStep:
				input.Input = "yes"
			}
		}
		if response.ClientError == nil {
			for _, input := range response.ClientInputs {
				if input.StepType == types.FormStep {
					invalidGenericInputSubmitted = true
					break
				}
			}
		}
		payload = &types.JourneyPayloadReq{Jwt: response.Jwt, ClientInputs: response.ClientInputs}
	}
	return result, fmt.Errorf("journey did not finish after 12 invocations")
}

func asJSON(value any) string {
	if value == nil {
		return "null"
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("<cannot encode: %v>", err)
	}
	return string(data)
}

func stateContexts(state *types.JourneyState) string {
	return asJSON(map[string]any{
		"ctx":         treeMapValue(state.GetCtx()),
		"encCtx":      treeMapValue(state.GetEncryptedCtx()),
		"closedCtx":   treeMapValue(state.GetClosedCtx()),
		"tempCtx":     treeMapValue(state.GetTempCtx()),
		"trackingIDs": state.TrackingsID,
	})
}

func treeMapValue(value interface {
	AsMap() (map[string]any, error)
}) map[string]any {
	if value == nil || (reflect.ValueOf(value).Kind() == reflect.Map && reflect.ValueOf(value).IsNil()) {
		return map[string]any{}
	}
	result, err := value.AsMap()
	if err != nil {
		return map[string]any{}
	}
	return result
}

func startScenarioServer() (*requestLog, func(), error) {
	listener, err := net.Listen("tcp", "127.0.0.1:18080")
	if err != nil {
		return nil, nil, err
	}
	log := &requestLog{counts: map[string]int{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/slow/profile", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		log.record(r.URL.Path)
		writeJSON(w, map[string]any{"name": "Ada"})
	})
	mux.HandleFunc("/slow/permissions", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		log.record(r.URL.Path)
		writeJSON(w, map[string]any{"role": "admin"})
	})
	mux.HandleFunc("/background/audit", func(w http.ResponseWriter, r *http.Request) {
		log.record(r.URL.Path)
		writeJSON(w, map[string]any{"accepted": true})
	})
	mux.HandleFunc("/background/analytics", func(w http.ResponseWriter, r *http.Request) {
		log.record(r.URL.Path)
		writeJSON(w, map[string]any{"accepted": true})
	})
	mux.HandleFunc("/chain/session", func(w http.ResponseWriter, r *http.Request) {
		log.record(r.URL.Path)
		writeJSON(w, map[string]any{"id": "session-42"})
	})
	mux.HandleFunc("/chain/audit", func(w http.ResponseWriter, r *http.Request) {
		log.record(r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		writeJSON(w, map[string]any{"session": body["session"]})
	})
	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(listener) }()
	return log, func() { _ = server.Close() }, nil
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func verifyEveryDefaultStepHasFixture() error {
	covered := map[string]bool{}
	files, err := filepath.Glob(filepath.Join(journeyFolder, "*.json"))
	if err != nil {
		return err
	}
	for _, filename := range files {
		data, err := os.ReadFile(filename)
		if err != nil {
			return err
		}
		var journey types.JourneyConfiguration
		if err := json.Unmarshal(data, &journey); err != nil {
			return fmt.Errorf("%s: %w", filename, err)
		}
		for _, configuredStep := range journey.Steps {
			collectStepTypes(configuredStep, covered)
		}
	}

	missing := []string{}
	for stepType := range steps.GetDefaultStepRegistry().GetSteps() {
		if !covered[stepType] {
			missing = append(missing, stepType)
		}
	}
	sort.Strings(missing)
	if len(missing) != 0 {
		return fmt.Errorf("default steps without a journey fixture: %v", missing)
	}
	return nil
}

func collectStepTypes(configuredStep *types.Step, covered map[string]bool) {
	if configuredStep == nil {
		return
	}
	covered[configuredStep.StepType] = true
	config, ok := configuredStep.Config.(map[string]any)
	if !ok {
		return
	}
	nested, ok := config["steps"].([]any)
	if !ok {
		return
	}
	for _, raw := range nested {
		data, _ := json.Marshal(raw)
		var child types.Step
		if json.Unmarshal(data, &child) == nil {
			collectStepTypes(&child, covered)
		}
	}
}

func fatal(action string, err error) {
	fmt.Printf("FAIL  %s: %v\n", action, err)
	os.Exit(1)
}
