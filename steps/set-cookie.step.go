package steps

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type SetCookie struct {
	BasicStep

	_       struct{}          `description:"Sets one or more HTTP response cookies."`
	Cookies []SetCookieConfig `json:"cookies" required:"true" minItems:"1"`
	Outcome struct {
		True  string `json:"true" required:"true" format:"uuid"`
		Error string `json:"error,omitempty" format:"uuid"`
	} `json:"outcome" required:"true"`
}

type SetCookieConfig struct {
	Name     string `json:"name" required:"true" minLength:"1"`
	Value    string `json:"value,omitempty"`
	Path     string `json:"path,omitempty"`
	Domain   string `json:"domain,omitempty"`
	Expires  string `json:"expires,omitempty" description:"RFC3339 expiration timestamp. Supports placeholders."`
	MaxAge   int    `json:"max_age,omitempty"`
	Secure   bool   `json:"secure,omitempty"`
	HTTPOnly bool   `json:"http_only,omitempty"`
	SameSite string `json:"same_site,omitempty" enum:"default,lax,strict,none"`
	Delete   bool   `json:"delete,omitempty" description:"When true, sends the cookie as expired."`
}

func (*SetCookie) GetStepType() string { return types.SetCookieStep }

func (*SetCookie) Execute(transaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	if transaction == nil || transaction.Response == nil {
		return "error", errors.New("response is not available")
	}
	var cookies []SetCookieConfig
	if err := config.Get("cookies").AsStruct(&cookies); err != nil || len(cookies) == 0 {
		return "error", errors.New("cookies are required")
	}
	for _, item := range cookies {
		cookie, err := setCookieConfigToHTTPCookie(item)
		if err != nil {
			return "error", err
		}
		transaction.Response.SetCookie(cookie)
	}
	return "true", nil
}

func setCookieConfigToHTTPCookie(config SetCookieConfig) (*http.Cookie, error) {
	name := strings.TrimSpace(config.Name)
	if name == "" {
		return nil, errors.New("cookie name is required")
	}
	cookie := &http.Cookie{
		Name:     name,
		Value:    config.Value,
		Path:     config.Path,
		Domain:   config.Domain,
		MaxAge:   config.MaxAge,
		Secure:   config.Secure,
		HttpOnly: config.HTTPOnly,
		SameSite: setCookieSameSite(config.SameSite),
	}
	if strings.TrimSpace(config.Expires) != "" {
		expires, err := time.Parse(time.RFC3339, strings.TrimSpace(config.Expires))
		if err != nil {
			return nil, err
		}
		cookie.Expires = expires
	}
	if config.Delete {
		cookie.Value = ""
		cookie.MaxAge = -1
		cookie.Expires = time.Unix(1, 0).UTC()
	}
	return cookie, nil
}

func setCookieSameSite(value string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "default":
		return http.SameSiteDefaultMode
	case "lax":
		return http.SameSiteLaxMode
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return 0
	}
}

func init() {
	defaultStepRegistry.AddStep(&SetCookie{}, map[string]map[string]any{
		".": {"x-category": types.OperationalCategory, "x-order": []string{"cookies", "outcome"}},
	})
}
