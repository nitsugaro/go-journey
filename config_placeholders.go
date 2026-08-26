package gojourney

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	jcache "github.com/nitsugaro/go-journey/cache"
	"github.com/nitsugaro/go-journey/types"
)

var configPlaceholderPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_-]*)\.([^{}]+)\}`)

// NewCacheConfigPlaceholderResolver resolves placeholders in persisted runtime
// instance configuration before the instance factory is called. The stored JSON
// remains unchanged; only the constructor copy receives resolved values.
func NewCacheConfigPlaceholderResolver(resolvers map[string]types.PlaceholderResolver) jcache.ConfigResolver {
	cloned := clonePlaceholderResolvers(resolvers)
	return func(cacheKey, instanceID string, config json.RawMessage) (json.RawMessage, error) {
		if len(cloned) == 0 || !bytes.Contains(config, []byte("${")) {
			return append(json.RawMessage(nil), config...), nil
		}
		var value any
		decoder := json.NewDecoder(bytes.NewReader(config))
		decoder.UseNumber()
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		resolved, err := resolveConfigPlaceholders(value, cloned)
		if err != nil {
			return nil, fmt.Errorf("%s/%s: %w", cacheKey, instanceID, err)
		}
		return json.Marshal(resolved)
	}
}

func resolveConfigPlaceholders(value any, resolvers map[string]types.PlaceholderResolver) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		for key, raw := range typed {
			resolved, err := resolveConfigPlaceholders(raw, resolvers)
			if err != nil {
				return nil, err
			}
			typed[key] = resolved
		}
		return typed, nil
	case []any:
		for index, raw := range typed {
			resolved, err := resolveConfigPlaceholders(raw, resolvers)
			if err != nil {
				return nil, err
			}
			typed[index] = resolved
		}
		return typed, nil
	case string:
		return resolveConfigStringPlaceholders(typed, resolvers)
	default:
		return value, nil
	}
}

func resolveConfigStringPlaceholders(value string, resolvers map[string]types.PlaceholderResolver) (any, error) {
	matches := configPlaceholderPattern.FindAllStringSubmatchIndex(value, -1)
	if len(matches) == 0 {
		return value, nil
	}
	if len(matches) == 1 && matches[0][0] == 0 && matches[0][1] == len(value) {
		return resolveConfigPlaceholder(value[matches[0][2]:matches[0][3]], value[matches[0][4]:matches[0][5]], resolvers)
	}
	var builder strings.Builder
	offset := 0
	for _, match := range matches {
		builder.WriteString(value[offset:match[0]])
		resolved, err := resolveConfigPlaceholder(value[match[2]:match[3]], value[match[4]:match[5]], resolvers)
		if err != nil {
			return nil, err
		}
		builder.WriteString(fmt.Sprint(resolved))
		offset = match[1]
	}
	builder.WriteString(value[offset:])
	return builder.String(), nil
}

func resolveConfigPlaceholder(prefix, path string, resolvers map[string]types.PlaceholderResolver) (any, error) {
	if types.IsCtx(prefix) || prefix == "encCtx" || prefix == "closedCtx" || prefix == "tempCtx" {
		return nil, fmt.Errorf("context placeholder %q cannot be used in runtime instance configuration", prefix)
	}
	resolver := resolvers[prefix]
	if resolver == nil {
		return nil, fmt.Errorf("placeholder resolver %q is not configured", prefix)
	}
	return resolver(path)
}
