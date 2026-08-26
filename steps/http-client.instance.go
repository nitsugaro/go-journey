package steps

type HTTPClientInstanceConfig struct {
	BaseURL         string            `json:"baseUrl,omitempty" description:"Optional base URL prepended by the HTTP client when relative request URIs are used."`
	Timeout         int64             `json:"timeout,omitempty" minimum:"0" description:"Request timeout in nanoseconds. Use 60000000000 for 60 seconds. Empty or 0 leaves the HTTP client default."`
	SkipVerifySSL   bool              `json:"skipVerifySSL,omitempty" default:"false" description:"Skip TLS server certificate verification. Useful only for controlled development or private environments."`
	TrustedCertPEM  string            `json:"trustedCertPEM,omitempty" description:"Base64-encoded PEM CA/server certificate bundle trusted by this client. Can use config placeholders."`
	ClientCertPEM   string            `json:"clientCertPEM,omitempty" description:"Base64-encoded PEM client certificate used for mutual TLS. Must be set together with clientKeyPEM."`
	ClientKeyPEM    string            `json:"clientKeyPEM,omitempty" description:"Base64-encoded PEM private key used for mutual TLS. Must be set together with clientCertPEM."`
	DefaultHeaders  map[string]string `json:"defaultHeaders,omitempty" additionalProperties.type:"string" description:"Headers added to every request made by this HTTP client instance."`
	FollowRedirects bool              `json:"followRedirects,omitempty" default:"true" description:"Follow HTTP redirects automatically."`
}

func init() {
	RegisterInstance(&InstanceDefinition{
		Key:          HTTPClientCacheKey,
		Config:       &HTTPClientInstanceConfig{},
		Factory:      HTTPClientFactory,
		MaxInstances: 10,
		Description:  "Reusable HTTP client.",
	}, map[string]map[string]any{
		".": {"x-category": "network", "x-order": []string{
			"baseUrl",
			"timeout",
			"followRedirects",
			"defaultHeaders",
			"skipVerifySSL",
			"trustedCertPEM",
			"clientCertPEM",
			"clientKeyPEM",
		}},
		"trustedCertPEM": {"x-type": "expandable-text"},
		"clientCertPEM":  {"x-type": "expandable-text"},
		"clientKeyPEM":   {"x-type": "expandable-text"},
	})
}
