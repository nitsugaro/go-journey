package steps

import (
	jcache "github.com/nitsugaro/go-journey/cache"
	jwtek "github.com/nitsugaro/go-jwte-manager/v2"
)

func EnsureDefaultCacheConfigurations(manager *jcache.Manager) error {
	if manager == nil {
		return nil
	}
	if err := defaultInstanceRegistry.Apply(manager); err != nil {
		return err
	}
	if jwtek.GetExternalJwkStorage() != nil {
		if _, found := manager.GetCacheInstance(JWKCacheKey, jcache.DefaultInstanceID); !found {
			if err := manager.UpdateRuntimeCacheInstance(JWKCacheKey, jcache.DefaultInstanceID, &jwteManagerJWKCache{}, 0); err != nil {
				return err
			}
		}
	}
	return nil
}
