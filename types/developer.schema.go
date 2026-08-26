package types

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/nitsugaro/go-nstore"
	validator "github.com/santhosh-tekuri/jsonschema/v5"
)

type DeveloperSchema struct {
	*nstore.Metadata

	Name        string         `json:"name" binding:"required"`
	Description string         `json:"description,omitempty"`
	Realm       string         `json:"realm,omitempty"`
	Draft       string         `json:"draft,omitempty"`
	Schema      map[string]any `json:"schema" binding:"required"`
}

type DeveloperSchemaProvider interface {
	Load(id string) (*DeveloperSchema, error)
	Validate(id string, data any) error
	ListOfCache() []*DeveloperSchema
}

type DeveloperSchemaCompiler struct {
	mu      sync.RWMutex
	schemas map[string]*validator.Schema
}

func NewDeveloperSchemaCompiler() *DeveloperSchemaCompiler {
	return &DeveloperSchemaCompiler{schemas: map[string]*validator.Schema{}}
}

func (compiler *DeveloperSchemaCompiler) Compile(schema *DeveloperSchema) error {
	if schema == nil {
		return errors.New("schema is required")
	}
	if strings.TrimSpace(schema.Name) == "" {
		return errors.New("schema name is required")
	}
	if len(schema.Schema) == 0 {
		return errors.New("schema body is required")
	}
	raw, err := json.Marshal(schema.Schema)
	if err != nil {
		return err
	}
	resourceName := developerSchemaResourceName(schema)
	jsonCompiler := validator.NewCompiler()
	if err := jsonCompiler.AddResource(resourceName, bytes.NewReader(raw)); err != nil {
		return err
	}
	compiled, err := jsonCompiler.Compile(resourceName)
	if err != nil {
		return err
	}
	compiler.mu.Lock()
	compiler.schemas[schemaKey(schema)] = compiled
	compiler.mu.Unlock()
	return nil
}

func (compiler *DeveloperSchemaCompiler) Delete(schema *DeveloperSchema) {
	if schema == nil {
		return
	}
	compiler.mu.Lock()
	delete(compiler.schemas, schemaKey(schema))
	compiler.mu.Unlock()
}

func (compiler *DeveloperSchemaCompiler) Validate(schema *DeveloperSchema, data any) error {
	if schema == nil {
		return errors.New("schema is required")
	}
	key := schemaKey(schema)
	compiler.mu.RLock()
	compiled := compiler.schemas[key]
	compiler.mu.RUnlock()
	if compiled == nil {
		if err := compiler.Compile(schema); err != nil {
			return err
		}
		compiler.mu.RLock()
		compiled = compiler.schemas[key]
		compiler.mu.RUnlock()
	}
	if compiled == nil {
		return fmt.Errorf("schema %q is not compiled", schema.Name)
	}
	return compiled.Validate(data)
}

func schemaKey(schema *DeveloperSchema) string {
	if schema == nil {
		return ""
	}
	if schema.Metadata != nil && schema.Metadata.ID != "" {
		return schema.Metadata.ID
	}
	return strings.TrimSpace(schema.Realm) + "/" + strings.TrimSpace(schema.Name)
}

func developerSchemaResourceName(schema *DeveloperSchema) string {
	return "journey://developer-schemas/" + strings.ReplaceAll(schemaKey(schema), " ", "%20")
}
