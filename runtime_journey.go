package gojourney

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/nitsugaro/go-journey/types"
)

// runtimeJourneyCache keeps immutable, execution-only journey configurations.
// Persisted configurations are never modified, so REST reads and exports keep
// their original placeholders.
type runtimeJourneyCache struct {
	mu      sync.RWMutex
	entries map[string]runtimeJourneyEntry
}

type runtimeJourneyEntry struct {
	version string
	journey *types.JourneyConfiguration
}

// staticEnvironmentCache memoizes successful env resolver results for the
// lifetime of one manager. This gives env.* deployment-static semantics even
// when several journeys or revisions reference the same path.
type staticEnvironmentCache struct {
	mu      sync.Mutex
	values  map[string]json.RawMessage
	pending map[string]*staticEnvironmentCall
}

type staticEnvironmentCall struct {
	done chan struct{}
	raw  json.RawMessage
	err  error
}

func (cache *staticEnvironmentCache) resolve(path string, resolver types.PlaceholderResolver) (any, error) {
	cache.mu.Lock()
	if raw, found := cache.values[path]; found {
		cache.mu.Unlock()
		return decodeStaticEnvironmentValue(raw)
	}
	if call, found := cache.pending[path]; found {
		cache.mu.Unlock()
		<-call.done
		if call.err != nil {
			return nil, call.err
		}
		return decodeStaticEnvironmentValue(call.raw)
	}
	if cache.pending == nil {
		cache.pending = make(map[string]*staticEnvironmentCall)
	}
	call := &staticEnvironmentCall{done: make(chan struct{})}
	cache.pending[path] = call
	cache.mu.Unlock()

	var raw json.RawMessage
	var err error
	if resolver == nil {
		err = fmt.Errorf("placeholder resolver %q is not registered", "env")
	} else {
		var value any
		value, err = resolver(path)
		if err == nil {
			raw, err = json.Marshal(value)
			if err != nil {
				err = fmt.Errorf("env.%s returned a non-JSON value: %w", path, err)
			}
		}
	}

	cache.mu.Lock()
	call.raw = append(json.RawMessage(nil), raw...)
	call.err = err
	delete(cache.pending, path)
	if err == nil {
		if cache.values == nil {
			cache.values = make(map[string]json.RawMessage)
		}
		cache.values[path] = append(json.RawMessage(nil), raw...)
	}
	close(call.done)
	cache.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return decodeStaticEnvironmentValue(raw)
}

func decodeStaticEnvironmentValue(raw json.RawMessage) (any, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func (jm *journeyManager) runtimeJourney(raw *types.JourneyConfiguration) (*types.JourneyConfiguration, error) {
	if raw == nil {
		return nil, fmt.Errorf("journey is nil")
	}
	version, err := runtimeJourneyVersion(raw)
	if err != nil {
		return nil, err
	}
	key := raw.ID
	if key == "" {
		key = version
	}
	jm.runtimeJourneys.mu.RLock()
	entry, found := jm.runtimeJourneys.entries[key]
	jm.runtimeJourneys.mu.RUnlock()
	if found && entry.version == version {
		return entry.journey, nil
	}

	compiled, err := jm.compileRuntimeJourney(raw)
	if err != nil {
		return nil, err
	}
	jm.runtimeJourneys.mu.Lock()
	defer jm.runtimeJourneys.mu.Unlock()
	if current, ok := jm.runtimeJourneys.entries[key]; ok && current.version == version {
		return current.journey, nil
	}
	if jm.runtimeJourneys.entries == nil {
		jm.runtimeJourneys.entries = make(map[string]runtimeJourneyEntry)
	}
	jm.runtimeJourneys.entries[key] = runtimeJourneyEntry{version: version, journey: compiled}
	return compiled, nil
}

func runtimeJourneyVersion(journey *types.JourneyConfiguration) (string, error) {
	if journey.Metadata != nil && journey.Rev != "" {
		return journey.Rev, nil
	}
	raw, err := json.Marshal(journey)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

func (jm *journeyManager) compileRuntimeJourney(raw *types.JourneyConfiguration) (*types.JourneyConfiguration, error) {
	serialized, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var compiled types.JourneyConfiguration
	if err := json.Unmarshal(serialized, &compiled); err != nil {
		return nil, err
	}
	for stepID, step := range compiled.Steps {
		if step == nil {
			continue
		}
		resolved, err := jm.resolveStaticEnvironmentPlaceholders(step.Config)
		if err != nil {
			return nil, fmt.Errorf("step %q static placeholders: %w", stepID, err)
		}
		step.Config = resolved
		// GenerateStepVariables rebuilds templates and offsets after static
		// substitution while preserving explicitly declared variable types.
		if err := types.GenerateStepVariables(step, jm.steps); err != nil {
			return nil, fmt.Errorf("step %q runtime placeholders: %w", stepID, err)
		}
	}
	return &compiled, nil
}

func (jm *journeyManager) resolveStaticEnvironmentPlaceholders(value any) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			resolved, err := jm.resolveStaticEnvironmentPlaceholders(child)
			if err != nil {
				return nil, err
			}
			typed[key] = resolved
		}
		return typed, nil
	case []any:
		for index, child := range typed {
			resolved, err := jm.resolveStaticEnvironmentPlaceholders(child)
			if err != nil {
				return nil, err
			}
			typed[index] = resolved
		}
		return typed, nil
	case string:
		return jm.resolveStaticEnvironmentString(typed)
	default:
		return value, nil
	}
}

func (jm *journeyManager) resolveStaticEnvironmentString(value string) (any, error) {
	matches := configPlaceholderPattern.FindAllStringSubmatchIndex(value, -1)
	if len(matches) == 0 {
		return value, nil
	}
	if len(matches) == 1 && matches[0][0] == 0 && matches[0][1] == len(value) && value[matches[0][2]:matches[0][3]] == "env" {
		return jm.resolveStaticEnvironmentPath(value[matches[0][4]:matches[0][5]])
	}
	var builder strings.Builder
	offset := 0
	for _, match := range matches {
		builder.WriteString(value[offset:match[0]])
		if value[match[2]:match[3]] != "env" {
			builder.WriteString(value[match[0]:match[1]])
		} else {
			resolved, err := jm.resolveStaticEnvironmentPath(value[match[4]:match[5]])
			if err != nil {
				return nil, err
			}
			if resolved != nil {
				builder.WriteString(fmt.Sprint(resolved))
			}
		}
		offset = match[1]
	}
	builder.WriteString(value[offset:])
	return builder.String(), nil
}

func (jm *journeyManager) resolveStaticEnvironmentPath(path string) (any, error) {
	return jm.staticEnv.resolve(path, jm.placeholderResolvers["env"])
}
