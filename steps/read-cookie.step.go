package steps

import (
	"errors"
	"strings"

	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type ReadCookie struct {
	BasicStep

	_       struct{} `description:"Reads cookies from the current HTTP request and saves them into context."`
	Name    string   `json:"name,omitempty" description:"Cookie name to read. Empty only when all is true."`
	All     bool     `json:"all,omitempty" default:"false" description:"When true, saves all request cookies as a map of name to values."`
	Context string   `json:"context" enum:"ctx,encCtx,closedCtx,tempCtx" default:"ctx"`
	Output  string   `json:"output" required:"true" minLength:"1" description:"Context path where cookie data will be saved."`
	Outcome struct {
		True     string `json:"true" required:"true" format:"uuid"`
		NotFound string `json:"not_found,omitempty" format:"uuid"`
		Error    string `json:"error,omitempty" format:"uuid"`
	} `json:"outcome" required:"true"`
}

func (*ReadCookie) GetStepType() string { return types.ReadCookieStep }

func (*ReadCookie) Execute(transaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	if transaction == nil || transaction.Request == nil || transaction.State == nil {
		return "error", errors.New("request is not available")
	}
	ctxName := config.Get("context").AsStringOr("ctx")
	ctx := transaction.State.Get(ctxName)
	if ctx == nil {
		return "error", errors.New("context is not supported")
	}
	output := strings.TrimSpace(config.Get("output").AsStringOr(""))
	if output == "" {
		return "error", errors.New("output is required")
	}
	if config.Get("all").AsBoolOr(false) {
		ctx.Set(output, requestCookiesMap(transaction.Request))
		return "true", nil
	}
	name := strings.TrimSpace(config.Get("name").AsStringOr(""))
	if name == "" {
		return "error", errors.New("cookie name is required")
	}
	cookie, found := transaction.Request.Cookie(name)
	if !found {
		return "not_found", nil
	}
	ctx.Set(output, cookie.Value)
	return "true", nil
}

func requestCookiesMap(request types.RequestAccessor) map[string][]string {
	result := map[string][]string{}
	for _, cookie := range request.CookiesList() {
		if cookie == nil {
			continue
		}
		result[cookie.Name] = append(result[cookie.Name], cookie.Value)
	}
	return result
}

func init() {
	defaultStepRegistry.AddStep(&ReadCookie{}, map[string]map[string]any{
		".": {"x-category": types.OperationalCategory, "x-order": []string{"name", "all", "context", "output", "outcome"}},
	})
}
