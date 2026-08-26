package steps_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	jcache "github.com/nitsugaro/go-journey/cache"
	"github.com/nitsugaro/go-journey/inputs"
	journeysteps "github.com/nitsugaro/go-journey/steps"
	"github.com/nitsugaro/go-journey/types"
	jsrun "github.com/nitsugaro/go-jsruntime/v2"
	"github.com/nitsugaro/go-nstore"
	"github.com/nitsugaro/go-utils/encoding"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type scriptBindingsProviderFunc func(*journeysteps.ScriptBindingContext) (map[string]any, error)

func (provider scriptBindingsProviderFunc) Bindings(context *journeysteps.ScriptBindingContext) (map[string]any, error) {
	return provider(context)
}

type scriptRuntimeBindingsProviderFunc func(*journeysteps.ScriptBindingContext) (map[string]any, error)

func (provider scriptRuntimeBindingsProviderFunc) GetBindings(context *journeysteps.ScriptBindingContext) (map[string]any, error) {
	return provider(context)
}

type scriptDescriptorProvider struct {
	scriptRuntimeBindingsProviderFunc
	descriptors []journeysteps.ScriptBindingDescriptor
}

type scriptHTTPClient struct {
	method          string
	uri             string
	headers         map[string]string
	body            []byte
	requestCalls    int
	contextReqCalls int
}

func (client *scriptHTTPClient) Request(method string, uri string, headers map[string]string, body []byte) (*goutils.Response, error) {
	client.requestCalls++
	client.method = method
	client.uri = uri
	client.headers = headers
	client.body = body
	return &goutils.Response{
		RequestUri: uri,
		Status:     http.StatusCreated,
		Headers:    http.Header{"X-Trace": []string{"abc"}},
		Body:       []byte(`{"ok":true}`),
	}, nil
}

func (client *scriptHTTPClient) RequestWithContext(ctx context.Context, method string, uri string, headers map[string]string, body []byte) (*goutils.Response, error) {
	client.contextReqCalls++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return client.Request(method, uri, headers, body)
}

func (provider scriptDescriptorProvider) BindingDescriptors(context *journeysteps.ScriptBindingDescriptorContext) ([]journeysteps.ScriptBindingDescriptor, error) {
	return provider.descriptors, nil
}

