package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	gojourney "github.com/nitsugaro/go-journey"
	jcache "github.com/nitsugaro/go-journey/cache"
	journeysteps "github.com/nitsugaro/go-journey/steps"
	"github.com/nitsugaro/go-journey/types"
	jsrun "github.com/nitsugaro/go-jsruntime/v2"
	"github.com/nitsugaro/go-utils/v2/encoding"
)

type restTestAPI struct {
	router       *gin.Engine
	successes    int
	failures     int
	successRealm string
}

type restTestTokens struct{}

func (restTestTokens) Sign(state *types.JourneyState) ([]byte, error) {
	return json.Marshal(state)
}

func (restTestTokens) Validate(token string) (*types.JourneyState, error) {
	var state types.JourneyState
	if err := json.Unmarshal([]byte(token), &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func newRESTTestAPI(t *testing.T) *restTestAPI {
	return newRESTTestAPIWithConfig(t, nil)
}

func newRESTTestAPIWithConfig(t *testing.T, configure func(*gojourney.RESTAPIConfig)) *restTestAPI {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	journeys, err := gojourney.NewJourneyStorage(t.TempDir(), journeysteps.GetDefaultStepRegistry())
	if err != nil {
		t.Fatal(err)
	}
	cacheManager, err := jcache.NewManager(&jcache.ManagerConfig{
		FolderPath: t.TempDir(),
		Caches: map[string]jcache.CacheConfig{
			journeysteps.HTTPClientCacheKey: {Factory: journeysteps.HTTPClientFactory, MaxInstances: 10},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	scriptManager, scriptStorage := jsrun.NewDefaultStorage(t.TempDir())
	if err := journeysteps.ConfigureScriptRuntime(cacheManager, scriptManager, scriptStorage); err != nil {
		t.Fatal(err)
	}
	schemaStorage, err := gojourney.NewDeveloperSchemaStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	scheduleStorage, err := gojourney.NewScheduleStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	result := &restTestAPI{router: router}
	adminMiddleware := func(context *gin.Context) {
		if context.GetHeader("X-Admin") != "yes" {
			context.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		context.Next()
	}
	invokeMiddleware := func(context *gin.Context) {
		context.Header("X-Journey-Invocation", "true")
		context.Next()
	}
	restConfig := &gojourney.RESTAPIConfig{
		Enabled: true, Router: router, BasePath: "/api/journey",
		AdminMiddleware: []gin.HandlerFunc{adminMiddleware}, InvocationMiddleware: []gin.HandlerFunc{invokeMiddleware},
		SchemaStorage:   schemaStorage,
		ScheduleStorage: scheduleStorage,
		MaxBodyBytes:    1 << 20,
		PrepareExecution: func(context *gin.Context, execution *types.JourneyExecute) error {
			expectedRealm := context.Param("realm")
			if value, found := context.Get("realm"); found {
				expectedRealm, _ = value.(string)
			}
			if execution.Payload.GetRealm() == nil || execution.Payload.GetRealm().Name != expectedRealm {
				t.Fatalf("realm was not propagated: payload=%+v expected=%q", execution.Payload.GetRealm(), expectedRealm)
			}
			return nil
		},
		OnSuccess: func(_ *gin.Context, state *types.JourneyState) {
			result.successes++
			result.successRealm = state.GetRealm()
		},
		OnFailure: func(_ *gin.Context, _ *types.JourneyState, _ error) { result.failures++ },
	}
	if configure != nil {
		configure(restConfig)
	}
	gojourney.NewManager(&gojourney.JourneyManagerConfig{
		JourneyStorage: journeys,
		CacheManager:   cacheManager,
		Tokens:         restTestTokens{},
		RESTAPI:        restConfig,
	})
	return result
}

func performRESTRequest(router http.Handler, method, path string, body any, admin bool) *httptest.ResponseRecorder {
	var data []byte
	if body != nil {
		if raw, ok := body.([]byte); ok {
			data = raw
		} else {
			data, _ = json.Marshal(body)
		}
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(data))
	request.Header.Set("Content-Type", "application/json")
	if admin {
		request.Header.Set("X-Admin", "yes")
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func performSingleRESTRequest(router http.Handler, method, path string, body any, admin bool) *httptest.ResponseRecorder {
	var data []byte
	if body != nil {
		if raw, ok := body.([]byte); ok {
			data = raw
		} else {
			data, _ = json.Marshal(body)
		}
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(data))
	request.Header.Set("Content-Type", "application/json")
	if admin {
		request.Header.Set("X-Admin", "yes")
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestRESTAPIServesEmbeddedUI(t *testing.T) {
	api := newRESTTestAPIWithConfig(t, func(config *gojourney.RESTAPIConfig) {
		config.UIEnabled = true
	})
	index := performSingleRESTRequest(api.router, http.MethodGet, "/journey-ui/", nil, false)
	if index.Code != http.StatusOK || !strings.Contains(index.Body.String(), `<div id="root"></div>`) ||
		!strings.Contains(index.Body.String(), `<base href="/journey-ui/">`) {
		t.Fatalf("ui index status=%d location=%s body=%s", index.Code, index.Header().Get("Location"), index.Body.String())
	}
	assetPath := regexp.MustCompile(`src="\.(/assets/[^"]+\.js)"`).FindStringSubmatch(index.Body.String())
	if len(assetPath) != 2 {
		t.Fatalf("ui index did not include js asset: %s", index.Body.String())
	}
	asset := performSingleRESTRequest(api.router, http.MethodGet, "/journey-ui"+assetPath[1], nil, false)
	if asset.Code != http.StatusOK || !strings.Contains(asset.Header().Get("Content-Type"), "javascript") {
		t.Fatalf("ui asset status=%d content-type=%s", asset.Code, asset.Header().Get("Content-Type"))
	}
	fallback := performSingleRESTRequest(api.router, http.MethodGet, "/journey-ui/journeys/new", nil, false)
	if fallback.Code != http.StatusOK || !strings.Contains(fallback.Body.String(), `<div id="root"></div>`) ||
		!strings.Contains(fallback.Body.String(), `<base href="/journey-ui/">`) {
		t.Fatalf("ui fallback status=%d body=%s", fallback.Code, fallback.Body.String())
	}
}

func TestRESTAPIInstanceCRUD(t *testing.T) {
	api := newRESTTestAPI(t)

	create := performRESTRequest(api.router, http.MethodPut, "/api/journey/alpha/instances/http_client/default", map[string]any{}, true)
	if create.Code != http.StatusCreated {
		t.Fatalf("create instance status=%d body=%s", create.Code, create.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created["cache_key"] != "http_client" || created["instance_id"] != "default" || created["persisted"] != true {
		t.Fatalf("unexpected created instance: %#v", created)
	}

	list := performRESTRequest(api.router, http.MethodGet, "/api/journey/alpha/instances?cache_key=http_client", nil, true)
	if list.Code != http.StatusOK {
		t.Fatalf("list instances status=%d body=%s", list.Code, list.Body.String())
	}
	if !strings.Contains(list.Body.String(), `"resultCount":1`) || !strings.Contains(list.Body.String(), `"caches"`) {
		t.Fatalf("unexpected list body: %s", list.Body.String())
	}

	get := performRESTRequest(api.router, http.MethodGet, "/api/journey/alpha/instances/http_client/default", nil, true)
	if get.Code != http.StatusOK || !strings.Contains(get.Body.String(), `"persisted":true`) {
		t.Fatalf("get instance status=%d body=%s", get.Code, get.Body.String())
	}

	remove := performRESTRequest(api.router, http.MethodDelete, "/api/journey/alpha/instances/http_client/default", nil, true)
	if remove.Code != http.StatusNoContent {
		t.Fatalf("delete instance status=%d body=%s", remove.Code, remove.Body.String())
	}

	missing := performRESTRequest(api.router, http.MethodGet, "/api/journey/alpha/instances/http_client/default", nil, true)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing instance status=%d body=%s", missing.Code, missing.Body.String())
	}
}

func TestOptionalRESTAPIJourneyCRUDAndExecution(t *testing.T) {
	api := newRESTTestAPI(t)
	if response := performRESTRequest(api.router, http.MethodGet, "/api/journey/configured", nil, false); response.Code != http.StatusUnauthorized {
		t.Fatalf("unprotected admin status=%d", response.Code)
	}
	journey := map[string]any{
		"name": "REST success", "active": true, "default_exp": 1, "realm": "payload-realm-must-be-overridden",
		"start_step_id": "start", "sub_entries": []string{},
		"steps": map[string]any{"start": map[string]any{
			"name": "Complete", "step_type": types.SuccessStep,
			"config": map[string]any{"data": map[string]any{"source": "rest"}},
		}},
	}
	created := performRESTRequest(api.router, http.MethodPut, "/api/journey/configured", journey, true)
	if created.Code != http.StatusCreated {
		t.Fatalf("save status=%d body=%s", created.Code, created.Body.String())
	}
	var stored types.JourneyConfiguration
	if err := json.Unmarshal(created.Body.Bytes(), &stored); err != nil || stored.ID == "" {
		t.Fatalf("saved journey=%#v err=%v", stored, err)
	}
	if stored.Realm != "configured" {
		t.Fatalf("realm from path was not applied: %q", stored.Realm)
	}
	get := performRESTRequest(api.router, http.MethodGet, "/api/journey/configured/"+stored.ID, nil, true)
	if get.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", get.Code, get.Body.String())
	}
	stored.Description = "updated"
	updated := performRESTRequest(api.router, http.MethodPut, "/api/journey/configured", &stored, true)
	if updated.Code != http.StatusOK || !strings.Contains(updated.Body.String(), "updated") {
		t.Fatalf("update status=%d body=%s", updated.Code, updated.Body.String())
	}
	duplicateWithoutID := journey
	duplicateWithoutID["description"] = "duplicate by name"
	duplicate := performRESTRequest(api.router, http.MethodPut, "/api/journey/configured", duplicateWithoutID, true)
	if duplicate.Code != http.StatusConflict || !strings.Contains(duplicate.Body.String(), "duplicate_journey") {
		t.Fatalf("duplicate status=%d body=%s", duplicate.Code, duplicate.Body.String())
	}
	list := performRESTRequest(api.router, http.MethodGet, "/api/journey/configured?name=REST&limit=1", nil, true)
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"resultCount":1`) || !strings.Contains(list.Body.String(), `"result":[`) {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
	wrongRealm := performRESTRequest(api.router, http.MethodPost, "/api/journey/random-realm/invoke", map[string]any{"journey_id": stored.ID}, false)
	if wrongRealm.Code != http.StatusNotFound || api.failures != 1 {
		t.Fatalf("wrong realm status=%d failures=%d body=%s", wrongRealm.Code, api.failures, wrongRealm.Body.String())
	}
	invoked := performRESTRequest(api.router, http.MethodPost, "/api/journey/configured/invoke", map[string]any{"journey_id": stored.ID}, false)
	if invoked.Code != http.StatusOK || invoked.Header().Get("X-Journey-Invocation") != "true" ||
		strings.Contains(invoked.Body.String(), `"ctx"`) || strings.Contains(invoked.Body.String(), `"status"`) ||
		strings.Contains(invoked.Body.String(), `"state"`) || api.successes != 1 || api.successRealm != "configured" {
		t.Fatalf("invoke status=%d body=%s successes=%d realm=%q", invoked.Code, invoked.Body.String(), api.successes, api.successRealm)
	}
	var successBody map[string]any
	if err := json.Unmarshal(invoked.Body.Bytes(), &successBody); err != nil ||
		successBody["realm"] != "configured" ||
		successBody["data"].(map[string]any)["source"] != "rest" {
		t.Fatalf("success terminal body=%#v err=%v", successBody, err)
	}
	missing := performRESTRequest(api.router, http.MethodPost, "/api/journey/configured/invoke", map[string]any{"journey_id": "missing"}, false)
	if missing.Code != http.StatusNotFound || api.failures != 2 {
		t.Fatalf("missing status=%d failures=%d body=%s", missing.Code, api.failures, missing.Body.String())
	}
	deleted := performRESTRequest(api.router, http.MethodDelete, "/api/journey/configured/"+stored.ID, nil, true)
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d body=%s", deleted.Code, deleted.Body.String())
	}
}

func TestRESTAPIResourceFallbackUsesFirstPathSegmentAsJourneyName(t *testing.T) {
	var expectedJourneyID string
	var observedPrefix string
	var observedResourcePath string
	var observedBody string
	api := newRESTTestAPIWithConfig(t, func(config *gojourney.RESTAPIConfig) {
		config.PrepareExecution = func(context *gin.Context, execution *types.JourneyExecute) error {
			if routeRealm := strings.TrimSpace(context.Param("realm")); routeRealm != "" {
				execution.Payload.SetRealm(&types.Realm{Name: routeRealm})
			}
			if expectedJourneyID != "" && execution.Payload.JourneyID != expectedJourneyID {
				t.Fatalf("route dispatcher selected journey %q, expected %q", execution.Payload.JourneyID, expectedJourneyID)
			}
			if execution.Request != nil {
				observedPrefix = execution.Request.RoutePrefixValue()
				observedResourcePath = execution.Request.RoutePathValue()
				body, _ := execution.Request.BodyBytes()
				observedBody = string(body)
			}
			if execution.Payload.GetRealm() == nil || execution.Payload.GetRealm().Name != "alpha" {
				t.Fatalf("resource journey realm was overwritten by route params: params=%#v realm=%#v", context.Params, execution.Payload.GetRealm())
			}
			return nil
		}
	})
	makeRouteJourney := func(name, marker string) map[string]any {
		responseID := "00000000-0000-0000-0000-000000000901"
		finishID := "00000000-0000-0000-0000-000000000902"
		return map[string]any{
			"name": name, "active": true, "default_exp": 1, "realm": "alpha", "journey_type": types.ResourceJourney,
			"start_step_id": responseID, "sub_entries": []string{},
			"steps": map[string]any{
				responseID: map[string]any{
					"name": "Route complete", "step_type": types.HTTPResponseStep,
					"config": map[string]any{
						"status_code":  202,
						"headers":      map[string]any{"X-Route-Marker": marker},
						"content_type": "JSON",
						"body":         map[string]any{"route": marker},
						"outcome":      map[string]any{"true": finishID},
					},
				},
				finishID: map[string]any{
					"name": "Finish response", "step_type": types.HTTPFinishResponseStep,
					"config": map[string]any{},
				},
			},
		}
	}
	shortCreated := performRESTRequest(api.router, http.MethodPut, "/api/journey/alpha", makeRouteJourney("server1", "short"), true)
	if shortCreated.Code != http.StatusCreated {
		t.Fatalf("create short route status=%d body=%s", shortCreated.Code, shortCreated.Body.String())
	}
	otherCreated := performRESTRequest(api.router, http.MethodPut, "/api/journey/alpha", makeRouteJourney("/server1-api", "other"), true)
	if otherCreated.Code != http.StatusCreated {
		t.Fatalf("create other route status=%d body=%s", otherCreated.Code, otherCreated.Body.String())
	}
	collisionCreated := performRESTRequest(api.router, http.MethodPut, "/api/journey/alpha", makeRouteJourney("alpha", "collision"), true)
	if collisionCreated.Code != http.StatusCreated {
		t.Fatalf("create colliding route status=%d body=%s", collisionCreated.Code, collisionCreated.Body.String())
	}
	registeredRoute := performRESTRequest(api.router, http.MethodGet, "/api/journey/alpha/scripts", nil, true)
	if registeredRoute.Code != http.StatusOK || registeredRoute.Header().Get("X-Route-Marker") != "" {
		t.Fatalf("registered route did not take priority: status=%d body=%s", registeredRoute.Code, registeredRoute.Body.String())
	}
	registeredInstances := performRESTRequest(api.router, http.MethodGet, "/api/journey/alpha/instances?limit=500", nil, true)
	if registeredInstances.Code != http.StatusOK || registeredInstances.Header().Get("X-Route-Marker") != "" ||
		!strings.Contains(registeredInstances.Body.String(), `"caches"`) {
		t.Fatalf("registered instances route did not take priority: status=%d body=%s", registeredInstances.Code, registeredInstances.Body.String())
	}
	var shortStored types.JourneyConfiguration
	if err := json.Unmarshal(shortCreated.Body.Bytes(), &shortStored); err != nil || shortStored.ID == "" {
		t.Fatalf("stored resource route=%#v err=%v", shortStored, err)
	}
	expectedJourneyID = shortStored.ID
	invoked := performRESTRequest(api.router, http.MethodPost, "/api/journey/server1/api/users/42?debug=true", []byte(`raw body`), false)
	if invoked.Code != http.StatusAccepted || invoked.Header().Get("X-Route-Marker") != "short" || !strings.Contains(invoked.Body.String(), `"route":"short"`) ||
		observedPrefix != "/server1" || observedResourcePath != "/api/users/42" || observedBody != "raw body" {
		t.Fatalf("route invoke status=%d body=%s prefix=%q path=%q body=%q", invoked.Code, invoked.Body.String(), observedPrefix, observedResourcePath, observedBody)
	}
	second := performRESTRequest(api.router, http.MethodGet, "/api/journey/server1/status", nil, true)
	if second.Code != http.StatusAccepted || observedPrefix != "/server1" || observedResourcePath != "/status" {
		t.Fatalf("cached route status=%d body=%s prefix=%q path=%q", second.Code, second.Body.String(), observedPrefix, observedResourcePath)
	}
	missing := performRESTRequest(api.router, http.MethodGet, "/api/journey/unknown/path", nil, true)
	if missing.Code != http.StatusNotFound || missing.Body.Len() != 0 {
		t.Fatalf("missing route status=%d body=%s", missing.Code, missing.Body.String())
	}
}

func TestRESTAPIInvokesAndResumesInteractiveJourney(t *testing.T) {
	api := newRESTTestAPI(t)
	formID := "00000000-0000-0000-0000-000000000201"
	successID := "00000000-0000-0000-0000-000000000202"
	journey := map[string]any{
		"name": "REST form", "active": true, "default_exp": 1, "realm": "rest-realm",
		"start_step_id": formID, "sub_entries": []string{},
		"steps": map[string]any{
			formID: map[string]any{
				"name": "Collect nickname", "step_type": types.FormStep,
				"config": map[string]any{
					"inputs": []any{map[string]any{
						"id": "nickname", "external_id": "profile.nickname", "type": "string", "required": true,
					}},
					"context": "ctx", "outcome": map[string]any{"true": successID},
				},
			},
			successID: map[string]any{"name": "Complete", "step_type": types.SuccessStep, "config": map[string]any{}},
		},
	}
	created := performRESTRequest(api.router, http.MethodPut, "/api/journey/rest-realm", journey, true)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	var stored types.JourneyConfiguration
	if err := json.Unmarshal(created.Body.Bytes(), &stored); err != nil {
		t.Fatal(err)
	}
	pausedResponse := performRESTRequest(api.router, http.MethodPost, "/api/journey/rest-realm/invoke", map[string]any{"journey_id": stored.ID}, false)
	var paused types.JourneyPayloadReq
	if err := json.Unmarshal(pausedResponse.Body.Bytes(), &paused); err != nil || pausedResponse.Code != http.StatusOK ||
		len(paused.ClientInputs) != 1 || paused.Jwt == "" || strings.Contains(pausedResponse.Body.String(), `"status"`) ||
		strings.Contains(pausedResponse.Body.String(), `"response"`) {
		t.Fatalf("paused status=%d body=%s parsed=%#v err=%v", pausedResponse.Code, pausedResponse.Body.String(), paused, err)
	}
	paused.ClientInputs[0].Input = "Ada"
	completed := performRESTRequest(api.router, http.MethodPost, "/api/journey/rest-realm/invoke", &paused, false)
	if completed.Code != http.StatusOK || strings.Contains(completed.Body.String(), `"nickname":"Ada"`) ||
		strings.Contains(completed.Body.String(), `"ctx"`) || strings.Contains(completed.Body.String(), `"status"`) ||
		strings.Contains(completed.Body.String(), `"state"`) || api.successes != 1 {
		t.Fatalf("completed status=%d body=%s successes=%d", completed.Code, completed.Body.String(), api.successes)
	}
}

func TestRESTAPIJourneyFailureReturnsUnauthorized(t *testing.T) {
	api := newRESTTestAPI(t)
	journey := map[string]any{
		"name": "REST failure", "active": true, "default_exp": 1, "realm": "rest-realm",
		"start_step_id": "failure", "sub_entries": []string{},
		"steps": map[string]any{
			"failure": map[string]any{"name": "Unauthorized", "step_type": types.FailureStep, "config": map[string]any{
				"failure_url": "/login",
				"data":        map[string]any{"reason": "invalid_credentials"},
			}},
		},
	}
	created := performRESTRequest(api.router, http.MethodPut, "/api/journey/rest-realm", journey, true)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	var stored types.JourneyConfiguration
	if err := json.Unmarshal(created.Body.Bytes(), &stored); err != nil || stored.ID == "" {
		t.Fatalf("stored=%#v err=%v", stored, err)
	}
	response := performRESTRequest(api.router, http.MethodPost, "/api/journey/rest-realm/invoke", map[string]any{"journey_id": stored.ID}, false)
	if response.Code != http.StatusUnauthorized || strings.Contains(response.Body.String(), "journey_failed") || api.failures != 1 {
		t.Fatalf("failure status=%d failures=%d body=%s", response.Code, api.failures, response.Body.String())
	}
	var failureBody map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &failureBody); err != nil ||
		failureBody["realm"] != "rest-realm" ||
		failureBody["failure_url"] != "/login" ||
		failureBody["data"].(map[string]any)["reason"] != "invalid_credentials" {
		t.Fatalf("failure terminal body=%#v err=%v", failureBody, err)
	}
}

func TestRESTAPIStepSchemas(t *testing.T) {
	api := newRESTTestAPI(t)
	if response := performRESTRequest(api.router, http.MethodGet, "/api/journey/step-schemas", nil, false); response.Code != http.StatusUnauthorized {
		t.Fatalf("unprotected schemas status=%d", response.Code)
	}
	list := performRESTRequest(api.router, http.MethodGet, "/api/journey/step-schemas?type=If&limit=1", nil, true)
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"resultCount":1`) ||
		!strings.Contains(list.Body.String(), `"step_type":"IfExpression"`) ||
		!strings.Contains(list.Body.String(), `"schema"`) {
		t.Fatalf("list schemas status=%d body=%s", list.Code, list.Body.String())
	}
	schema := performRESTRequest(api.router, http.MethodGet, "/api/journey/step-schemas/IfExpression", nil, true)
	if schema.Code != http.StatusOK || !strings.Contains(schema.Body.String(), `"properties"`) ||
		!strings.Contains(schema.Body.String(), `"expression"`) {
		t.Fatalf("get schema status=%d body=%s", schema.Code, schema.Body.String())
	}
	missing := performRESTRequest(api.router, http.MethodGet, "/api/journey/step-schemas/MissingStep", nil, true)
	if missing.Code != http.StatusNotFound || !strings.Contains(missing.Body.String(), "step_schema_not_found") {
		t.Fatalf("missing schema status=%d body=%s", missing.Code, missing.Body.String())
	}
}

func TestRESTAPICustomRoutes(t *testing.T) {
	api := newRESTTestAPIWithConfig(t, func(config *gojourney.RESTAPIConfig) {
		config.Routes = gojourney.RESTAPIRoutes{
			Journeys:           []string{"/:realm/flows"},
			JourneyItems:       []string{"/:realm/flows/:journeyId"},
			Scripts:            []string{"/:realm/script-routes"},
			ScriptItems:        []string{"/:realm/script-routes/:scriptId"},
			ScriptBindings:     []string{"/:realm/script-routes/:scriptId/bindings"},
			ScriptTypeBindings: []string{"/:realm/script-bindings"},
			StepSchemas:        []string{"/schemas"},
			StepSchemaItems:    []string{"/schemas/:stepType"},
			Invoke:             []string{"/:realm/run"},
		}
	})
	journey := map[string]any{
		"name": "Custom routes", "active": true, "default_exp": 1, "realm": "alpha",
		"start_step_id": "start", "sub_entries": []string{},
		"steps": map[string]any{"start": map[string]any{
			"name": "Complete", "step_type": types.SuccessStep, "config": map[string]any{
				"data": map[string]any{"route": "custom"},
			},
		}},
	}
	created := performRESTRequest(api.router, http.MethodPut, "/api/journey/alpha/flows", journey, true)
	if created.Code != http.StatusCreated {
		t.Fatalf("custom journey save status=%d body=%s", created.Code, created.Body.String())
	}
	var stored types.JourneyConfiguration
	if err := json.Unmarshal(created.Body.Bytes(), &stored); err != nil || stored.ID == "" {
		t.Fatalf("stored=%#v err=%v", stored, err)
	}
	if response := performRESTRequest(api.router, http.MethodGet, "/api/journey/schemas/Success", nil, true); response.Code != http.StatusOK {
		t.Fatalf("custom schema status=%d body=%s", response.Code, response.Body.String())
	}
	if response := performRESTRequest(api.router, http.MethodGet, "/api/journey/alpha/script-routes", nil, true); response.Code != http.StatusOK {
		t.Fatalf("custom script list status=%d body=%s", response.Code, response.Body.String())
	}
	if response := performRESTRequest(api.router, http.MethodGet, "/api/journey/alpha/script-bindings?type="+journeysteps.JourneyScript, nil, true); response.Code != http.StatusOK {
		t.Fatalf("custom script bindings status=%d body=%s", response.Code, response.Body.String())
	}
	invoked := performRESTRequest(api.router, http.MethodPost, "/api/journey/alpha/run", map[string]any{"journey_id": stored.ID}, false)
	if invoked.Code != http.StatusOK || !strings.Contains(invoked.Body.String(), `"route":"custom"`) {
		t.Fatalf("custom invoke status=%d body=%s", invoked.Code, invoked.Body.String())
	}
}

func TestRESTAPIRegistersDynamicJourneyItemsAfterSpecificRoutes(t *testing.T) {
	api := newRESTTestAPIWithConfig(t, func(config *gojourney.RESTAPIConfig) {
		config.Routes = gojourney.RESTAPIRoutes{
			Journeys:        []string{"/:realm"},
			JourneyItems:    []string{"/:realm/:journeyId"},
			Scripts:         []string{"/:realm/scripts"},
			ScriptItems:     []string{"/:realm/scripts/:scriptId"},
			StepSchemas:     []string{"/step-schemas"},
			StepSchemaItems: []string{"/step-schemas/:stepType"},
			Invoke:          []string{"/:realm/invoke"},
		}
	})
	journey := map[string]any{
		"name": "Compact routes", "active": true, "default_exp": 1, "realm": "alpha",
		"start_step_id": "start", "sub_entries": []string{},
		"steps": map[string]any{"start": map[string]any{
			"name": "Complete", "step_type": types.SuccessStep, "config": map[string]any{
				"data": map[string]any{"route": "compact"},
			},
		}},
	}
	created := performRESTRequest(api.router, http.MethodPut, "/api/journey/alpha", journey, true)
	if created.Code != http.StatusCreated {
		t.Fatalf("compact journey save status=%d body=%s", created.Code, created.Body.String())
	}
	var stored types.JourneyConfiguration
	if err := json.Unmarshal(created.Body.Bytes(), &stored); err != nil || stored.ID == "" {
		t.Fatalf("stored=%#v err=%v", stored, err)
	}
	if response := performRESTRequest(api.router, http.MethodGet, "/api/journey/alpha/scripts", nil, true); response.Code != http.StatusOK {
		t.Fatalf("compact script list status=%d body=%s", response.Code, response.Body.String())
	}
	if response := performRESTRequest(api.router, http.MethodGet, "/api/journey/step-schemas/Success", nil, true); response.Code != http.StatusOK {
		t.Fatalf("compact schema status=%d body=%s", response.Code, response.Body.String())
	}
	if response := performRESTRequest(api.router, http.MethodGet, "/api/journey/alpha/"+stored.ID, nil, true); response.Code != http.StatusOK {
		t.Fatalf("compact journey get status=%d body=%s", response.Code, response.Body.String())
	}
	invoked := performRESTRequest(api.router, http.MethodPost, "/api/journey/alpha/invoke", map[string]any{"journey_id": stored.ID}, false)
	if invoked.Code != http.StatusOK || !strings.Contains(invoked.Body.String(), `"route":"compact"`) {
		t.Fatalf("compact invoke status=%d body=%s", invoked.Code, invoked.Body.String())
	}
}

func TestRESTAPIUsesContextRealmWhenRoutesDoNotHaveRealmParam(t *testing.T) {
	api := newRESTTestAPIWithConfig(t, func(config *gojourney.RESTAPIConfig) {
		config.Middleware = []gin.HandlerFunc{
			func(context *gin.Context) {
				context.Set("realm", "host-realm")
				context.Next()
			},
		}
		config.Routes = gojourney.RESTAPIRoutes{
			Journeys:        []string{"/journeys"},
			JourneyItems:    []string{"/journeys/:journeyId"},
			Scripts:         []string{"/scripts"},
			ScriptItems:     []string{"/scripts/:scriptId"},
			StepSchemas:     []string{"/step-schemas"},
			StepSchemaItems: []string{"/step-schemas/:stepType"},
			Invoke:          []string{"/invoke"},
		}
	})
	journey := map[string]any{
		"name": "Host realm", "active": true, "default_exp": 1, "realm": "ignored-body-realm",
		"start_step_id": "start", "sub_entries": []string{},
		"steps": map[string]any{"start": map[string]any{
			"name": "Complete", "step_type": types.SuccessStep, "config": map[string]any{
				"data": map[string]any{"realm_source": "context"},
			},
		}},
	}
	created := performRESTRequest(api.router, http.MethodPut, "/api/journey/journeys", journey, true)
	if created.Code != http.StatusCreated {
		t.Fatalf("host realm save status=%d body=%s", created.Code, created.Body.String())
	}
	var stored types.JourneyConfiguration
	if err := json.Unmarshal(created.Body.Bytes(), &stored); err != nil || stored.ID == "" {
		t.Fatalf("stored=%#v err=%v", stored, err)
	}
	if stored.Realm != "host-realm" {
		t.Fatalf("expected context realm to override body/path realm, got %q", stored.Realm)
	}
	if response := performRESTRequest(api.router, http.MethodGet, "/api/journey/journeys/"+stored.ID, nil, true); response.Code != http.StatusOK {
		t.Fatalf("host realm get status=%d body=%s", response.Code, response.Body.String())
	}
	invoked := performRESTRequest(api.router, http.MethodPost, "/api/journey/invoke", map[string]any{"journey_id": stored.ID}, false)
	if invoked.Code != http.StatusOK || !strings.Contains(invoked.Body.String(), `"realm":"host-realm"`) ||
		!strings.Contains(invoked.Body.String(), `"realm_source":"context"`) {
		t.Fatalf("host realm invoke status=%d body=%s", invoked.Code, invoked.Body.String())
	}
}

func TestRESTAPIJourneyReturnVarsFlag(t *testing.T) {
	journeyWithPlaceholder := func(name string) map[string]any {
		return map[string]any{
			"name": name, "active": true, "default_exp": 1, "realm": "vars-realm",
			"start_step_id": "start", "sub_entries": []string{},
			"steps": map[string]any{"start": map[string]any{
				"name": "Complete", "step_type": types.SuccessStep,
				"config": map[string]any{"data": map[string]any{"message": "Hello ${ctx.name}"}},
			}},
		}
	}

	defaultAPI := newRESTTestAPI(t)
	created := performRESTRequest(defaultAPI.router, http.MethodPut, "/api/journey/vars-realm", journeyWithPlaceholder("Hidden vars"), true)
	if created.Code != http.StatusCreated || strings.Contains(created.Body.String(), `"vars"`) {
		t.Fatalf("default return_vars status=%d body=%s", created.Code, created.Body.String())
	}
	var hiddenStored types.JourneyConfiguration
	if err := json.Unmarshal(created.Body.Bytes(), &hiddenStored); err != nil || hiddenStored.ID == "" {
		t.Fatalf("hidden stored=%#v err=%v", hiddenStored, err)
	}
	readHidden := performRESTRequest(defaultAPI.router, http.MethodGet, "/api/journey/vars-realm/"+hiddenStored.ID, nil, true)
	if readHidden.Code != http.StatusOK || strings.Contains(readHidden.Body.String(), `"vars"`) {
		t.Fatalf("hidden read status=%d body=%s", readHidden.Code, readHidden.Body.String())
	}
	listHidden := performRESTRequest(defaultAPI.router, http.MethodGet, "/api/journey/vars-realm?name=Hidden", nil, true)
	if listHidden.Code != http.StatusOK || strings.Contains(listHidden.Body.String(), `"vars"`) {
		t.Fatalf("hidden list status=%d body=%s", listHidden.Code, listHidden.Body.String())
	}

	varsAPI := newRESTTestAPIWithConfig(t, func(config *gojourney.RESTAPIConfig) {
		config.ReturnVars = true
	})
	returned := performRESTRequest(varsAPI.router, http.MethodPut, "/api/journey/vars-realm", journeyWithPlaceholder("Returned vars"), true)
	if returned.Code != http.StatusCreated || !strings.Contains(returned.Body.String(), `"vars"`) ||
		!strings.Contains(returned.Body.String(), `"ctx.name"`) {
		t.Fatalf("enabled return_vars status=%d body=%s", returned.Code, returned.Body.String())
	}
}

func TestRESTAPIRoutesRequireSpecificIDs(t *testing.T) {
	assertInvalidRoutes := func(name string, routes gojourney.RESTAPIRoutes, expected string) {
		t.Helper()
		defer func() {
			recovered := recover()
			message, _ := recovered.(string)
			if recovered == nil || !strings.Contains(message, expected) {
				t.Fatalf("%s: expected invalid route panic containing %q, got %#v", name, expected, recovered)
			}
		}()
		_ = newRESTTestAPIWithConfig(t, func(config *gojourney.RESTAPIConfig) {
			config.Routes = routes
		})
	}
	validBase := gojourney.RESTAPIRoutes{
		Journeys:        []string{"/:realm/flows"},
		JourneyItems:    []string{"/:realm/flows/:journeyId"},
		Scripts:         []string{"/:realm/scripts"},
		ScriptItems:     []string{"/:realm/scripts/:scriptId"},
		StepSchemas:     []string{"/schemas"},
		StepSchemaItems: []string{"/schemas/:stepType"},
		Invoke:          []string{"/:realm/run"},
	}
	missingScriptID := validBase
	missingScriptID.ScriptItems = []string{"/:realm/scripts"}
	assertInvalidRoutes("missing script id", missingScriptID, "must include :scriptId")
}

func TestOptionalRESTAPIScriptCRUD(t *testing.T) {
	api := newRESTTestAPI(t)
	invalid := performRESTRequest(api.router, http.MethodPut, "/api/journey/rest-realm/scripts", map[string]any{
		"name": "invalid", "type": journeysteps.JourneyScript,
		"code_base64": encoding.EncodeBase64([]byte(`function (`)),
	}, true)
	if invalid.Code != http.StatusBadRequest || !strings.Contains(invalid.Body.String(), "invalid_script") {
		t.Fatalf("invalid status=%d body=%s", invalid.Code, invalid.Body.String())
	}
	script := map[string]any{
		"name": "rest-script", "type": journeysteps.JourneyScript,
		"code_base64": encoding.EncodeBase64([]byte(`setOutcome("done")`)),
	}
	created := performRESTRequest(api.router, http.MethodPut, "/api/journey/rest-realm/scripts", script, true)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	var stored jsrun.Script
	if err := json.Unmarshal(created.Body.Bytes(), &stored); err != nil || stored.ID == "" {
		t.Fatalf("created script=%#v err=%v", stored, err)
	}
	if response := performRESTRequest(api.router, http.MethodGet, "/api/journey/rest-realm/scripts/"+stored.ID, nil, true); response.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", response.Code, response.Body.String())
	}
	stored.Name = "rest-script-updated"
	if response := performRESTRequest(api.router, http.MethodPut, "/api/journey/rest-realm/scripts", &stored, true); response.Code != http.StatusOK {
		t.Fatalf("update status=%d body=%s", response.Code, response.Body.String())
	}
	upsertByName := map[string]any{
		"name": "rest-script-updated", "type": journeysteps.JourneyScript,
		"code_base64": encoding.EncodeBase64([]byte(`setOutcome("again")`)),
	}
	if response := performRESTRequest(api.router, http.MethodPut, "/api/journey/rest-realm/scripts", upsertByName, true); response.Code != http.StatusConflict ||
		!strings.Contains(response.Body.String(), "duplicate_script") {
		t.Fatalf("duplicate script status=%d body=%s", response.Code, response.Body.String())
	}
	if response := performRESTRequest(api.router, http.MethodGet, "/api/journey/rest-realm/scripts?name=updated&type="+journeysteps.JourneyScript+"&limit=1", nil, true); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"resultCount":1`) {
		t.Fatalf("list status=%d body=%s", response.Code, response.Body.String())
	}
	if response := performRESTRequest(api.router, http.MethodDelete, "/api/journey/rest-realm/scripts/"+stored.ID, nil, true); response.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestRESTAPIScriptBindingDescriptors(t *testing.T) {
	api := newRESTTestAPI(t)
	generic := performRESTRequest(api.router, http.MethodGet, "/api/journey/rest-realm/script-bindings?type="+journeysteps.JourneyScript, nil, true)
	if generic.Code != http.StatusOK || !strings.Contains(generic.Body.String(), `"name":"setOutcome"`) ||
		!strings.Contains(generic.Body.String(), `"name":"requestQuery"`) ||
		!strings.Contains(generic.Body.String(), `"name":"First"`) {
		t.Fatalf("generic script bindings status=%d body=%s", generic.Code, generic.Body.String())
	}
	allSets := performRESTRequest(api.router, http.MethodGet, "/api/journey/rest-realm/script-bindings", nil, true)
	if allSets.Code != http.StatusOK ||
		!strings.Contains(allSets.Body.String(), `"type":"auth"`) ||
		!strings.Contains(allSets.Body.String(), `"type":"resource"`) {
		t.Fatalf("script binding sets status=%d body=%s", allSets.Code, allSets.Body.String())
	}
	invalid := performRESTRequest(api.router, http.MethodGet, "/api/journey/rest-realm/script-bindings?type=unknown", nil, true)
	if invalid.Code != http.StatusOK || !strings.Contains(invalid.Body.String(), `"type":"unknown"`) || !strings.Contains(invalid.Body.String(), `"bindings":[]`) {
		t.Fatalf("invalid script binding status=%d body=%s", invalid.Code, invalid.Body.String())
	}
	script := map[string]any{
		"name": "binding-script", "type": journeysteps.JourneyScript,
		"code_base64": encoding.EncodeBase64([]byte(`setOutcome("done")`)),
	}
	created := performRESTRequest(api.router, http.MethodPut, "/api/journey/rest-realm/scripts", script, true)
	if created.Code != http.StatusCreated {
		t.Fatalf("create script status=%d body=%s", created.Code, created.Body.String())
	}
	var stored jsrun.Script
	if err := json.Unmarshal(created.Body.Bytes(), &stored); err != nil || stored.ID == "" {
		t.Fatalf("created script=%#v err=%v", stored, err)
	}
	specific := performRESTRequest(api.router, http.MethodGet, "/api/journey/rest-realm/scripts/"+stored.ID+"/bindings", nil, true)
	if specific.Code != http.StatusOK || !strings.Contains(specific.Body.String(), `"name":"crypto"`) ||
		!strings.Contains(specific.Body.String(), `"name":"SHA256"`) {
		t.Fatalf("specific script bindings status=%d body=%s", specific.Code, specific.Body.String())
	}
}

func TestRESTAPIBodyLimit(t *testing.T) {
	api := newRESTTestAPI(t)
	oversized := []byte(`{"journey_id":"` + strings.Repeat("x", (1<<20)+1) + `"}`)
	response := performRESTRequest(api.router, http.MethodPost, "/api/journey/rest-realm/invoke", oversized, false)
	if response.Code != http.StatusRequestEntityTooLarge || !strings.Contains(response.Body.String(), "body_too_large") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
