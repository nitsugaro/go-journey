package steps

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type Failure struct {
	BasicStep

	_          struct{}          `description:"Failure step. Terminates journey without session"`
	FailureURL string            `json:"failure_url" description:"Redirect Failure URL when is not possible to authenticate user."`
	Data       map[string]string `json:"data" description:"Additional data information to return to the client."`
}

func (uns *Failure) EndJourney() bool {
	return true
}

func (uns *Failure) JourneySucceeded() bool {
	return false
}

func (uns *Failure) GetStepType() string {
	return types.FailureStep
}

func (uns *Failure) Execute(jtx *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	if jtx.State.ExistsTracking() {
		_, stepID := jtx.State.GetTracking()
		jtx.State.GetClosedCtx().Set(stepID, false)
		return "true", nil
	}

	if jtx.Response == nil {
		return "true", errors.New("response is not available")
	}
	response := map[string]any{}
	if realm := jtx.State.GetRealm(); realm != "" {
		response["realm"] = realm
	}
	if config.IsDefined("failure_url") {
		response["failure_url"] = config.Get("failure_url").AsStringOr("")
	}
	if config.IsDefined("data") {
		response["data"] = config.Get("data").AsAnyOr(nil)
	}
	jtx.Response.Status(http.StatusUnauthorized)
	data, err := json.Marshal(response)
	if err != nil {
		return "true", err
	}
	jtx.Response.Body("application/json; charset=utf-8", data)

	return "true", nil
}

func init() {
	defaultStepRegistry.AddStep(&Failure{}, map[string]map[string]any{
		".": {"x-category": types.SessionCategory, "x-end-journey": true},
	})
}
