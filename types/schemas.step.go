package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	goconf "github.com/nitsugaro/go-conf"
	jenv "github.com/nitsugaro/go-journey/env"
	validator "github.com/santhosh-tekuri/jsonschema/v5"
	"github.com/swaggest/jsonschema-go"
)

const defaultSchemaBaseURL = "https://localhost:3000"

var schemaBaseURL atomic.Value

func init() {
	schemaBaseURL.Store(defaultSchemaBaseURL)
	goconf.OnLoad(func() {
		schemaBaseURL.Store(jenv.GetOptionalJourneyField("base_url", defaultSchemaBaseURL))
	})
}

func schemaResourceName(name string) string {
	baseURL, _ := schemaBaseURL.Load().(string)
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultSchemaBaseURL
	}
	return strings.TrimRight(baseURL, "/") + "/schemas/" + name + ".json"
}

type SchemaForm struct {
	mu       sync.RWMutex
	compiler *validator.Compiler
	schemas  map[string]*validator.Schema
	raw      map[string][]byte
}

func New() *SchemaForm {
	form := &SchemaForm{
		compiler: validator.NewCompiler(),
		schemas:  make(map[string]*validator.Schema),
		raw:      make(map[string][]byte),
	}
	goconf.OnLoad(form.recompileResources)
	return form
}

// recompileResources applies a base URL loaded after package initialization to
// schemas that were already registered. A failed rebuild leaves the previously
// compiled, valid registry untouched.
func (sf *SchemaForm) recompileResources() {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	compiler := validator.NewCompiler()
	names := make([]string, 0, len(sf.raw))
	for name := range sf.raw {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := compiler.AddResource(schemaResourceName(name), bytes.NewReader(sf.raw[name])); err != nil {
			return
		}
	}
	compiled := make(map[string]*validator.Schema, len(names))
	for _, name := range names {
		schema, err := compiler.Compile(schemaResourceName(name))
		if err != nil {
			return
		}
		compiled[name] = schema
	}
	sf.compiler = compiler
	sf.schemas = compiled
}

func (sf *SchemaForm) AddSchema(v interface{}, extraProperties map[string]map[string]any) error {
	sf.mu.Lock()
	defer sf.mu.Unlock()

	// nombre del struct
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return fmt.Errorf("solo se aceptan structs, no %s", t.Kind())
	}
	name := t.Name()

	ref := jsonschema.Reflector{}
	schemaVal, err := ref.Reflect(v)
	if err != nil {
		return err
	}

	for key := range extraProperties {
		if key == "." {
			schemaVal.ExtraProperties = extraProperties[key]
		} else if expr, ok := schemaVal.Properties[key]; ok {
			expr.TypeObject.WithExtraProperties(extraProperties[key])
		}
	}

	schemaBytes, _ := json.Marshal(schemaVal)
	var schemaDocument map[string]any
	if err := json.Unmarshal(schemaBytes, &schemaDocument); err != nil {
		return err
	}
	allowStepPlaceholders(schemaDocument)
	schemaBytes, _ = json.MarshalIndent(schemaDocument, "", "  ")
	// agregar a compiler
	resourceName := schemaResourceName(name)
	if err := sf.compiler.AddResource(resourceName, strings.NewReader(string(schemaBytes))); err != nil {
		return err
	}

	// compilar
	sch, err := sf.compiler.Compile(resourceName)
	if err != nil {
		return err
	}

	// guardar
	sf.schemas[name] = sch
	sf.raw[name] = schemaBytes
	return nil
}

var placeholderSchema = map[string]any{
	"type":    "string",
	"pattern": `\$\{[^{}.]+\.[^{}]+\}`,
}

// allowStepPlaceholders keeps the original constraint and additionally accepts
// a placeholder-bearing string. Runtime resolution restores the native value
// before a step receives its configuration.
func allowStepPlaceholders(schema map[string]any) {
	decorateSchemaChildren(schema)
	properties, _ := schema["properties"].(map[string]any)
	for name, rawProperty := range properties {
		property, ok := rawProperty.(map[string]any)
		if !ok {
			continue
		}
		decorateSchemaChildren(property)
		if staticStepProperty(name) {
			continue
		}
		properties[name] = map[string]any{
			"anyOf": []any{property, clonePlaceholderSchema()},
		}
	}
}

func decorateSchemaChildren(schema map[string]any) {
	if definitions, ok := schema["definitions"].(map[string]any); ok {
		for _, raw := range definitions {
			if child, ok := raw.(map[string]any); ok {
				allowStepPlaceholders(child)
			}
		}
	}
	if items, ok := schema["items"].(map[string]any); ok {
		allowStepPlaceholders(items)
	}
	if additional, ok := schema["additionalProperties"].(map[string]any); ok {
		allowStepPlaceholders(additional)
	}
}

func staticStepProperty(name string) bool {
	switch name {
	case "outcome", "steps", "step_type", "name", "vars":
		return true
	default:
		return false
	}
}

func clonePlaceholderSchema() map[string]any {
	return map[string]any{"type": placeholderSchema["type"], "pattern": placeholderSchema["pattern"]}
}

func (sf *SchemaForm) GetSchema(name string) ([]byte, bool) {
	sf.mu.RLock()
	defer sf.mu.RUnlock()
	data, ok := sf.raw[name]
	return data, ok
}

func (sf *SchemaForm) AddRawSchema(name string, schemaBytes []byte) error {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	resourceName := schemaResourceName(name)
	if err := sf.compiler.AddResource(resourceName, strings.NewReader(string(schemaBytes))); err != nil {
		return err
	}
	schema, err := sf.compiler.Compile(resourceName)
	if err != nil {
		return err
	}
	sf.schemas[name] = schema
	sf.raw[name] = append([]byte(nil), schemaBytes...)
	return nil
}

func (sf *SchemaForm) Validate(name string, data []byte) error {
	sf.mu.RLock()
	sch, ok := sf.schemas[name]
	sf.mu.RUnlock()
	if !ok {
		return fmt.Errorf("schema %q no encontrado", name)
	}

	var v interface{}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return err
	}

	return sch.Validate(v)
}
