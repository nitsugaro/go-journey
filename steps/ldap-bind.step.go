package steps

import (
	ldap "github.com/go-ldap/ldap/v3"
	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type LDAPBind struct {
	BasicStep

	_          struct{} `description:"Validates LDAP credentials by binding on a fresh connection from the selected LDAP repository configuration."`
	Connection string   `json:"connection" required:"true" minLength:"1"`
	DN         string   `json:"dn" required:"true" minLength:"1"`
	Password   string   `json:"password,omitempty"`
	Outcome    struct {
		Valid   string `json:"valid" required:"true" format:"uuid"`
		Invalid string `json:"invalid" required:"true" format:"uuid"`
		Error   string `json:"error" required:"true" format:"uuid"`
	} `json:"outcome" required:"true"`
}

func (*LDAPBind) GetStepType() string { return types.LDAPBindStep }

func (*LDAPBind) Execute(transaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	repository, err := transactionLDAPRepository(transaction, config.Get("connection").AsStringOr(""))
	if err != nil {
		return "error", nil
	}
	err = repository.Bind(transactionContext(transaction), config.Get("dn").AsStringOr(""), config.Get("password").AsStringOr(""))
	if err == nil {
		return "valid", nil
	}
	if ldap.IsErrorWithCode(err, ldap.LDAPResultInvalidCredentials) {
		return "invalid", nil
	}
	return "error", nil
}

func init() {
	defaultStepRegistry.AddStep(&LDAPBind{}, map[string]map[string]any{
		".": {"x-category": types.AuthCategory, "x-order": []string{"connection", "dn", "password", "outcome"}},
	})
}
