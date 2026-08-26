package steps

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	jcache "github.com/nitsugaro/go-journey/cache"
	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

var Client, err = goutils.NewHttpClient(&goutils.ClientConfig{
	FollowRedirects: true,
	Timeout:         1 * time.Minute,
})

const HTTPClientCacheKey = "http_client"

type HTTPClient interface {
	Request(method, uri string, headers map[string]string, body []byte) (*goutils.Response, error)
	RequestWithContext(ctx context.Context, method, uri string, headers map[string]string, body []byte) (*goutils.Response, error)
}

// HTTPClientFactory reconstructs the default HTTP client implementation from
// persisted go-utils ClientConfig JSON.
func HTTPClientFactory(config json.RawMessage) (any, error) {
	var clientConfig goutils.ClientConfig
	if err := json.Unmarshal(config, &clientConfig); err != nil {
		return nil, err
	}
	return goutils.NewHttpClient(&clientConfig)
}

func setContentType(headers map[string]string, contentType string) {
	switch strings.ToUpper(contentType) {
	case "JSON":
		headers["Content-Type"] = "application/json"

	case "TEXT":
		headers["Content-Type"] = "text/plain; charset=utf-8"

	case "URLENCODED":
		headers["Content-Type"] = "application/x-www-form-urlencoded"

	case "XML":
		headers["Content-Type"] = "application/xml"

	case "FORMDATA":
		headers["Content-Type"] = "multipart/form-data"

	case "OCTETSTREAM":
		headers["Content-Type"] = "application/octet-stream"

	case "HTML":
		headers["Content-Type"] = "text/html; charset=utf-8"

	case "CSV":
		headers["Content-Type"] = "text/csv; charset=utf-8"

	default:
		headers["Content-Type"] = "application/octet-stream"
	}
}

type HttpRequest struct {
	BasicStep
	_ struct{} `description:"Sends an HTTP request and save response on the selected context."`

	HTTPInstance         string            `json:"http_instance" minLength:"1" description:"HTTP Instance ID to use. If empty use default"`
	URI                  string            `json:"uri" required:"true" minLength:"1" description:"URI of the request."`
	Method               string            `json:"method" enum:"GET,POST,PUT,DELETE,HEAD,PATCH,OPTIONS" default:"GET" required:"true" description:"HTTP method."`
	ContentType          string            `json:"content_type" enum:"JSON,TEXT,URLENCODED,XML,FORMDATA,OCTETSTREAM,HTML,CSV" default:"JSON" description:"Body request format."`
	Headers              map[string]string `json:"headers" default:"{}" required:"true" additionalProperties.type:"string"`
	Body                 string            `json:"body"`
	ResponseOutput       string            `json:"response_output" required:"true" description:"Identifier path key where store HTTP response. Must start with context prefix." pattern:"^(ctx|encCtx|closedCtx)(\\.\\w+)+$"`
	ParseJsonResponse    bool              `json:"parse_json" default:"false" description:"Parse JSON response. Saves it to 'jsonBody' key on path."`
	ReExecuteOnChainStep bool              `json:"re_execute_on_chain_step" default:"false" description:"If checked, the node will be re-execute when ChainStep has interactions with Client Inputs."`
	Outcome              struct {
		True string `json:"true" format:"uuid"`
	} `json:"outcome" required:"true"`
}

func (uns *HttpRequest) GetStepType() string {
	return types.HttpRequestStep
}

func (uns *HttpRequest) Execute(journeyTransaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	var client HTTPClient = Client
	if journeyTransaction.CacheManager != nil {
		if configured, ok := journeyTransaction.CacheManager.GetCacheInstance(HTTPClientCacheKey, config.Get("http_instance").AsStringOr(jcache.DefaultInstanceID)); ok {
			if configuredClient, valid := configured.(HTTPClient); valid {
				client = configuredClient
			}
		}
	}
	if client == nil {
		return "", errors.New("HTTP client is not configured")
	}
	closedCtx := journeyTransaction.State.GetClosedCtx()
	uri := config.Get("uri").AsStringOr("")
	method := config.Get("method").AsStringOr("GET")
	contentType := config.Get("content_type").AsStringOr("JSON")
	body := config.Get("body").AsStringOr("")
	headers := map[string]string{}

	if len(journeyTransaction.ChainStepID) != 0 && !config.Get("re_execute_on_chain_step").AsBoolOr(false) {
		key := journeyTransaction.ChainStepID + "." + journeyTransaction.CurrentStepID

		if closedCtx.Get(key).AsBoolOr(false) {
			return "true", nil
		} else {
			closedCtx.Set(key, true)
		}

	}

	err := config.Get("headers").AsStruct(&headers)

	if err != nil {
		headers = map[string]string{}
	}

	if len(body) != 0 {
		setContentType(headers, contentType)
	}

	var res *goutils.Response
	if journeyTransaction.Context == nil {
		res, err = client.Request(method, uri, headers, []byte(body))
	} else {
		res, err = client.RequestWithContext(journeyTransaction.Context, method, uri, headers, []byte(body))
	}

	if err != nil {
		return "true", err
	}

	keyOutput := config.Get("response_output").AsStringOr("httpResponse")

	headersMap := make(map[string]any)
	for k, v := range res.Headers {
		headersMap[strings.ToLower(k)] = v
	}

	responseMap := map[string]any{"status": res.Status, "headers": headersMap, "duration": res.Duration}

	if config.Get("parse_json").AsBoolOr(false) {
		var responseBody any
		if err := json.Unmarshal(res.Body, &responseBody); err != nil {
			responseMap["jsonBody"] = nil
		} else {
			responseMap["jsonBody"] = responseBody
		}
	} else {
		responseMap["rawBody"] = string(res.Body)
	}

	ctx, keyPath := journeyTransaction.State.GetCtxPath(keyOutput)
	if ctx == nil || keyPath == "" {
		return "", types.StepInvalidConfig(journeyTransaction.CurrentStepID, "invalid response_output context path")
	}
	ctx.Set(keyPath, responseMap)

	return "true", nil
}

func init() {
	defaultStepRegistry.AddStep(&HttpRequest{}, map[string]map[string]any{
		".": {
			"x-category": types.ContextCategory,
			"x-order":    []string{"http_instance", "uri", "method", "headers", "content_type", "body", "response_output", "parse_json", "re_execute_on_chain_step", "outcome"},
		},
		"response_output": {
			"x-error": "Value doesn't match pattern: '<CTX>.PATH.TO.KEY.CONTEXT'",
		},
	})
}
