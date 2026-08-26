package steps

func init() {
	RegisterInstance(&InstanceDefinition{
		Key:          JWKCacheKey,
		MaxInstances: 1,
		Description:  "Runtime JWK cache used by JWT verification.",
	}, map[string]map[string]any{
		".": {"x-category": "security"},
	})
}
