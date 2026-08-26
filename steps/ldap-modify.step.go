package steps

import (
	"strings"

	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type LDAPModify struct {
	BasicStep

	_          struct{}           `description:"Applies LDAP attribute changes to one DN."`
	Connection string             `json:"connection" required:"true" minLength:"1"`
	DN         string             `json:"dn" required:"true" minLength:"1"`
	Changes    []LDAPModification `json:"changes" required:"true" minItems:"1"`
	Outcome    struct {
		Success  string `json:"success" required:"true" format:"uuid"`
		NotFound string `json:"not_found" required:"true" format:"uuid"`
		Error    string `json:"error" required:"true" format:"uuid"`
	} `json:"outcome" required:"true"`
}

func (*LDAPModify) GetStepType() string { return types.LDAPModifyStep }

func (*LDAPModify) VerifyConfig(stepName string, config goutils.TreeMapImpl) error {
	var changes []LDAPModification
	if err := config.Get("changes").AsStruct(&changes); err != nil {
		return nil
	}
	for _, change := range changes {
		switch strings.ToLower(change.Operation) {
		case "add", "delete", "replace", "increment":
		default:
			return types.StepInvalidConfig(stepName, "unsupported LDAP modify operation "+change.Operation)
		}
	}
	return nil
}

func (*LDAPModify) Execute(transaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	repository, err := transactionLDAPRepository(transaction, config.Get("connection").AsStringOr(""))
	if err != nil {
		return "error", nil
	}
	var changes []LDAPModification
	_ = config.Get("changes").AsStruct(&changes)
	err = repository.Modify(transactionContext(transaction), &LDAPModifyRequest{DN: config.Get("dn").AsStringOr(""), Changes: changes})
	return ldapWriteOutcome(err), nil
}

func init() {
	defaultStepRegistry.AddStep(&LDAPModify{}, map[string]map[string]any{
		".": {"x-category": types.ContextCategory, "x-order": []string{"connection", "dn", "changes", "outcome"}},
	})
}
