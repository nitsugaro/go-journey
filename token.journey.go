package gojourney

import (
	"encoding/json"
	"sync"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jws"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/nitsugaro/go-journey/types"
	jwtek "github.com/nitsugaro/go-jwte-manager/v2"
)

const JOURNEY_PURPOSE = "JOURNEY"

// JourneyTokens is the signing and validation boundary for resumable tokens.
// Implementations must authenticate all returned state claims.
type JourneyTokens interface {
	Validate(token string) (*types.JourneyState, error)
	Sign(state *types.JourneyState) ([]byte, error)
}

type defaultJourneyTokens struct{}

var journeyKeyMu sync.Mutex

func (defaultJourneyTokens) Validate(token string) (*types.JourneyState, error) {
	return validateJourneyToken(token)
}

func (defaultJourneyTokens) Sign(state *types.JourneyState) ([]byte, error) {
	return signJourneyToken(state)
}

func getJourneyKey() (*jwtek.Key, error) {
	keys, total := jwtek.GetKeyStorage().Query(
		jwtek.ChainCondition(jwtek.IsActive, jwtek.IsNotExpired, jwtek.IsHmacAlg, jwtek.IsPurpose(JOURNEY_PURPOSE)),
		1)

	if total == 0 {
		return nil, ErrInvalidJourneyToken
	}

	return keys[0], nil
}

func validateJourneyToken(journeyToken string) (*types.JourneyState, error) {
	msg, err := jws.Parse([]byte(journeyToken))
	if err != nil || len(msg.Signatures()) == 0 {
		return nil, ErrInvalidJourneyToken
	}

	kid := msg.Signatures()[0].ProtectedHeaders().KeyID()
	key, err := jwtek.GetKeyStorage().Load(kid)
	if err != nil {
		return nil, ErrInvalidJourneyToken
	}

	_, err = jwt.Parse([]byte(journeyToken), jwt.WithKey(jwa.HS256, key.GetValue()))
	if err != nil {
		return nil, ErrInvalidJourneyToken
	}

	var claims types.JourneyState
	err = json.Unmarshal(msg.Payload(), &claims)
	if err != nil {
		return nil, ErrInvalidJourneyToken
	}

	return &claims, nil
}

func signJourneyToken(state *types.JourneyState) ([]byte, error) {
	key, err := getJourneyKey()
	if err != nil {
		journeyKeyMu.Lock()
		defer journeyKeyMu.Unlock()
		key, err = getJourneyKey()
		if err != nil {
			key = jwtek.NewHMACKey(jwtek.HS256, JOURNEY_PURPOSE)
			if saveErr := jwtek.GetKeyStorage().Save(key); saveErr != nil {
				return nil, saveErr
			}
		}
	}

	payload := make(map[string]any)
	payload["jti"] = state.Jti
	payload["trackings_id"] = state.TrackingsID
	if state.Realm != "" {
		payload["realm"] = state.Realm
	}
	if len(state.Ctx) > 0 {
		payload["ctx"] = state.Ctx
	}
	if state.EncryptedCtx != "" {
		payload["encrypted_ctx"] = state.EncryptedCtx
	}

	headers := jws.NewHeaders()
	headers.Set("kid", key.ID)

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	signed, err := jws.Sign(payloadBytes,
		jws.WithKey(jwa.HS256, key.GetValue(), jws.WithProtectedHeaders(headers)),
	)
	if err != nil {
		return nil, err
	}

	return signed, nil
}
