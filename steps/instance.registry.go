package steps

import (
	"encoding/json"
	"strings"
	"sync"

	jcache "github.com/nitsugaro/go-journey/cache"
	"github.com/swaggest/jsonschema-go"
)

type InstanceDefinition struct {
	Key          string
	Config       any
	Factory      jcache.Factory
	MaxInstances int
	MaxSizeBytes int64
	Description  string
	Internal     bool
}

type registeredInstance struct {
	definition InstanceDefinition
	schema     json.RawMessage
}

type InstanceRegistry struct {
	mu          sync.RWMutex
	definitions map[string]registeredInstance
}

var defaultInstanceRegistry = NewInstanceRegistry()

func NewInstanceRegistry() *InstanceRegistry {
	return &InstanceRegistry{definitions: map[string]registeredInstance{}}
}

func RegisterInstance(definition *InstanceDefinition, schemaExtensions map[string]map[string]any) {
	defaultInstanceRegistry.AddInstance(definition, schemaExtensions)
}

func (registry *InstanceRegistry) AddInstance(definition *InstanceDefinition, schemaExtensions map[string]map[string]any) {
	if registry == nil || definition == nil {
		panic("instance definition is required")
	}
	def := *definition
	def.Key = strings.TrimSpace(def.Key)
	if def.Key == "" {
		panic("instance key is required")
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.definitions[def.Key] = registeredInstance{
		definition: def,
		schema:     instanceConfigSchema(def.Config, def.Description, schemaExtensions),
	}
}

func (registry *InstanceRegistry) Apply(manager *jcache.Manager) error {
	if registry == nil || manager == nil {
		return nil
	}
	registry.mu.RLock()
	definitions := make([]registeredInstance, 0, len(registry.definitions))
	for _, definition := range registry.definitions {
		definitions = append(definitions, definition)
	}
	registry.mu.RUnlock()

	for _, registered := range definitions {
		def := registered.definition
		if err := manager.ConfigureCacheIfMissing(def.Key, &jcache.CacheConfig{
			Factory: def.Factory, MaxInstances: def.MaxInstances, MaxSizeBytes: def.MaxSizeBytes,
			Description: def.Description, Schema: append(json.RawMessage(nil), registered.schema...),
			UserConfigurable: def.Factory != nil && !def.Internal,
		}); err != nil {
			return err
		}
	}
	return nil
}

func instanceConfigSchema(value any, description string, extensions map[string]map[string]any) json.RawMessage {
	if value == nil {
		return nil
	}
	reflector := jsonschema.Reflector{}
	schema, err := reflector.Reflect(value)
	if err != nil {
		return nil
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		return nil
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		return nil
	}
	if description != "" {
		document["description"] = description
	}
	applyInstanceSchemaExtensions(document, extensions)
	allowInstanceConfigPlaceholders(document)
	raw, err = json.Marshal(document)
	if err != nil {
		return nil
	}
	return raw
}

func applyInstanceSchemaExtensions(schema map[string]any, extensions map[string]map[string]any) {
	for path, values := range extensions {
		target := instanceSchemaPath(schema, path)
		if target == nil {
			continue
		}
		for key, value := range values {
			target[key] = value
		}
	}
}

func instanceSchemaPath(schema map[string]any, path string) map[string]any {
	path = strings.TrimSpace(path)
	if path == "" || path == "." {
		return schema
	}
	current := schema
	for _, part := range strings.Split(path, ".") {
		properties, _ := current["properties"].(map[string]any)
		next, _ := properties[part].(map[string]any)
		if next == nil {
			return nil
		}
		current = next
	}
	return current
}

func allowInstanceConfigPlaceholders(schema map[string]any) {
	decorateInstanceConfigSchemaChildren(schema)
	properties, _ := schema["properties"].(map[string]any)
	for name, rawProperty := range properties {
		property, ok := rawProperty.(map[string]any)
		if !ok {
			continue
		}
		decorateInstanceConfigSchemaChildren(property)
		properties[name] = map[string]any{"anyOf": []any{property, cloneInstanceConfigPlaceholderSchema()}}
	}
}

func decorateInstanceConfigSchemaChildren(schema map[string]any) {
	if definitions, ok := schema["definitions"].(map[string]any); ok {
		for _, raw := range definitions {
			if child, ok := raw.(map[string]any); ok {
				allowInstanceConfigPlaceholders(child)
			}
		}
	}
	if items, ok := schema["items"].(map[string]any); ok {
		allowInstanceConfigPlaceholders(items)
	}
	if additional, ok := schema["additionalProperties"].(map[string]any); ok {
		allowInstanceConfigPlaceholders(additional)
	}
}

func cloneInstanceConfigPlaceholderSchema() map[string]any {
	return map[string]any{"type": "string", "pattern": `\$\{[^{}.]+\.[^{}]+\}`}
}
