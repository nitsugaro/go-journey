package steps

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

const HTTPResponseContextKey = "http_response"

type HTTPResponse struct {
	BasicStep

	_           struct{}          `description:"Writes the current resource journey HTTP response and continues through outcome.true."`
	StatusCode  int               `json:"status_code" default:"200" minimum:"100" maximum:"599" description:"HTTP status code returned to the caller."`
	Headers     map[string]string `json:"headers" default:"{}" additionalProperties.type:"string" description:"HTTP headers returned to the caller."`
	ContentType string            `json:"content_type" enum:"JSON,TEXT,HTML,XML,CSV,OCTETSTREAM" default:"JSON" description:"Response body format. Used when Content-Type header is not configured."`
	Body        any               `json:"body" description:"Response body. Supports placeholders and structured JSON values."`
	Outcome     struct {
		True string `json:"true" required:"true" format:"uuid"`
	} `json:"outcome" required:"true"`
}

func (*HTTPResponse) GetStepType() string {
	return types.HTTPResponseStep
}

func (*HTTPResponse) Execute(transaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	if transaction == nil || transaction.Response == nil {
		return "true", errors.New("response is not available")
	}
	statusCode := config.Get("status_code").AsIntOr(200)
	if statusCode < 100 || statusCode > 599 {
		statusCode = 200
	}
	headers := map[string]string{}
	_ = config.Get("headers").AsStruct(&headers)
	for key, value := range headers {
		transaction.Response.Header(key, value)
	}
	body := config.Get("body").AsAnyOr(nil)
	contentType := httpResponseContentType(config.Get("content_type").AsStringOr("JSON"), headers, body)
	transaction.Response.Status(int(statusCode))
	if body != nil {
		data, err := httpResponseBodyBytes(body, contentType)
		if err != nil {
			return "true", err
		}
		transaction.Response.Body(contentType, data)
	}
	return "true", nil
}

func httpResponseContentType(configured string, headers map[string]string, body any) string {
	for key, value := range headers {
		if strings.EqualFold(key, "Content-Type") && strings.TrimSpace(value) != "" {
			return value
		}
	}
	switch strings.ToUpper(strings.TrimSpace(configured)) {
	case "TEXT":
		return "text/plain; charset=utf-8"
	case "HTML":
		return "text/html; charset=utf-8"
	case "XML":
		return "application/xml"
	case "CSV":
		return "text/csv; charset=utf-8"
	case "OCTETSTREAM":
		return "application/octet-stream"
	default:
		if _, ok := body.(string); ok && strings.ToUpper(strings.TrimSpace(configured)) != "JSON" {
			return "text/plain; charset=utf-8"
		}
		return "application/json; charset=utf-8"
	}
}

func httpResponseBodyBytes(body any, contentType string) ([]byte, error) {
	if data, ok := body.([]byte); ok {
		return append([]byte(nil), data...), nil
	}
	if strings.Contains(strings.ToLower(contentType), "json") {
		return json.Marshal(body)
	}
	return []byte(fmt.Sprint(body)), nil
}

func init() {
	defaultStepRegistry.AddStep(&HTTPResponse{}, map[string]map[string]any{
		".": {"x-category": types.FlowCategory, "x-order": []string{"status_code", "headers", "content_type", "body", "outcome"}},
	})
}
