package steps

import (
	ldap "github.com/go-ldap/ldap/v3"
	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type LDAPAdd struct {
	BasicStep

	_          struct{}            `description:"Adds one LDAP entry."`
	Connection string              `json:"connection" required:"true" minLength:"1"`
	DN         string              `json:"dn" required:"true" minLength:"1"`
	Attributes map[string][]string `json:"attributes" required:"true"`
	Outcome    struct {
		Success       string `json:"success" required:"true" format:"uuid"`
		AlreadyExists string `json:"already_exists" required:"true" format:"uuid"`
		Error         string `json:"error" required:"true" format:"uuid"`
	} `json:"outcome" required:"true"`
}

func (*LDAPAdd) GetStepType() string { return types.LDAPAddStep }

func (*LDAPAdd) Execute(transaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	repository, err := transactionLDAPRepository(transaction, config.Get("connection").AsStringOr(""))
	if err != nil {
		return "error", nil
	}
	var attributes map[string][]string
	_ = config.Get("attributes").AsStruct(&attributes)
	err = repository.Add(transactionContext(transaction), &LDAPAddRequest{DN: config.Get("dn").AsStringOr(""), Attributes: attributes})
	if err == nil {
		return "success", nil
	}
	if ldap.IsErrorWithCode(err, ldap.LDAPResultEntryAlreadyExists) {
		return "already_exists", nil
	}
	return "error", nil
}

func init() {
	defaultStepRegistry.AddStep(&LDAPAdd{}, map[string]map[string]any{
		".": {"x-category": types.ContextCategory, "x-order": []string{"connection", "dn", "attributes", "outcome"}},
	})
}