func configureJourneyScript(t *testing.T, transaction *types.JourneyTransaction, code string) string {
	t.Helper()
	manager, storage := jsrun.NewDefaultStorage(t.TempDir())
	script := &jsrun.Script{
		Metadata: &nstore.Metadata{ID: "00000000-0000-0000-0000-000000000901"},
		Name:     "test-script-" + t.Name(), Type: journeysteps.JourneyScript,
		CodeBase64: encoding.EncodeBase64([]byte(code)),
	}
	if err := storage.Save(script); err != nil {
		t.Fatal(err)
	}
	cacheManager, err := jcache.NewManager(&jcache.ManagerConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if err := journeysteps.ConfigureScriptRuntime(cacheManager, manager, storage); err != nil {
		t.Fatal(err)
	}
	transaction.CacheManager = cacheManager
	return script.ID
}

func configureScript(t *testing.T, transaction *types.JourneyTransaction, scriptType string, code string) string {
	t.Helper()
	manager, storage := jsrun.NewDefaultStorage(t.TempDir())
	script := &jsrun.Script{
		Metadata:   &nstore.Metadata{ID: "00000000-0000-0000-0000-000000000902"},
		Name:       "test-script-" + t.Name(),
		Type:       scriptType,
		CodeBase64: encoding.EncodeBase64([]byte(code)),
	}
	if err := storage.Save(script); err != nil {
		t.Fatal(err)
	}
	cacheManager, err := jcache.NewManager(&jcache.ManagerConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if err := journeysteps.ConfigureScriptRuntime(cacheManager, manager, storage); err != nil {
		t.Fatal(err)
	}
	transaction.CacheManager = cacheManager
	return script.ID
}

func configureDeclaredScript(t *testing.T, transaction *types.JourneyTransaction, outcomes []string, code string) string {
	t.Helper()
	manager, storage := jsrun.NewDefaultStorage(t.TempDir())
	script := &jsrun.Script{
		Metadata:        &nstore.Metadata{ID: "00000000-0000-0000-0000-000000000903"},
		Name:            "test-script-" + t.Name(),
		Type:            journeysteps.JourneyScript,
		CodeBase64:      encoding.EncodeBase64([]byte(code)),
		AdditionalProps: map[string]any{journeysteps.ScriptOutcomesProp: outcomes},
	}
	if err := storage.Save(script); err != nil {
		t.Fatal(err)
	}
	cacheManager, err := jcache.NewManager(&jcache.ManagerConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if err := journeysteps.ConfigureScriptRuntime(cacheManager, manager, storage); err != nil {
		t.Fatal(err)
	}
	transaction.CacheManager = cacheManager
	return script.ID
}

func TestDeclaredScriptOutcomesAreRestrictedAndNormalized(t *testing.T) {
	supported := []string{
		journeysteps.JourneyScript,
		journeysteps.ResourceScript,
		journeysteps.WorkflowScript,
	}
	for _, scriptType := range supported {
		t.Run(scriptType, func(t *testing.T) {
			script := &jsrun.Script{
				Type: scriptType,
				AdditionalProps: map[string]any{
					journeysteps.ScriptOutcomesProp: []any{" Success ", "FAILURE", "success"},
				},
			}
			if err := journeysteps.NormalizeDeclaredScriptOutcomes(script); err != nil {
				t.Fatal(err)
			}
			outcomes := journeysteps.DeclaredScriptOutcomes(script)
			if len(outcomes) != 2 || outcomes[0] != "success" || outcomes[1] != "failure" {
				t.Fatalf("outcomes=%#v", outcomes)
			}
		})
	}

	excluded := []string{
		journeysteps.AsyncScript,
		journeysteps.ScheduleScript,
		journeysteps.LibraryScript,
	}
	for _, scriptType := range excluded {
		t.Run(scriptType, func(t *testing.T) {
			script := &jsrun.Script{
				Type:            scriptType,
				AdditionalProps: map[string]any{journeysteps.ScriptOutcomesProp: []any{"success"}},
			}
			if err := journeysteps.NormalizeDeclaredScriptOutcomes(script); err != nil {
				t.Fatal(err)
			}
			if len(journeysteps.DeclaredScriptOutcomes(script)) != 0 || script.AdditionalProps != nil {
				t.Fatalf("excluded type retained outcomes: %#v", script.AdditionalProps)
			}
		})
	}
}

func TestScriptOutcomeIsCaseInsensitiveAndMustBeDeclared(t *testing.T) {
	transaction := newStepTransaction()
	scriptID := configureDeclaredScript(t, transaction, []string{"success"}, `setOutcome("SuCcEsS");`)
	outcome, err := types.ExecuteStepConfig(&journeysteps.Script{}, transaction, map[string]any{
		"script_id": scriptID, "timeout_seconds": 2,
		"outcome": map[string]any{"SUCCESS": "next"},
	})
	if err != nil || outcome != "success" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}

	transaction = newStepTransaction()
	scriptID = configureDeclaredScript(t, transaction, []string{"failure"}, `setOutcome("success");`)
	if _, err := types.ExecuteStepConfig(&journeysteps.Script{}, transaction, map[string]any{
		"script_id": scriptID, "timeout_seconds": 2,
		"outcome": map[string]any{"failure": "next"},
	}); err == nil {
		t.Fatal("expected undeclared script outcome to fail")
	}
}

func TestOutcomeNormalizationDoesNotAffectOtherScriptTypes(t *testing.T) {
	transaction := newStepTransaction()
	scriptID := configureScript(t, transaction, journeysteps.ScheduleScript, `setOutcome("MiXeD");`)
	outcome, err := types.ExecuteStepConfig(&journeysteps.Script{}, transaction, map[string]any{
		"script_id": scriptID, "timeout_seconds": 2,
		"outcome": map[string]any{"MiXeD": "next"},
	})
	if err != nil || outcome != "MiXeD" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
}

func TestScriptUsesResolvedArgsAndAllCurrentBindings(t *testing.T) {
	transaction := newStepTransaction()
	scriptID := configureJourneyScript(t, transaction, `
        ctx.Set("script.ctx", args.message);
        encCtx.Set("script.enc", true);
        closedCtx.Set("script.closed", realm);
        tempCtx.Set("script.temp", request.Method);
        setOutcome("done");
    `)
	transaction.State.SetRealm("alpha")
	transaction.State.GetCtx().Set("source", "resolved")
	transaction.Request = &types.JourneyRequest{Method: "POST"}
	step := &types.Step{StepType: types.ScriptStep, Config: map[string]any{
		"script_id": scriptID, "timeout_seconds": 2,
		"args":    map[string]any{"message": "${ctx.source}"},
		"outcome": map[string]any{"done": "next"},
	}}
	if err := types.GenerateStepVariables(step, journeysteps.GetDefaultStepRegistry()); err != nil {
		t.Fatal(err)
	}
	outcome, err := types.ExecuteStepConfig(&journeysteps.Script{}, transaction, step.Config)
	if err != nil || outcome != "done" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
	if transaction.State.GetCtx().Get("script.ctx").AsStringOr("") != "resolved" ||
		!transaction.State.GetEncryptedCtx().Get("script.enc").AsBoolOr(false) ||
		transaction.State.GetClosedCtx().Get("script.closed").AsStringOr("") != "alpha" ||
		transaction.State.GetTempCtx().Get("script.temp").AsStringOr("") != "POST" {
		t.Fatal("script did not receive the current context, request, realm, or resolved args bindings")
	}
}

func TestScriptCanSafelyReadRequestQueryAndHeaders(t *testing.T) {
	transaction := newStepTransaction()
	scriptID := configureJourneyScript(t, transaction, `
        ctx.Set("query.first", requestQuery.First("name", "missing"));
        ctx.Set("query.missing", requestQuery.First("missing", "fallback"));
        ctx.Set("query.allCount", requestQuery.All("tag").length);
        ctx.Set("header.contentType", requestHeader.First("content-type", "none"));
        ctx.Set("header.hasTrace", requestHeader.Has("X-TRACE"));
        setOutcome("done");
    `)
	transaction.Request = &types.JourneyRequest{
		QueryParameters: map[string][]string{"name": {"ada"}, "tag": {"one", "two"}},
		Headers:         map[string][]string{"Content-Type": {"application/json"}, "X-Trace": {"abc"}},
	}
	outcome, err := types.ExecuteStepConfig(&journeysteps.Script{}, transaction, map[string]any{
		"script_id": scriptID, "timeout_seconds": 2,
		"outcome": map[string]any{"done": "next"},
	})
	if err != nil || outcome != "done" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
	if transaction.State.GetCtx().Get("query.first").AsStringOr("") != "ada" ||
		transaction.State.GetCtx().Get("query.missing").AsStringOr("") != "fallback" ||
		transaction.State.GetCtx().Get("query.allCount").AsIntOr(0) != 2 ||
		transaction.State.GetCtx().Get("header.contentType").AsStringOr("") != "application/json" ||
		!transaction.State.GetCtx().Get("header.hasTrace").AsBoolOr(false) {
		t.Fatal("script request bindings did not produce the expected context values")
	}
}

func TestScriptCanSendHTTPWithConfiguredInstance(t *testing.T) {
	transaction := newStepTransaction()
	scriptID := configureJourneyScript(t, transaction, `
        const response = http.Send("https://api.example.test/users", {
            method: "post",
            instance: "analytics",
            headers: { "X-User": args.user },
            body: { id: args.user }
        });
        ctx.Set("http.status", response.status);
        ctx.Set("http.body", response.body);
        ctx.Set("http.trace", response.headers["X-Trace"][0]);
        setOutcome("done");
    `)
	client := &scriptHTTPClient{}
	if err := transaction.CacheManager.UpdateRuntimeCacheInstance(journeysteps.HTTPClientCacheKey, "analytics", client, 0); err != nil {
		t.Fatal(err)
	}
	outcome, err := types.ExecuteStepConfig(&journeysteps.Script{}, transaction, map[string]any{
		"script_id": scriptID, "timeout_seconds": 2,
		"args":    map[string]any{"user": "ada"},
		"outcome": map[string]any{"done": "next"},
	})
	if err != nil || outcome != "done" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
	if client.method != http.MethodPost || client.uri != "https://api.example.test/users" ||
		client.headers["X-User"] != "ada" || string(client.body) != `{"id":"ada"}` {
		t.Fatalf("http request method=%q uri=%q headers=%#v body=%s", client.method, client.uri, client.headers, string(client.body))
	}
	if transaction.State.GetCtx().Get("http.status").AsIntOr(0) != http.StatusCreated ||
		transaction.State.GetCtx().Get("http.body").AsStringOr("") != `{"ok":true}` ||
		transaction.State.GetCtx().Get("http.trace").AsStringOr("") != "abc" {
		t.Fatal("script did not receive expected http response binding")
	}
}

func TestScriptHTTPSendDoesNotInheritCanceledTransactionContext(t *testing.T) {
	transaction := newStepTransaction()
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	transaction.Context = cancelled
	scriptID := configureJourneyScript(t, transaction, `
        const response = http.Send("https://api.example.test/async", {
            instance: "analytics"
        });
        ctx.Set("http.status", response.status);
        setOutcome("done");
    `)
	client := &scriptHTTPClient{}
	if err := transaction.CacheManager.UpdateRuntimeCacheInstance(journeysteps.HTTPClientCacheKey, "analytics", client, 0); err != nil {
		t.Fatal(err)
	}
	outcome, err := types.ExecuteStepConfig(&journeysteps.Script{}, transaction, map[string]any{
		"script_id": scriptID, "timeout_seconds": 2,
		"outcome": map[string]any{"done": "next"},
	})
	if err != nil || outcome != "done" {
		t.Fatalf("outcome=%q err=%v", outcome, err)
	}
	if client.requestCalls != 1 || client.contextReqCalls != 0 {
		t.Fatalf("http.Send should use Request without transaction context, requestCalls=%d contextReqCalls=%d", client.requestCalls, client.contextReqCalls)
	}
	if transaction.State.GetCtx().Get("http.status").AsIntOr(0) != http.StatusCreated {
		t.Fatal("script did not receive expected http response")
	}
}

func TestScriptRequestsAndReadsGenericValueInputByExternalID(t *testing.T) {
	first := newStepTransaction()
	scriptID := configureJourneyScript(t, first, `
        if (clientInputs.IsClientEmpty()) {
            clientInputs.AddValueInput({
                external_id: "profile.nickname",
                type: "string",
                label: "Nickname",
                required: true
            });
        } else {
            const input = clientInputs.GetByExternalID("profile.nickname");
            ctx.Set("nickname", input.Input);
            setOutcome("done");
        }
    `)
	state := types.NewJourneyState()
	first.State = state
	first.ClientInputsBuilder = inputs.NewClientInputBuilder(nil, state.GetCtx())
	config := map[string]any{
		"script_id": scriptID, "timeout_seconds": 2,
		"outcome": map[string]any{"done": "next"},
	}
	outcome, err := types.ExecuteStepConfig(&journeysteps.Script{}, first, config)
	if err != nil || outcome != "" {
		t.Fatalf("request outcome=%q err=%v", outcome, err)
	}
	requested := first.ClientInputsBuilder.GetNewInputs()
	if len(requested) != 1 || requested[0].ExternalID != "profile.nickname" || requested[0].ID != "" || requested[0].StepType != types.ScriptStep {
		t.Fatalf("generic script input=%#v", requested)
	}
	provided := &inputs.ClientInput{ExternalID: "profile.nickname", StepType: types.ScriptStep, Type: inputs.STRING_INPUT, Input: "Ada"}
	secondBuilder := inputs.NewClientInputBuilder([]*inputs.ClientInput{provided}, state.GetCtx())
	if validationErr := secondBuilder.ValidateProvided(first.CurrentStepID); validationErr != nil {
		t.Fatalf("script input validation failed: %#v", validationErr)
	}
	second := newStepTransaction()
	second.State = state
	second.CacheManager = first.CacheManager
	second.ClientInputsBuilder = secondBuilder
	outcome, err = types.ExecuteStepConfig(&journeysteps.Script{}, second, config)
	if err != nil || outcome != "done" || state.GetCtx().Get("nickname").AsStringOr("") != "Ada" {
		t.Fatalf("resume outcome=%q nickname=%q err=%v", outcome, state.GetCtx().Get("nickname").AsStringOr(""), err)
	}
}

func TestScriptSchemaSupportsTypedPlaceholderTimeoutAndArgs(t *testing.T) {
	target := "00000000-0000-0000-0000-000000000001"
	step := &types.Step{StepType: types.ScriptStep, Config: map[string]any{
		"script_id":       "00000000-0000-0000-0000-000000000901",
		"timeout_seconds": "${ctx.timeout}",
		"args":            map[string]any{"payload": "${ctx.payload}"},
		"outcome":         map[string]any{"done": target},
	}}
	registry := journeysteps.GetDefaultStepRegistry()
	if err := types.GenerateStepVariables(step, registry); err != nil {
		t.Fatal(err)
	}
	if err := registry.ValidateStep(step); err != nil {
		t.Fatal(err)
	}
}

func TestScriptBindingsProviderCanExtendDefaults(t *testing.T) {
	transaction := newStepTransaction()
	scriptID := configureJourneyScript(t, transaction, `
        ctx.Set("customBinding", tenant + ":" + args.suffix);
        setOutcome("done");
    `)
	provider := scriptBindingsProviderFunc(func(context *journeysteps.ScriptBindingContext) (map[string]any, error) {
		if context.Transaction != transaction || context.Args["suffix"] != "journey" || context.DefaultBindings["ctx"] == nil {
			return nil, errors.New("binding context is incomplete")
		}
		return map[string]any{"tenant": "alpha"}, nil
	})
	if err := journeysteps.ConfigureScriptBindings(transaction.CacheManager, journeysteps.ScriptBindingsExtend, provider); err != nil {
		t.Fatal(err)
	}
	config := map[string]any{
		"script_id": scriptID, "timeout_seconds": 2, "args": map[string]any{"suffix": "journey"},
		"outcome": map[string]any{"done": "next"},
	}
	outcome, err := types.ExecuteStepConfig(&journeysteps.Script{}, transaction, config)
	if err != nil || outcome != "done" || transaction.State.GetCtx().Get("customBinding").AsStringOr("") != "alpha:journey" {
		t.Fatalf("outcome=%q custom=%q err=%v", outcome, transaction.State.GetCtx().Get("customBinding").AsStringOr(""), err)
	}
}

func TestScriptBindingsProviderCanReplaceDefaults(t *testing.T) {
	transaction := newStepTransaction()
	scriptID := configureJourneyScript(t, transaction, `
        if (typeof ctx !== "undefined") throw new Error("default ctx binding was retained");
        capture(customValue);
        setOutcome("done");
    `)
	captured := ""
	provider := scriptBindingsProviderFunc(func(context *journeysteps.ScriptBindingContext) (map[string]any, error) {
		return map[string]any{
			"customValue": "replacement",
			"capture":     func(value string) { captured = value },
			"setOutcome":  context.DefaultBindings["setOutcome"],
		}, nil
	})
	if err := journeysteps.ConfigureScriptBindings(transaction.CacheManager, journeysteps.ScriptBindingsReplace, provider); err != nil {
		t.Fatal(err)
	}
	outcome, err := types.ExecuteStepConfig(&journeysteps.Script{}, transaction, map[string]any{
		"script_id": scriptID, "timeout_seconds": 2, "outcome": map[string]any{"done": "next"},
	})
	if err != nil || outcome != "done" || captured != "replacement" {
		t.Fatalf("outcome=%q captured=%q err=%v", outcome, captured, err)
	}
}

func TestCustomScriptTypeCanProvideRuntimeBindingsAndDescriptors(t *testing.T) {
	const customType = "risk-score-test"
	if err := journeysteps.RegisterScriptType(journeysteps.ScriptTypeDefinition{
		Type:        customType,
		Name:        "Risk score script",
		Description: "Custom test script type.",
		Runnable:    true,
	}); err != nil {
		t.Fatal(err)
	}
	transaction := newStepTransaction()
	scriptID := configureScript(t, transaction, customType, `
        capture(customRisk());
        setOutcome("done");
    `)
	captured := ""
	provider := scriptDescriptorProvider{
		scriptRuntimeBindingsProviderFunc: func(context *journeysteps.ScriptBindingContext) (map[string]any, error) {
			if context.ScriptType != customType || context.DefaultBindings["setOutcome"] == nil || context.DefaultBindings["args"] == nil || context.DefaultBindings["ctx"] != nil {
				return nil, errors.New("custom script type received unexpected default bindings")
			}
			return map[string]any{
				"customRisk": func() int { return 7 },
				"capture":    func(value int) { captured = "risk" },
				"setOutcome": context.DefaultBindings["setOutcome"],
			}, nil
		},
		descriptors: []journeysteps.ScriptBindingDescriptor{
			{Name: "customRisk", Type: "function", Signature: "customRisk(): number"},
		},
	}
	if err := journeysteps.ConfigureScriptTypeBindings(transaction.CacheManager, customType, journeysteps.ScriptBindingsExtend, provider); err != nil {
		t.Fatal(err)
	}
	descriptors, err := journeysteps.ScriptBindingDescriptors(transaction.CacheManager, "alpha", customType, nil)
	if err != nil || len(descriptors) != 1 || descriptors[0].Name != "customRisk" {
		t.Fatalf("descriptors=%#v err=%v", descriptors, err)
	}
	outcome, err := types.ExecuteStepConfig(&journeysteps.Script{}, transaction, map[string]any{
		"script_id": scriptID, "timeout_seconds": 2, "outcome": map[string]any{"done": "next"},
	})
	if err != nil || outcome != "done" || captured != "risk" {
		t.Fatalf("outcome=%q captured=%q err=%v", outcome, captured, err)
	}
}

func TestHTTPBindingDescriptorsFollowRunnableDefaults(t *testing.T) {
	journeyDescriptors, err := journeysteps.ScriptBindingDescriptors(nil, "alpha", journeysteps.JourneyScript, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !hasScriptBindingDescriptor(journeyDescriptors, "http") {
		t.Fatal("journey script descriptors do not include http binding")
	}
	asyncDescriptors, err := journeysteps.ScriptBindingDescriptors(nil, "alpha", journeysteps.AsyncScript, nil)
	if err != nil {
		t.Fatal(err)
	}
	if hasScriptBindingDescriptor(asyncDescriptors, "http") {
		t.Fatal("async script descriptors should not include default http binding")
	}
}

func TestScheduleScriptPublishesOnlyExplicitResult(t *testing.T) {
	manager, _ := jsrun.NewDefaultStorage(t.TempDir())
	program, err := manager.CompileScript("schedule-result", `
		SetResult({ token: previousResult.token + "-next" })
		"ignored completion value"
	`)
	if err != nil {
		t.Fatal(err)
	}
	resultContext := types.NewScheduleResultContext(map[string]any{"token": "old"})
	bindings, err := journeysteps.ResolvedScheduleScriptBindings(
		context.Background(), nil, nil, "alpha", &jsrun.Script{Name: "schedule-result", Type: journeysteps.ScheduleScript}, nil, 2, resultContext,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.ExecuteWithBindings(program, bindings, 2*time.Second); err != nil {
		t.Fatal(err)
	}
	value, found := resultContext.Result()
	object, ok := value.(map[string]any)
	if !found || !ok || object["token"] != "old-next" {
		t.Fatalf("explicit result found=%v value=%#v", found, value)
	}

	withoutResult := types.NewScheduleResultContext(nil)
	program, err = manager.CompileScript("schedule-no-result", `crypto.NewUUID()`)
	if err != nil {
		t.Fatal(err)
	}
	bindings, err = journeysteps.ResolvedScheduleScriptBindings(
		context.Background(), nil, nil, "alpha", &jsrun.Script{Name: "schedule-no-result", Type: journeysteps.ScheduleScript}, nil, 2, withoutResult,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.ExecuteWithBindings(program, bindings, 2*time.Second); err != nil {
		t.Fatal(err)
	}
	if value, found := withoutResult.Result(); found {
		t.Fatalf("completion expression was treated as explicit result: %#v", value)
	}
}

func hasScriptBindingDescriptor(descriptors []journeysteps.ScriptBindingDescriptor, name string) bool {
	for _, descriptor := range descriptors {
		if descriptor.Name == name {
			return true
		}
	}
	return false
}
