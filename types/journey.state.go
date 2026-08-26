package types

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	goutils "github.com/nitsugaro/go-utils/v2"
)

type JourneyState struct {
	mu        sync.Mutex          `json:"-"`
	ClosedCtx goutils.TreeMapImpl `json:"-"`
	Exp       time.Duration       `json:"-"`

	//["journey_id:step_id"]
	TrackingsID []string `json:"trackings_id"`
	Jti         string   `json:"jti"`
	Realm       string   `json:"realm,omitempty"`

	//Raw map context retrieved from JWT.
	Ctx goutils.DefaultMap `json:"ctx"`
	//Tree Context restored from raw context.
	ctx           goutils.TreeMapImpl `json:"-"`
	EncryptedCtx  string              `json:"encrypted_ctx"`
	encryptedCtx  goutils.TreeMapImpl `json:"-"`
	tempCtx       goutils.TreeMapImpl `json:"-"`
	encryptionKey []byte              `json:"-"`
	encryptionErr error               `json:"-"`
	result        any                 `json:"-"`
	resultSet     bool                `json:"-"`
}

type journeyStateStorageJSON struct {
	TrackingsID     []string           `json:"trackings_id,omitempty"`
	Jti             string             `json:"jti,omitempty"`
	Realm           string             `json:"realm,omitempty"`
	Ctx             goutils.DefaultMap `json:"ctx,omitempty"`
	EncryptedCtx    string             `json:"encrypted_ctx,omitempty"`
	EncryptedCtxMap goutils.DefaultMap `json:"encrypted_ctx_map,omitempty"`
	ClosedCtx       goutils.DefaultMap `json:"closed_ctx,omitempty"`
	TempCtx         goutils.DefaultMap `json:"temp_ctx,omitempty"`
	Exp             int64              `json:"exp,omitempty"`
}

func NewJourneyState() *JourneyState {
	return &JourneyState{
		tempCtx: goutils.NewTreeMap(),
	}
}

func (js *JourneyState) MarshalStorageJSON() ([]byte, error) {
	if js == nil {
		return nil, nil
	}
	snapshot := journeyStateStorageJSON{
		TrackingsID:  append([]string(nil), js.TrackingsID...),
		Jti:          js.Jti,
		Realm:        js.Realm,
		EncryptedCtx: js.EncryptedCtx,
		Exp:          int64(js.Exp),
	}
	var err error
	if snapshot.Ctx, err = treeMapStorageMap(js.GetCtx()); err != nil {
		return nil, err
	}
	if js.encryptedCtx != nil {
		if snapshot.EncryptedCtxMap, err = treeMapStorageMap(js.encryptedCtx); err != nil {
			return nil, err
		}
	}
	if js.ClosedCtx != nil {
		if snapshot.ClosedCtx, err = treeMapStorageMap(js.ClosedCtx); err != nil {
			return nil, err
		}
	}
	if js.tempCtx != nil {
		if snapshot.TempCtx, err = treeMapStorageMap(js.tempCtx); err != nil {
			return nil, err
		}
	}
	return json.Marshal(snapshot)
}

func UnmarshalJourneyStateStorageJSON(data []byte) (*JourneyState, error) {
	var snapshot journeyStateStorageJSON
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, err
	}
	state := &JourneyState{
		TrackingsID:  append([]string(nil), snapshot.TrackingsID...),
		Jti:          snapshot.Jti,
		Realm:        snapshot.Realm,
		Ctx:          snapshot.Ctx,
		EncryptedCtx: snapshot.EncryptedCtx,
		Exp:          time.Duration(snapshot.Exp),
	}
	if snapshot.Ctx != nil {
		state.ctx = goutils.NewTreeMap(snapshot.Ctx)
	}
	if snapshot.EncryptedCtxMap != nil {
		state.encryptedCtx = goutils.NewTreeMap(snapshot.EncryptedCtxMap)
	}
	if snapshot.ClosedCtx != nil {
		state.ClosedCtx = goutils.NewTreeMap(snapshot.ClosedCtx)
	}
	if snapshot.TempCtx != nil {
		state.tempCtx = goutils.NewTreeMap(snapshot.TempCtx)
	} else {
		state.tempCtx = goutils.NewTreeMap()
	}
	return state, nil
}

