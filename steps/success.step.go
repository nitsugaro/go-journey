package steps

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type Success struct {
	BasicStep

	_          struct{}       `description:"Journey ends successfully."`
	SuccessURL string         `json:"success_url" description:"Redirect Success URL when user is authenticated."`
	Data       map[string]any `json:"data" description:"Additional data information to return to the client."`
}

func (uns *Success) EndJourney() bool {
	return true
}

func (uns *Success) GetStepType() string {
	return types.SuccessStep
}

func (uns *Success) Execute(jtx *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	if jtx.State.ExistsTracking() {
		_, stepID := jtx.State.GetTracking()
		jtx.State.ClosedCtx.Set(stepID, true)
		return "true", nil
	}

	if jtx.Response == nil {
		return "true", errors.New("response is not available")
	}
	response := map[string]any{}
	if realm := jtx.State.GetRealm(); realm != "" {
		response["realm"] = realm
	}
	if config.IsDefined("success_url") {
		response["success_url"] = config.Get("success_url").AsStringOr("")
	}
	if config.IsDefined("data") {
		response["data"] = config.Get("data").AsAnyOr(nil)
	}
	data, err := json.Marshal(response)
	if err != nil {
		return "true", err
	}
	jtx.Response.Status(http.StatusOK)
	jtx.Response.Body("application/json; charset=utf-8", data)

	return "true", nil
}

func init() {
	defaultStepRegistry.AddStep(&Success{}, map[string]map[string]any{
		".": {"x-category": types.SessionCategory, "x-end-journey": true, "x-order": []string{"success_url", "data"}},
	})
}
