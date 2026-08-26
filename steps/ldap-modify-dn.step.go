package steps

import (
	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type LDAPModifyDN struct {
	BasicStep

	_            struct{} `description:"Renames or moves one LDAP entry."`
	Connection   string   `json:"connection" required:"true" minLength:"1"`
	DN           string   `json:"dn" required:"true" minLength:"1"`
	RDN          string   `json:"rdn" required:"true" minLength:"1"`
	DeleteOldRDN bool     `json:"delete_old_rdn" default:"true"`
	NewSuperior  string   `json:"new_superior,omitempty"`
	Outcome      struct {
		Success  string `json:"success" required:"true" format:"uuid"`
		NotFound string `json:"not_found" required:"true" format:"uuid"`
		Error    string `json:"error" required:"true" format:"uuid"`
	} `json:"outcome" required:"true"`
}

func (*LDAPModifyDN) GetStepType() string { return types.LDAPModifyDNStep }

func (*LDAPModifyDN) Execute(transaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	repository, err := transactionLDAPRepository(transaction, config.Get("connection").AsStringOr(""))
	if err != nil {
		return "error", nil
	}
	err = repository.ModifyDN(transactionContext(transaction), &LDAPModifyDNRequest{
		DN:           config.Get("dn").AsStringOr(""),
		RDN:          config.Get("rdn").AsStringOr(""),
		DeleteOldRDN: config.Get("delete_old_rdn").AsBoolOr(true),
		NewSuperior:  config.Get("new_superior").AsStringOr(""),
	})
	return ldapWriteOutcome(err), nil
}

func init() {
	defaultStepRegistry.AddStep(&LDAPModifyDN{}, map[string]map[string]any{
		".": {"x-category": types.ContextCategory, "x-order": []string{"connection", "dn", "rdn", "delete_old_rdn", "new_superior", "outcome"}},
	})
}