func treeMapStorageMap(value goutils.TreeMapImpl) (goutils.DefaultMap, error) {
	if value == nil {
		return nil, nil
	}
	return value.AsMap()
}

func (js *JourneyState) Init() {
	js.tempCtx = goutils.NewTreeMap()
}

func (js *JourneyState) SetEncryptionKey(key []byte) {
	js.encryptionKey = append(js.encryptionKey[:0], key...)
	js.encryptionErr = nil
}

func (js *JourneyState) MergeState(state *JourneyState) {
	if js.TrackingsID == nil {
		js.TrackingsID = state.TrackingsID
	}

	if js.encryptedCtx == nil {
		js.encryptedCtx = state.encryptedCtx
	}

	if js.ClosedCtx == nil {
		js.ClosedCtx = state.ClosedCtx
	}

	if js.tempCtx == nil {

		js.tempCtx = state.tempCtx
	}

	if js.Realm == "" {
		js.Realm = state.Realm
	}

	if js.result == nil {
		js.result = state.result
	}
}

func (js *JourneyState) GetRealm() string {
	return js.Realm
}

func (js *JourneyState) SetRealm(realm string) {
	js.Realm = realm
}

func (js *JourneyState) UnshiftTracking(journeyID string, stepID string) *JourneyState {
	js.mu.Lock()
	defer js.mu.Unlock()
	js.TrackingsID = append([]string{journeyID + ":" + stepID}, js.TrackingsID...)
	return js
}

func (js *JourneyState) PushTracking(journeyID string, stepID string) *JourneyState {
	js.mu.Lock()
	defer js.mu.Unlock()
	js.TrackingsID = append(js.TrackingsID, journeyID+":"+stepID)
	return js
}

func (js *JourneyState) PopTracking() (string, string) {
	js.mu.Lock()
	defer js.mu.Unlock()
	if len(js.TrackingsID) != 0 {
		length := len(js.TrackingsID)
		parts := strings.SplitN(js.TrackingsID[length-1], ":", 2)
		js.TrackingsID = js.TrackingsID[0 : length-1]
		if len(parts) != 2 {
			return "", ""
		}
		return parts[0], parts[1]
	}

	return "", ""
}

func (js *JourneyState) GetTracking() (string, string) {
	js.mu.Lock()
	defer js.mu.Unlock()
	if len(js.TrackingsID) != 0 {
		parts := strings.SplitN(js.TrackingsID[len(js.TrackingsID)-1], ":", 2)
		if len(parts) != 2 {
			return "", ""
		}
		return parts[0], parts[1]
	}

	return "", ""
}

func (js *JourneyState) ExistsTracking() bool {
	js.mu.Lock()
	defer js.mu.Unlock()
	return len(js.TrackingsID) != 0
}

func (js *JourneyState) GetID() string {
	return js.Jti
}

func (js *JourneyState) SetID(jti string) {
	js.Jti = jti
}

func (js *JourneyState) SetResult(result any) {
	if js == nil {
		return
	}
	js.mu.Lock()
	defer js.mu.Unlock()
	js.result = result
	js.resultSet = true
}

func (js *JourneyState) GetResult() any {
	if js == nil {
		return nil
	}
	js.mu.Lock()
	defer js.mu.Unlock()
	return js.result
}

func (js *JourneyState) HasResult() bool {
	if js == nil {
		return false
	}
	js.mu.Lock()
	defer js.mu.Unlock()
	return js.resultSet
}

func (js *JourneyState) ClearState() {
	js.ctx = nil
	js.encryptedCtx = nil
	js.tempCtx = nil
}

