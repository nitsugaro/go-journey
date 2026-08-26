package steps

func init() {
	RegisterInstance(&InstanceDefinition{
		Key:          LDAPClientCacheKey,
		Config:       &LDAPClientConfig{},
		Factory:      LDAPClientFactory,
		MaxInstances: 10,
		Description:  "Reusable LDAP connection pool.",
	}, map[string]map[string]any{
		".": {"x-category": "directory", "x-order": []string{
			"urls",
			"host",
			"port",
			"base_dn",
			"use_tls",
			"start_tls",
			"server_name",
			"insecure_skip_verify",
			"root_cas",
			"client_cert",
			"client_key",
			"bind",
			"connect_timeout_seconds",
			"operation_timeout_seconds",
			"pool",
		}},
		"client_cert": {"x-type": "expandable-text"},
		"client_key":  {"x-type": "expandable-text"},
		"bind":        {"x-order": []string{"method", "dn", "password", "username", "realm", "sasl_mechanism", "properties"}},
		"pool":        {"x-order": []string{"max_open", "max_idle", "max_lifetime_seconds"}},
	})
}
