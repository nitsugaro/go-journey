package types

type RequestValuesBinding struct {
	request         RequestAccessor
	source          string
	caseInsensitive bool
}

func NewRequestQueryBinding(request RequestAccessor) *RequestValuesBinding {
	return &RequestValuesBinding{request: request, source: "query"}
}

func NewRequestHeaderBinding(request RequestAccessor) *RequestValuesBinding {
	return &RequestValuesBinding{request: request, source: "header", caseInsensitive: true}
}

func (binding *RequestValuesBinding) First(key string, defaultValue ...string) string {
	values := binding.All(key)
	if len(values) > 0 {
		return values[0]
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return ""
}

func (binding *RequestValuesBinding) All(key string) []string {
	if binding == nil || binding.request == nil {
		return []string{}
	}
	if binding.source == "query" {
		return binding.request.QueryValues(key)
	}
	if binding.source == "header" {
		return binding.request.HeaderValues(key)
	}
	return []string{}
}

func (binding *RequestValuesBinding) Has(key string) bool {
	return len(binding.All(key)) > 0
}
