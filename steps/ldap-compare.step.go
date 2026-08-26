package steps

import (
	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type LDAPCompare struct {
	BasicStep

	_          struct{} `description:"Compares one LDAP attribute value on a DN and routes to true, false, or error."`
	Connection string   `json:"connection" required:"true" minLength:"1"`
	DN         string   `json:"dn" required:"true" minLength:"1"`
	Attribute  string   `json:"attribute" required:"true" minLength:"1"`
	Value      string   `json:"value" required:"true"`
	Outcome    struct {
		True  string `json:"true" required:"true" format:"uuid"`
		False string `json:"false" required:"true" format:"uuid"`
		Error string `json:"error" required:"true" format:"uuid"`
	} `json:"outcome" required:"true"`
}

func (*LDAPCompare) GetStepType() string { return types.LDAPCompareStep }

func (*LDAPCompare) Execute(transaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	repository, err := transactionLDAPRepository(transaction, config.Get("connection").AsStringOr(""))
	if err != nil {
		return "error", nil
	}
	matched, err := repository.Compare(transactionContext(transaction), config.Get("dn").AsStringOr(""), config.Get("attribute").AsStringOr(""), config.Get("value").AsStringOr(""))
	if err != nil {
		return "error", nil
	}
	if matched {
		return "true", nil
	}
	return "false", nil
}

func init() {
	defaultStepRegistry.AddStep(&LDAPCompare{}, map[string]map[string]any{
		".": {"x-category": types.FlowCategory, "x-order": []string{"connection", "dn", "attribute", "value", "outcome"}},
	})
}