func (js *JourneyState) GetCtx() goutils.TreeMapImpl {
	if js.ctx != nil {
		return js.ctx
	}

	if js.Ctx != nil {
		js.ctx = goutils.NewTreeMap(js.Ctx)
	} else {
		js.ctx = goutils.NewTreeMap()
	}

	return js.ctx
}

func (js *JourneyState) SetCtx(ctx goutils.TreeMapImpl) {
	js.ctx = ctx
}

func (js *JourneyState) GetEncryptedCtx() goutils.TreeMapImpl {
	if js.encryptedCtx != nil {
		return js.encryptedCtx
	}

	if js.EncryptedCtx != "" {
		encryptedCtx, err := DecryptCtx(js.EncryptedCtx, js.encryptionKey)
		if err != nil {
			js.encryptionErr = err
			js.encryptedCtx = goutils.NewTreeMap()
			return js.encryptedCtx
		}

		js.encryptedCtx = encryptedCtx
	} else {
		js.encryptedCtx = goutils.NewTreeMap()
	}

	return js.encryptedCtx
}

func (js *JourneyState) GetEncryptionError() error {
	return js.encryptionErr
}

func (js *JourneyState) SetEncryptedCtx(encryptedCtx goutils.TreeMapImpl) {
	js.encryptedCtx = encryptedCtx
}

func (js *JourneyState) GetClosedCtx() goutils.TreeMapImpl {
	if js.ClosedCtx == nil {
		js.ClosedCtx = goutils.NewTreeMap()
	}
	return js.ClosedCtx
}

func (js *JourneyState) SetClosedCtx(closedCtx goutils.TreeMapImpl) {
	js.ClosedCtx = closedCtx
}

func (js *JourneyState) GetTempCtx() goutils.TreeMapImpl {
	if js.tempCtx == nil {
		js.tempCtx = goutils.NewTreeMap()
	}
	return js.tempCtx
}

func (js *JourneyState) SetTempCtx(tempCtx goutils.TreeMapImpl) {
	js.tempCtx = tempCtx
}

func (js *JourneyState) SetAllCtx(ctx, encryptedCtx, closedCtx, tempCtx goutils.TreeMapImpl) {
	js.SetCtx(ctx)
	js.SetEncryptedCtx(encryptedCtx)
	js.SetClosedCtx(closedCtx)
	js.SetTempCtx(tempCtx)
}

func (js *JourneyState) GetterFunctions() map[string]any {
	return map[string]any{
		"getCtxProperty": func(args ...any) any {
			key, def := parseArgs(args...)
			return js.GetCtx().Get(key).AsAnyOr(def)
		},
		"getClosedCtxProperty": func(args ...any) any {
			key, def := parseArgs(args...)
			return js.GetClosedCtx().Get(key).AsAnyOr(def)
		},
		"getEncCtxProperty": func(args ...any) any {
			key, def := parseArgs(args...)
			return js.GetEncryptedCtx().Get(key).AsAnyOr(def)
		},
		"getTempCtxProperty": func(args ...any) any {
			key, def := parseArgs(args...)
			return js.GetTempCtx().Get(key).AsAnyOr(def)
		},
	}
}

func (js *JourneyState) GetCtxPath(ctxPath string) (goutils.TreeMapImpl, string) {
	parts := strings.SplitN(ctxPath, ".", 2)
	if len(parts) != 2 || parts[1] == "" {
		return nil, ""
	}
	ctxType := parts[0]

	return js.Get(ctxType), parts[1]
}

func (js *JourneyState) Get(ctxType string) goutils.TreeMapImpl {
	switch ctxType {
	case CtxKey:
		return js.GetCtx()
	case EncCtxKey:
		return js.GetEncryptedCtx()
	case ClosedCtxKey:
		return js.GetClosedCtx()
	case TempCtxKey:
		return js.GetTempCtx()
	}

	return nil
}
