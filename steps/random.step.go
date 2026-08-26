package steps

import (
	cryptorand "crypto/rand"
	"fmt"
	"io"
	"math"
	"math/big"
	"sort"
	"strings"

	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type Random struct {
	BasicStep

	_             struct{}           `description:"Selects an outcome according to configured percentage probabilities."`
	Probabilities map[string]float64 `json:"probabilities" required:"true" minProperties:"1" additionalProperties.type:"number"`
	Outcome       map[string]string  `json:"outcome" required:"true"`
	Reader        io.Reader          `json:"-"`
}

func (*Random) GetStepType() string { return types.RandomStep }

func (*Random) VerifyConfig(stepName string, config goutils.TreeMapImpl) error {
	probabilities, err := config.Get("probabilities").AsMap()
	if err != nil || len(probabilities) == 0 {
		return types.StepInvalidConfig(stepName, "probabilities are required")
	}
	outcomes, err := config.Get("outcome").AsMap()
	if err != nil {
		return types.StepInvalidConfig(stepName, "outcome is required")
	}
	if len(outcomes) != len(probabilities) {
		return types.StepInvalidConfig(stepName, "probabilities and outcomes must contain the same names")
	}
	total := 0.0
	dynamic := false
	for name, raw := range probabilities {
		value := goutils.NewTreeMap(raw)
		if strings.Contains(value.AsStringOr(""), "${") {
			dynamic = true
			if _, found := outcomes[name]; !found {
				return types.StepInvalidConfig(stepName, "probability has no matching outcome: "+name)
			}
			continue
		}
		probability := value.AsFloatOr(-1)
		if probability < 0 {
			return types.StepInvalidConfig(stepName, "probability cannot be negative: "+name)
		}
		if _, found := outcomes[name]; !found {
			return types.StepInvalidConfig(stepName, "probability has no matching outcome: "+name)
		}
		total += probability
	}
	if !dynamic && math.Abs(total-100) > 1e-9 {
		return types.StepInvalidConfig(stepName, fmt.Sprintf("probabilities total %.10g, want 100", total))
	}
	return nil
}

func (step *Random) Execute(_ *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	probabilities, err := config.Get("probabilities").AsMap()
	if err != nil || len(probabilities) == 0 {
		return "", types.ErrInvalidStepConfig
	}
	names := make([]string, 0, len(probabilities))
	outcomes, err := config.Get("outcome").AsMap()
	if err != nil || len(outcomes) != len(probabilities) {
		return "", types.ErrInvalidStepConfig
	}
	total := 0.0
	for name, raw := range probabilities {
		probability := goutils.NewTreeMap(raw).AsFloatOr(-1)
		if probability < 0 {
			return "", types.ErrInvalidStepConfig
		}
		names = append(names, name)
		if _, found := outcomes[name]; !found {
			return "", types.ErrInvalidStepConfig
		}
		total += probability
	}
	if math.Abs(total-100) > 1e-9 {
		return "", types.ErrInvalidStepConfig
	}
	sort.Strings(names)
	reader := step.Reader
	if reader == nil {
		reader = cryptorand.Reader
	}
	rawPoint, err := cryptorand.Int(reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	point := float64(rawPoint.Int64()) / 10_000
	cumulative := 0.0
	for _, name := range names {
		cumulative += goutils.NewTreeMap(probabilities[name]).AsFloatOr(0)
		if point < cumulative {
			return name, nil
		}
	}
	return names[len(names)-1], nil
}

func init() {
	defaultStepRegistry.AddStep(&Random{}, map[string]map[string]any{
		".":       {"x-category": types.FlowCategory, "x-order": []string{"probabilities", "outcome"}},
		"outcome": {"x-dynamic-outcome": true},
	})
}
