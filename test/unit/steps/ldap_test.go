package steps_test

import (
	"context"
	"errors"
	"testing"

	ldap "github.com/go-ldap/ldap/v3"
	jcache "github.com/nitsugaro/go-journey/cache"
	journeysteps "github.com/nitsugaro/go-journey/steps"
	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type fakeLDAPRepository struct {
	searchResult *journeysteps.LDAPSearchResult
	searchReq    journeysteps.LDAPSearchRequest
	searchErr    error
	bindErr      error
	compare      bool
	compareErr   error
	modifyReq    journeysteps.LDAPModifyRequest
	modifyErr    error
	addReq       journeysteps.LDAPAddRequest
	addErr       error
	deleteDN     string
	deleteErr    error
	modifyDNReq  journeysteps.LDAPModifyDNRequest
	modifyDNErr  error
}

func (repo *fakeLDAPRepository) Search(_ context.Context, request *journeysteps.LDAPSearchRequest) (*journeysteps.LDAPSearchResult, error) {
	if request != nil {
		repo.searchReq = *request
	}
	if repo.searchErr != nil {
		return nil, repo.searchErr
	}
	if repo.searchResult == nil {
		return &journeysteps.LDAPSearchResult{}, nil
	}
	return repo.searchResult, nil
}

func (repo *fakeLDAPRepository) Bind(context.Context, string, string) error {
	return repo.bindErr
}

func (repo *fakeLDAPRepository) Compare(context.Context, string, string, string) (bool, error) {
	return repo.compare, repo.compareErr
}

func (repo *fakeLDAPRepository) Modify(_ context.Context, request *journeysteps.LDAPModifyRequest) error {
	if request != nil {
		repo.modifyReq = *request
	}
	return repo.modifyErr
}

func (repo *fakeLDAPRepository) Add(_ context.Context, request *journeysteps.LDAPAddRequest) error {
	if request != nil {
		repo.addReq = *request
	}
	return repo.addErr
}

func (repo *fakeLDAPRepository) Delete(_ context.Context, dn string) error {
	repo.deleteDN = dn
	return repo.deleteErr
}

func (repo *fakeLDAPRepository) ModifyDN(_ context.Context, request *journeysteps.LDAPModifyDNRequest) error {
	if request != nil {
		repo.modifyDNReq = *request
	}
	return repo.modifyDNErr
}

func ldapTransaction(t *testing.T, repo *fakeLDAPRepository) *types.JourneyTransaction {
	t.Helper()
	transaction := newStepTransaction()
	cacheManager, err := jcache.NewManager(&jcache.ManagerConfig{
		Caches: map[string]jcache.CacheConfig{
			journeysteps.LDAPClientCacheKey: {MaxInstances: 2},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := cacheManager.UpdateRuntimeCacheInstance(journeysteps.LDAPClientCacheKey, "directory", repo, 0); err != nil {
		t.Fatal(err)
	}
	transaction.CacheManager = cacheManager
	return transaction
}

func TestLDAPSearchStoresAttributeArraysAndRoutesFound(t *testing.T) {
	repo := &fakeLDAPRepository{searchResult: &journeysteps.LDAPSearchResult{Entries: []journeysteps.LDAPEntry{{
		DN: "uid=ada,ou=people,dc=example,dc=com",
		Attributes: map[string][]string{
			"uid":      {"ada"},
			"memberOf": {"cn=admins,dc=example,dc=com", "cn=users,dc=example,dc=com"},
		},
	}}}}
	transaction := ldapTransaction(t, repo)
	transaction.State.GetCtx().Set("username", "ada")
	step := &types.Step{StepType: types.LDAPSearchStep, Config: map[string]any{
		"connection": "directory", "base_dn": "ou=people,dc=example,dc=com",
		"filter": "uid=${ctx.username}", "attributes": []any{"uid", "memberOf"},
		"size_limit": 2, "time_limit_seconds": 3, "output": "closedCtx.ldap.user",
		"outcome": map[string]any{"found": "next", "not_found": "missing", "error": "failure"},
	}}
	if err := types.GenerateStepVariables(step, journeysteps.GetDefaultStepRegistry()); err != nil {
		t.Fatal(err)
	}
	outcome, err := types.ExecuteStepConfig(&journeysteps.LDAPSearch{}, transaction, step.Config)
	if err != nil || outcome != "found" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
	if repo.searchReq.Filter != "uid=ada" || repo.searchReq.SizeLimit != 2 || repo.searchReq.TimeLimitSeconds != 3 {
		t.Fatalf("search request = %#v", repo.searchReq)
	}
	if got := transaction.State.GetClosedCtx().Get("ldap.user.count").AsIntOr(0); got != 1 {
		t.Fatalf("count=%d", got)
	}
	entries, err := transaction.State.GetClosedCtx().Get("ldap.user.entries").AsSlice()
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries=%#v err=%v", entries, err)
	}
}

func TestLDAPSearchRoutesNotFound(t *testing.T) {
	transaction := ldapTransaction(t, &fakeLDAPRepository{})
	config := goutils.NewTreeMap(map[string]any{
		"connection": "directory", "filter": "(uid=missing)", "output": "ctx.ldap",
	})
	outcome, err := (&journeysteps.LDAPSearch{}).Execute(transaction, config)
	if err != nil || outcome != "not_found" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
}

func TestLDAPBindRoutesValidInvalidAndError(t *testing.T) {
	transaction := ldapTransaction(t, &fakeLDAPRepository{})
	config := goutils.NewTreeMap(map[string]any{"connection": "directory", "dn": "uid=ada", "password": "secret"})
	if outcome, err := (&journeysteps.LDAPBind{}).Execute(transaction, config); err != nil || outcome != "valid" {
		t.Fatalf("valid outcome=%q err=%v", outcome, err)
	}
	invalid := ldap.NewError(ldap.LDAPResultInvalidCredentials, errors.New("bad credentials"))
	transaction = ldapTransaction(t, &fakeLDAPRepository{bindErr: invalid})
	if outcome, err := (&journeysteps.LDAPBind{}).Execute(transaction, config); err != nil || outcome != "invalid" {
		t.Fatalf("invalid outcome=%q err=%v", outcome, err)
	}
	transaction = ldapTransaction(t, &fakeLDAPRepository{bindErr: errors.New("network")})
	if outcome, err := (&journeysteps.LDAPBind{}).Execute(transaction, config); err != nil || outcome != "error" {
		t.Fatalf("error outcome=%q err=%v", outcome, err)
	}
}

func TestLDAPCompareRoutesBoolean(t *testing.T) {
	transaction := ldapTransaction(t, &fakeLDAPRepository{compare: true})
	config := goutils.NewTreeMap(map[string]any{"connection": "directory", "dn": "uid=ada", "attribute": "memberOf", "value": "cn=admins"})
	if outcome, err := (&journeysteps.LDAPCompare{}).Execute(transaction, config); err != nil || outcome != "true" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
	transaction = ldapTransaction(t, &fakeLDAPRepository{compare: false})
	if outcome, err := (&journeysteps.LDAPCompare{}).Execute(transaction, config); err != nil || outcome != "false" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
}

func TestLDAPWriteStepsRouteExpectedOutcomes(t *testing.T) {
	notFound := ldap.NewError(ldap.LDAPResultNoSuchObject, errors.New("missing"))
	alreadyExists := ldap.NewError(ldap.LDAPResultEntryAlreadyExists, errors.New("exists"))

	modifyRepo := &fakeLDAPRepository{}
	transaction := ldapTransaction(t, modifyRepo)
	modifyConfig := goutils.NewTreeMap(map[string]any{
		"connection": "directory", "dn": "uid=ada",
		"changes": []any{map[string]any{"operation": "replace", "attribute": "mail", "values": []any{"ada@example.com"}}},
	})
	if outcome, err := (&journeysteps.LDAPModify{}).Execute(transaction, modifyConfig); err != nil || outcome != "success" {
		t.Fatalf("modify outcome=%q err=%v", outcome, err)
	}
	if modifyRepo.modifyReq.Changes[0].Values[0] != "ada@example.com" {
		t.Fatalf("modify request=%#v", modifyRepo.modifyReq)
	}

	transaction = ldapTransaction(t, &fakeLDAPRepository{deleteErr: notFound})
	if outcome, err := (&journeysteps.LDAPDelete{}).Execute(transaction, goutils.NewTreeMap(map[string]any{"connection": "directory", "dn": "uid=missing"})); err != nil || outcome != "not_found" {
		t.Fatalf("delete outcome=%q err=%v", outcome, err)
	}

	transaction = ldapTransaction(t, &fakeLDAPRepository{addErr: alreadyExists})
	addConfig := goutils.NewTreeMap(map[string]any{"connection": "directory", "dn": "uid=ada", "attributes": map[string]any{"uid": []any{"ada"}}})
	if outcome, err := (&journeysteps.LDAPAdd{}).Execute(transaction, addConfig); err != nil || outcome != "already_exists" {
		t.Fatalf("add outcome=%q err=%v", outcome, err)
	}

	modifyDNRepo := &fakeLDAPRepository{}
	transaction = ldapTransaction(t, modifyDNRepo)
	modifyDNConfig := goutils.NewTreeMap(map[string]any{"connection": "directory", "dn": "uid=ada", "rdn": "uid=ada2", "delete_old_rdn": true})
	if outcome, err := (&journeysteps.LDAPModifyDN{}).Execute(transaction, modifyDNConfig); err != nil || outcome != "success" {
		t.Fatalf("modifyDN outcome=%q err=%v", outcome, err)
	}
	if modifyDNRepo.modifyDNReq.RDN != "uid=ada2" || !modifyDNRepo.modifyDNReq.DeleteOldRDN {
		t.Fatalf("modifyDN request=%#v", modifyDNRepo.modifyDNReq)
	}
}

func TestLDAPStepsAreRegistered(t *testing.T) {
	registry := journeysteps.GetDefaultStepRegistry()
	for _, stepType := range []string{
		types.LDAPSearchStep, types.LDAPBindStep, types.LDAPCompareStep, types.LDAPModifyStep,
		types.LDAPAddStep, types.LDAPDeleteStep, types.LDAPModifyDNStep,
	} {
		if registry.GetStep(stepType) == nil {
			t.Fatalf("%s is not registered", stepType)
		}
	}
}
