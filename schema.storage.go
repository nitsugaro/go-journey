package gojourney

import (
	"errors"
	"strings"

	"github.com/nitsugaro/go-journey/types"
	"github.com/nitsugaro/go-nstore"
)

type DeveloperSchemaStorage struct {
	*nstore.NStorage[*types.DeveloperSchema]
	compiler *types.DeveloperSchemaCompiler
}

func NewDeveloperSchemaStorage(folder string) (*DeveloperSchemaStorage, error) {
	storage, err := nstore.New[*types.DeveloperSchema](folder)
	if err != nil {
		return nil, err
	}
	result := &DeveloperSchemaStorage{NStorage: storage, compiler: types.NewDeveloperSchemaCompiler()}
	if err := result.LoadFromDisk(); err != nil {
		return nil, err
	}
	for _, schema := range result.ListOfCache() {
		if err := result.compiler.Compile(schema); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (storage *DeveloperSchemaStorage) Save(schema *types.DeveloperSchema) error {
	if schema == nil {
		return errors.New("schema is required")
	}
	if strings.TrimSpace(schema.Name) == "" {
		return errors.New("schema name is required")
	}
	if schema.Draft == "" {
		schema.Draft = "draft-07"
	}
	if err := storage.compiler.Compile(schema); err != nil {
		return err
	}
	return storage.NStorage.Save(schema)
}

func (storage *DeveloperSchemaStorage) Delete(id string) error {
	schema, err := storage.Load(id)
	if err == nil {
		storage.compiler.Delete(schema)
	}
	return storage.NStorage.Delete(id)
}

func (storage *DeveloperSchemaStorage) Validate(id string, data any) error {
	schema, err := storage.Load(id)
	if err != nil {
		return err
	}
	return storage.compiler.Validate(schema, data)
}

func (storage *DeveloperSchemaStorage) GetByNameRealm(name string, realm string) (*types.DeveloperSchema, bool) {
	schemas, total := storage.Query(func(item *types.DeveloperSchema) bool {
		return item.Name == name && item.Realm == realm
	}, 1)
	if total != 1 {
		return nil, false
	}
	return schemas[0], true
}
