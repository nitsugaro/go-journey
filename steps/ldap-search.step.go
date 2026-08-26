package steps

import (
	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type LDAPSearch struct {
	BasicStep

	_                struct{} `description:"Searches an LDAP repository through a cache-managed connection pool and stores entries as DN plus attribute arrays."`
	Connection       string   `json:"connection" required:"true" minLength:"1"`
	BaseDN           string   `json:"base_dn,omitempty"`
	Scope            string   `json:"scope" enum:"base,one,sub" default:"sub"`
	DerefAliases     string   `json:"deref_aliases" enum:"never,searching,finding,always" default:"never"`
	Filter           string   `json:"filter" required:"true" minLength:"1"`
	Attributes       []string `json:"attributes,omitempty"`
	SizeLimit        int      `json:"size_limit,omitempty" minimum:"0"`
	TimeLimitSeconds int      `json:"time_limit_seconds,omitempty" minimum:"0"`
	TypesOnly        bool     `json:"types_only" default:"false"`
	Output           string   `json:"output" required:"true" pattern:"^(ctx|encCtx|closedCtx|tempCtx)(\\.\\w+)+$"`
	Outcome          struct {
		Found    string `json:"found" required:"true" format:"uuid"`
		NotFound string `json:"not_found" required:"true" format:"uuid"`
		Error    string `json:"error" required:"true" format:"uuid"`
	} `json:"outcome" required:"true"`
}

func (*LDAPSearch) GetStepType() string { return types.LDAPSearchStep }

func (*LDAPSearch) Execute(transaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	repository, err := transactionLDAPRepository(transaction, config.Get("connection").AsStringOr(""))
	if err != nil {
		return "error", nil
	}
	var attributes []string
	_ = config.Get("attributes").AsStruct(&attributes)
	request := &LDAPSearchRequest{
		BaseDN:           config.Get("base_dn").AsStringOr(""),
		Scope:            config.Get("scope").AsStringOr("sub"),
		DerefAliases:     config.Get("deref_aliases").AsStringOr("never"),
		Filter:           config.Get("filter").AsStringOr(""),
		Attributes:       attributes,
		SizeLimit:        int(config.Get("size_limit").AsIntOr(0)),
		TimeLimitSeconds: int(config.Get("time_limit_seconds").AsIntOr(0)),
		TypesOnly:        config.Get("types_only").AsBoolOr(false),
	}
	result, err := repository.Search(transactionContext(transaction), request)
	if err != nil {
		return "error", nil
	}
	if err := setLDAPOutput(transaction, config.Get("output").AsStringOr(""), map[string]any{"count": len(result.Entries), "entries": result.Entries}); err != nil {
		return "error", nil
	}
	if len(result.Entries) == 0 {
		return "not_found", nil
	}
	return "found", nil
}

func init() {
	defaultStepRegistry.AddStep(&LDAPSearch{}, map[string]map[string]any{
		".": {"x-category": types.ContextCategory, "x-order": []string{"connection", "base_dn", "scope", "deref_aliases", "filter", "attributes", "size_limit", "time_limit_seconds", "types_only", "output", "outcome"}},
	})
}
