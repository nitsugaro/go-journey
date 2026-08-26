package steps

import (
	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type LDAPDelete struct {
	BasicStep

	_          struct{} `description:"Deletes one LDAP entry."`
	Connection string   `json:"connection" required:"true" minLength:"1"`
	DN         string   `json:"dn" required:"true" minLength:"1"`
	Outcome    struct {
		Success  string `json:"success" required:"true" format:"uuid"`
		NotFound string `json:"not_found" required:"true" format:"uuid"`
		Error    string `json:"error" required:"true" format:"uuid"`
	} `json:"outcome" required:"true"`
}

func (*LDAPDelete) GetStepType() string { return types.LDAPDeleteStep }

func (*LDAPDelete) Execute(transaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	repository, err := transactionLDAPRepository(transaction, config.Get("connection").AsStringOr(""))
	if err != nil {
		return "error", nil
	}
	err = repository.Delete(transactionContext(transaction), config.Get("dn").AsStringOr(""))
	return ldapWriteOutcome(err), nil
}

func init() {
	defaultStepRegistry.AddStep(&LDAPDelete{}, map[string]map[string]any{
		".": {"x-category": types.ContextCategory, "x-order": []string{"connection", "dn", "outcome"}},
	})
}
