package types

import (
	"context"
	"sync"

	jcache "github.com/nitsugaro/go-journey/cache"
	"github.com/nitsugaro/go-journey/inputs"
	"github.com/nitsugaro/go-nstore"
	goutils "github.com/nitsugaro/go-utils/v2"
)

const (
	CtxKey       = "ctx"
	EncCtxKey    = "encCtx"
	ClosedCtxKey = "closedCtx"
	TempCtxKey   = "tempCtx"
)

var allCtxKeys = []string{CtxKey, EncCtxKey, ClosedCtxKey, TempCtxKey}

type JourneyConfiguration struct {
	*nstore.Metadata

	Name                  string `json:"name" binding:"required"`
	Description           string `json:"description"`
	EncryptedClientInputs bool   `json:"encrypted_client_inputs"`
	Confidential          bool   `json:"confidential"`
	Active                bool   `json:"active"`
	Debug                 bool   `json:"debug"`
	JourneyType           string `json:"journey_type,omitempty"`

	DefaultExp        int64            `json:"default_exp" binding:"required"`
	Realm             string           `json:"realm"`
	StartStepID       string           `json:"start_step_id" binding:"required"`
	SubEntries        []string         `json:"sub_entries" binding:"required"`
	Steps             map[string]*Step `json:"steps" binding:"required"`
	AdditionalProps   map[string]any   `json:"additional_properties"`
	additionalProps   goutils.TreeMapImpl
	additionalPropsMu sync.Mutex
}

func (jc *JourneyConfiguration) GetProp(property string) goutils.TreeMapImpl {
	jc.additionalPropsMu.Lock()
	defer jc.additionalPropsMu.Unlock()
	if jc.additionalProps == nil {
		jc.additionalProps = goutils.NewTreeMap(jc.AdditionalProps)
	}

	return jc.additionalProps.Get(property)
}

type JourneyTransaction struct {
	Context              context.Context
	Request              RequestAccessor
	Response             ResponseMutator
	CacheManager         *jcache.Manager
	Journey              *JourneyConfiguration
	CurrentStepID        string
	ChainStepID          string
	ClientInputsBuilder  *inputs.ClientInputsBuilder
	State                *JourneyState
	Steps                *Steps
	Payload              *JourneyPayloadReq
	OnAsyncError         func(step *Step, err error)
	PlaceholderResolvers map[string]PlaceholderResolver
	Observer             Observer
	// InteractionState contains native-only values for this invocation. It is
	// shared by steps and child transactions, but never serialized on pause.
	InteractionState TransactionValues `json:"-"`
	Ticks            map[string]*struct {
		Count         int64
		LastExecution int64
	}
}

// PlaceholderResolver resolves the path after a custom placeholder prefix.
// For ${secrets.api.key}, the resolver registered as "secrets" receives "api.key".
type PlaceholderResolver func(path string) (any, error)

func IsCtx(ctxType string) bool {
	return goutils.Some(allCtxKeys, func(str string, _ int) bool { return str == ctxType })
}
