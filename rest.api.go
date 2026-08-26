package gojourney

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/gin-gonic/gin"
	jcache "github.com/nitsugaro/go-journey/cache"
	"github.com/nitsugaro/go-journey/env"
	"github.com/nitsugaro/go-journey/steps"
	"github.com/nitsugaro/go-journey/types"
	jsrun "github.com/nitsugaro/go-jsruntime/v2"
	"github.com/nitsugaro/go-nstore"
)

const defaultRESTMaxBodyBytes int64 = 1 << 20

const (
	restEventErrorKey = "gojourney.rest.error"
	restEventCodeKey  = "gojourney.rest.error_code"
)

type restResponseMutator struct {
	context     *gin.Context
	headers     http.Header
	cookies     []*http.Cookie
	statusCode  int
	contentType string
	body        []byte
	bodySet     bool
}

func newRESTResponseMutator(context *gin.Context) *restResponseMutator {
	return &restResponseMutator{context: context, headers: http.Header{}}
}

func (response *restResponseMutator) Header(name string, value string) {
	if response == nil || strings.TrimSpace(name) == "" {
		return
	}
	response.headers.Set(name, value)
}

func (response *restResponseMutator) AddHeader(name string, value string) {
	if response == nil || strings.TrimSpace(name) == "" {
		return
	}
	response.headers.Add(name, value)
}

func (response *restResponseMutator) SetCookie(cookie *http.Cookie) {
	if response == nil || cookie == nil {
		return
	}
	copy := *cookie
	response.cookies = append(response.cookies, &copy)
}

func (response *restResponseMutator) Status(code int) {
	if response == nil {
		return
	}
	response.statusCode = code
}

func (response *restResponseMutator) Body(contentType string, data []byte) {
	if response == nil {
		return
	}
	response.contentType = contentType
	response.body = append([]byte(nil), data...)
	response.bodySet = true
}

func (response *restResponseMutator) applyHeaders() {
	if response == nil || response.context == nil {
		return
	}
	for key, values := range response.headers {
		response.context.Writer.Header().Del(key)
		for _, value := range values {
			response.context.Writer.Header().Add(key, value)
		}
	}
	for _, cookie := range response.cookies {
		http.SetCookie(response.context.Writer, cookie)
	}
}

func (response *restResponseMutator) applyTerminal() bool {
	if response == nil || response.context == nil {
		return false
	}
	response.applyHeaders()
	if response.bodySet {
		status := response.statusCode
		if status == 0 {
			status = http.StatusOK
		}
		response.context.Data(status, response.contentType, response.body)
		return true
	}
	if response.statusCode != 0 {
		response.context.Status(response.statusCode)
		return true
	}
	return false
}

type RESTJourneyStorage interface {
	JourneyConfigurations
	Save(*types.JourneyConfiguration) error
	Delete(string) error
	ListOfCache() []*types.JourneyConfiguration
}

type RESTJourneyNameLookup interface {
	GetJourneyByNameRealm(name string, realm string) (*types.JourneyConfiguration, bool)
}

type RESTScriptStorage interface {
	Load(string) (*jsrun.Script, error)
	Save(*jsrun.Script) error
	Delete(string) error
	ListOfCache() []*jsrun.Script
}

type RESTSchemaStorage interface {
	types.DeveloperSchemaProvider
	Save(*types.DeveloperSchema) error
	Delete(string) error
}

type RESTAPIConfig struct {
	Enabled              bool
	Router               gin.IRouter
	BasePath             string
	Routes               RESTAPIRoutes
	UIEnabled            bool
	UIPath               string
	UIFileSystem         http.FileSystem
	Middleware           []gin.HandlerFunc
	AdminMiddleware      []gin.HandlerFunc
	InvocationMiddleware []gin.HandlerFunc
	JourneyStorage       RESTJourneyStorage
	ScriptStorage        RESTScriptStorage
	SchemaStorage        RESTSchemaStorage
	ScheduleStorage      RESTScheduleStorage
	MaxBodyBytes         int64
	ReturnVars           bool
	PrepareExecution     func(*gin.Context, *types.JourneyExecute) error
	OnSuccess            func(*gin.Context, *types.JourneyState)
	OnFailure            func(*gin.Context, *types.JourneyState, error)
}

type RESTAPIRoutes struct {
	Journeys            []string
	JourneyItems        []string
	Scripts             []string
	ScriptItems         []string
	ScriptBindings      []string
	ScriptTypeBindings  []string
	Schemas             []string
	SchemaItems         []string
	Schedules           []string
	ScheduleItems       []string
	ScheduleTriggers    []string
	Instances           []string
	InstanceItems       []string
	InstanceSchemas     []string
	InstanceSchemaItems []string
	StepSchemas         []string
	StepSchemaItems     []string
	Invoke              []string
	// RouteInvoke is retained for source compatibility. Resource journeys now use the REST NoRoute fallback.
	RouteInvoke []string
}

type restAPI struct {
	manager       *journeyManager
	config        *RESTAPIConfig
	journeys      RESTJourneyStorage
	scripts       RESTScriptStorage
	schemas       RESTSchemaStorage
	schedules     RESTScheduleStorage
	scriptManager interface{ DeleteFromCache(string) }
	maxBodyBytes  int64
	routeCache    *journeyRouteCache
	basePath      string
}

type journeyRouteCache struct {
	mu      sync.RWMutex
	entries map[string]string
}

type restRouteJourneyMatch struct {
	Journey      *types.JourneyConfiguration
	RoutePrefix  string
	ResourcePath string
}

type restQueryResponse[T any] struct {
	Result      []T `json:"result"`
	ResultCount int `json:"resultCount"`
}

type restErrorBody struct {
	Error restError `json:"error"`
}

type restError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (jm *journeyManager) registerRESTAPI(config *RESTAPIConfig) error {
	if config == nil || !config.Enabled {
		return nil
	}
	if config.Router == nil {
		return errors.New("REST API router is required")
	}
	journeys := config.JourneyStorage
	if journeys == nil {
		journeys, _ = jm.storage.(RESTJourneyStorage)
	}
	if journeys == nil {
		return errors.New("REST API requires an administrative journey storage")
	}
	scripts := config.ScriptStorage
	if scripts == nil {
		value, found := jm.cacheManager.GetCacheInstance(steps.ScriptStorageCacheKey, jcache.DefaultInstanceID)
		if found {
			scripts, _ = value.(RESTScriptStorage)
		}
	}
	if scripts == nil {
		return errors.New("REST API requires an administrative script storage")
	}
	schemas := config.SchemaStorage
	if schemas == nil {
		schemas, _ = jm.schemas.(RESTSchemaStorage)
	}
	if schemas == nil {
		value, found := jm.cacheManager.GetCacheInstance(steps.SchemaStorageCacheKey, jcache.DefaultInstanceID)
		if found {
			schemas, _ = value.(RESTSchemaStorage)
		}
	}
	if schemas == nil {
		return errors.New("REST API requires an administrative schema storage")
	}
	schedules := config.ScheduleStorage
	if schedules == nil {
		schedules = jm.scheduleStorage
	}
	var scriptManager interface{ DeleteFromCache(string) }
	if value, found := jm.cacheManager.GetCacheInstance(steps.ScriptManagerCacheKey, jcache.DefaultInstanceID); found {
		scriptManager, _ = value.(interface{ DeleteFromCache(string) })
	}
	maxBodyBytes := config.MaxBodyBytes
	if maxBodyBytes <= 0 {
		maxBodyBytes = defaultRESTMaxBodyBytes
	}
	if restUIEnabled(config) {
		registerRESTUI(config.Router, restUIPath(config), config.UIFileSystem)
	}
	api := &restAPI{
		manager: jm, config: config, journeys: journeys, scripts: scripts, schemas: schemas, schedules: schedules,
		scriptManager: scriptManager, maxBodyBytes: maxBodyBytes, routeCache: newJourneyRouteCache(),
	}
	basePath := strings.TrimSpace(config.BasePath)
	if basePath == "" {
		basePath = restConfigString("rest_api.base_path", "/journey")
	}
	api.basePath = normalizeRESTBasePath(basePath)
	group := config.Router.Group(basePath, config.Middleware...)
	group.Use(api.accessEventMiddleware())
	admin := group.Group("", config.AdminMiddleware...)
	routes, err := restAPIRoutes(config.Routes)
	if err != nil {
		return err
	}
	for _, route := range routes.StepSchemas {
		admin.GET(route, api.listStepSchemas)
	}
	for _, route := range routes.StepSchemaItems {
		admin.GET(route, api.getStepSchema)
	}
	for _, route := range routes.InstanceSchemas {
		admin.GET(route, api.listInstanceSchemas)
	}
	for _, route := range routes.InstanceSchemaItems {
		admin.GET(route, api.getInstanceSchema)
	}
	for _, route := range routes.Journeys {
		admin.GET(route, api.listJourneys)
		admin.PUT(route, api.saveJourney)
	}
	for _, route := range routes.Scripts {
		admin.GET(route, api.listScripts)
		admin.PUT(route, api.saveScript)
	}
	for _, route := range routes.ScriptTypeBindings {
		admin.GET(route, api.listScriptTypeBindings)
	}
	for _, route := range routes.ScriptBindings {
		admin.GET(route, api.getScriptBindings)
	}
	for _, route := range routes.Schemas {
		admin.GET(route, api.listSchemas)
		admin.PUT(route, api.saveSchema)
	}
	if api.schedules != nil {
		for _, route := range routes.Schedules {
			admin.GET(route, api.listSchedules)
			admin.PUT(route, api.saveSchedule)
		}
	}
	for _, route := range routes.SchemaItems {
		admin.GET(route, api.getSchema)
		admin.DELETE(route, api.deleteSchema)
	}
	if api.schedules != nil {
		for _, route := range routes.ScheduleItems {
			admin.GET(route, api.getSchedule)
			admin.DELETE(route, api.deleteSchedule)
		}
		for _, route := range routes.ScheduleTriggers {
			admin.POST(route, api.triggerSchedule)
		}
	}
	for _, route := range routes.ScriptItems {
		admin.GET(route, api.getScript)
		admin.DELETE(route, api.deleteScript)
	}
	for _, route := range routes.Instances {
		admin.GET(route, api.listInstances)
	}
	for _, route := range routes.InstanceItems {
		admin.GET(route, api.getInstance)
		admin.PUT(route, api.saveInstance)
		admin.DELETE(route, api.deleteInstance)
	}
	invoke := group.Group("", config.InvocationMiddleware...)
	for _, route := range routes.Invoke {
		invoke.POST(route, api.invokeJourney)
	}
	for _, route := range routes.JourneyItems {
		admin.GET(route, api.getJourney)
		admin.DELETE(route, api.deleteJourney)
	}
	if err := api.registerResourceJourneyFallback(); err != nil {
		return err
	}
	return nil
}

type restNoRouteRegistrar interface {
	NoRoute(...gin.HandlerFunc)
}

func (api *restAPI) registerResourceJourneyFallback() error {
	router, ok := api.config.Router.(restNoRouteRegistrar)
	if !ok {
		return errors.New("resource journey fallback requires a Gin engine with NoRoute support")
	}
	handlers := append([]gin.HandlerFunc{}, api.config.Middleware...)
	handlers = append(handlers, api.accessEventMiddleware())
	handlers = append(handlers, api.config.InvocationMiddleware...)
	handlers = append(handlers, api.invokeRouteJourney)
	router.NoRoute(handlers...)
	return nil
}

func (api *restAPI) readJSON(context *gin.Context, target any) ([]byte, bool) {
	context.Request.Body = http.MaxBytesReader(context.Writer, context.Request.Body, api.maxBodyBytes)
	data, err := io.ReadAll(context.Request.Body)
	if err != nil {
		status := http.StatusBadRequest
		code := "invalid_body"
		if _, tooLarge := err.(*http.MaxBytesError); tooLarge {
			status, code = http.StatusRequestEntityTooLarge, "body_too_large"
		}
		writeRESTError(context, status, code, err)
		return nil, false
	}
	if len(data) == 0 || json.Unmarshal(data, target) != nil {
		writeRESTError(context, http.StatusBadRequest, "invalid_json", errors.New("valid JSON body is required"))
		return nil, false
	}
	return data, true
}

func (api *restAPI) readRawBody(context *gin.Context) ([]byte, bool) {
	context.Request.Body = http.MaxBytesReader(context.Writer, context.Request.Body, api.maxBodyBytes)
	data, err := io.ReadAll(context.Request.Body)
	if err != nil {
		status := http.StatusBadRequest
		code := "invalid_body"
		if _, tooLarge := err.(*http.MaxBytesError); tooLarge {
			status, code = http.StatusRequestEntityTooLarge, "body_too_large"
		}
		writeRESTError(context, status, code, err)
		return nil, false
	}
	return data, true
}

func restAPIRoutes(overrides RESTAPIRoutes) (RESTAPIRoutes, error) {
	routes := RESTAPIRoutes{
		Journeys:            restRouteTemplates(overrides.Journeys, []string{"/:realm"}, "journey_routes", "journeys"),
		JourneyItems:        restRouteTemplates(overrides.JourneyItems, []string{"/:realm/:journeyId"}, "journey_item_routes", "journey_items"),
		Scripts:             restRouteTemplates(overrides.Scripts, []string{"/:realm/scripts"}, "script_routes", "scripts"),
		ScriptItems:         restRouteTemplates(overrides.ScriptItems, []string{"/:realm/scripts/:scriptId"}, "script_item_routes", "script_items"),
		ScriptBindings:      restRouteTemplates(overrides.ScriptBindings, []string{"/:realm/scripts/:scriptId/bindings"}, "script_binding_routes", "script_bindings"),
		ScriptTypeBindings:  restRouteTemplates(overrides.ScriptTypeBindings, []string{"/:realm/script-bindings"}, "script_type_binding_routes", "script_type_bindings"),
		Schemas:             restRouteTemplates(overrides.Schemas, []string{"/:realm/schemas"}, "schema_routes", "schemas"),
		SchemaItems:         restRouteTemplates(overrides.SchemaItems, []string{"/:realm/schemas/:schemaId"}, "schema_item_routes", "schema_items"),
		Schedules:           restRouteTemplates(overrides.Schedules, []string{"/:realm/schedules"}, "schedule_routes", "schedules"),
		ScheduleItems:       restRouteTemplates(overrides.ScheduleItems, []string{"/:realm/schedules/:scheduleId"}, "schedule_item_routes", "schedule_items"),
		ScheduleTriggers:    restRouteTemplates(overrides.ScheduleTriggers, []string{"/:realm/schedules/:scheduleId/trigger"}, "schedule_trigger_routes", "schedule_triggers"),
		Instances:           restRouteTemplates(overrides.Instances, []string{"/:realm/instances"}, "instance_routes", "instances"),
		InstanceItems:       restRouteTemplates(overrides.InstanceItems, []string{"/:realm/instances/:cacheKey/:instanceId"}, "instance_item_routes", "instance_items"),
		InstanceSchemas:     restRouteTemplates(overrides.InstanceSchemas, []string{"/:realm/instance-schemas"}, "instance_schema_routes", "instance_schemas"),
		InstanceSchemaItems: restRouteTemplates(overrides.InstanceSchemaItems, []string{"/:realm/instance-schemas/:cacheKey"}, "instance_schema_item_routes", "instance_schema_items"),
		StepSchemas:         restRouteTemplates(overrides.StepSchemas, []string{"/step-schemas"}, "step_schema_routes", "step_schemas"),
		StepSchemaItems:     restRouteTemplates(overrides.StepSchemaItems, []string{"/step-schemas/:stepType"}, "step_schema_item_routes", "step_schema_items"),
		Invoke:              restRouteTemplates(overrides.Invoke, []string{"/:realm/invoke"}, "invoke_routes", "invoke"),
	}
	if err := validateRESTRouteTemplates("journey_routes", routes.Journeys); err != nil {
		return RESTAPIRoutes{}, err
	}
	if err := validateRESTRouteTemplates("journey_item_routes", routes.JourneyItems, ":journeyId"); err != nil {
		return RESTAPIRoutes{}, err
	}
	if err := validateRESTRouteTemplates("script_routes", routes.Scripts); err != nil {
		return RESTAPIRoutes{}, err
	}
	if err := validateRESTRouteTemplates("script_item_routes", routes.ScriptItems, ":scriptId"); err != nil {
		return RESTAPIRoutes{}, err
	}
	if err := validateRESTRouteTemplates("script_binding_routes", routes.ScriptBindings, ":scriptId"); err != nil {
		return RESTAPIRoutes{}, err
	}
	if err := validateRESTRouteTemplates("script_type_binding_routes", routes.ScriptTypeBindings); err != nil {
		return RESTAPIRoutes{}, err
	}
	if err := validateRESTRouteTemplates("schema_routes", routes.Schemas); err != nil {
		return RESTAPIRoutes{}, err
	}
	if err := validateRESTRouteTemplates("schema_item_routes", routes.SchemaItems, ":schemaId"); err != nil {
		return RESTAPIRoutes{}, err
	}
	if err := validateRESTRouteTemplates("schedule_routes", routes.Schedules); err != nil {
		return RESTAPIRoutes{}, err
	}
	if err := validateRESTRouteTemplates("schedule_item_routes", routes.ScheduleItems, ":scheduleId"); err != nil {
		return RESTAPIRoutes{}, err
	}
	if err := validateRESTRouteTemplates("schedule_trigger_routes", routes.ScheduleTriggers, ":scheduleId"); err != nil {
		return RESTAPIRoutes{}, err
	}
	if err := validateRESTRouteTemplates("instance_routes", routes.Instances); err != nil {
		return RESTAPIRoutes{}, err
	}
	if err := validateRESTRouteTemplates("instance_item_routes", routes.InstanceItems, ":cacheKey", ":instanceId"); err != nil {
		return RESTAPIRoutes{}, err
	}
	if err := validateRESTRouteTemplates("instance_schema_routes", routes.InstanceSchemas); err != nil {
		return RESTAPIRoutes{}, err
	}
	if err := validateRESTRouteTemplates("instance_schema_item_routes", routes.InstanceSchemaItems, ":cacheKey"); err != nil {
		return RESTAPIRoutes{}, err
	}
	if err := validateRESTRouteTemplates("step_schema_routes", routes.StepSchemas); err != nil {
		return RESTAPIRoutes{}, err
	}
	if err := validateRESTRouteTemplates("step_schema_item_routes", routes.StepSchemaItems, ":stepType"); err != nil {
		return RESTAPIRoutes{}, err
	}
	if err := validateRESTRouteTemplates("invoke_routes", routes.Invoke); err != nil {
		return RESTAPIRoutes{}, err
	}
	return routes, nil
}

func restRouteTemplates(overrides []string, defaults []string, keys ...string) []string {
	if cleaned := cleanRouteTemplates(overrides); len(cleaned) > 0 {
		return cleaned
	}
	for _, key := range keys {
		if configured, ok := restConfigStringSlice("rest_api.routes." + key); ok {
			if cleaned := cleanRouteTemplates(configured); len(cleaned) > 0 {
				return cleaned
			}
		}
	}
	return cleanRouteTemplates(defaults)
}

func restWildcardRouteTemplates(overrides []string, defaults []string, keys ...string) []string {
	if cleaned := cleanRouteTemplatesWithWildcard(overrides); len(cleaned) > 0 {
		return cleaned
	}
	for _, key := range keys {
		if configured, ok := restConfigStringSlice("rest_api.routes." + key); ok {
			if cleaned := cleanRouteTemplatesWithWildcard(configured); len(cleaned) > 0 {
				return cleaned
			}
		}
	}
	return cleanRouteTemplatesWithWildcard(defaults)
}

func restConfigStringSlice(key string) (values []string, ok bool) {
	defer func() {
		if recover() != nil {
			values = nil
			ok = false
		}
	}()
	values, err := env.GetJourneyField[[]string](key)
	if err != nil {
		return nil, false
	}
	return values, true
}

func restConfigString(key string, defaultValue string) (value string) {
	defer func() {
		if recover() != nil {
			value = defaultValue
		}
	}()
	return env.GetOptionalJourneyField(key, defaultValue)
}

func restConfigBool(key string, defaultValue bool) (value bool) {
	defer func() {
		if recover() != nil {
			value = defaultValue
		}
	}()
	return env.GetOptionalJourneyField(key, defaultValue)
}

func restUIEnabled(config *RESTAPIConfig) bool {
	if config != nil && config.UIEnabled {
		return true
	}
	return restConfigBool("rest_api.ui.enabled", false)
}

func restUIPath(config *RESTAPIConfig) string {
	if config != nil && strings.TrimSpace(config.UIPath) != "" {
		return config.UIPath
	}
	return restConfigString("rest_api.ui.path", "/journey-ui")
}

func cleanRouteTemplates(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.Trim(strings.TrimSpace(value), "/")
		if value == "" || strings.Contains(value, "*") {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func cleanRouteTemplatesWithWildcard(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.Trim(strings.TrimSpace(value), "/")
		if value == "" {
			continue
		}
		segments := strings.Split(value, "/")
		valid := true
		for i, segment := range segments {
			if strings.Contains(segment, "*") {
				if !strings.HasPrefix(segment, "*") || segment == "*" || i != len(segments)-1 {
					valid = false
					break
				}
			}
		}
		if !valid {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func validateRESTRouteTemplates(name string, routes []string, requiredParams ...string) error {
	if len(routes) == 0 {
		return fmt.Errorf("REST API %s must contain at least one route", name)
	}
	for _, route := range routes {
		segments := routeParamSet(route)
		for _, required := range requiredParams {
			if _, ok := segments[required]; !ok {
				return fmt.Errorf("REST API %s route %q must include %s", name, route, required)
			}
		}
	}
	return nil
}

func routeParamSet(route string) map[string]struct{} {
	result := map[string]struct{}{}
	for _, segment := range strings.Split(route, "/") {
		if strings.HasPrefix(segment, ":") || strings.HasPrefix(segment, "*") {
			result[segment] = struct{}{}
		}
	}
	return result
}

func (api *restAPI) listJourneys(context *gin.Context) {
	items := api.journeys.ListOfCache()
	nameFilter := strings.ToLower(strings.TrimSpace(context.Query("name")))
	rawTypeFilter := strings.TrimSpace(context.Query("type"))
	if rawTypeFilter == "" {
		rawTypeFilter = strings.TrimSpace(context.Query("journey_type"))
	}
	typeFilter := ""
	if rawTypeFilter != "" {
		typeFilter = types.NormalizeJourneyType(rawTypeFilter)
	}
	realmFilter := restRequestRealm(context)
	if realmFilter == "" {
		realmFilter = strings.TrimSpace(context.Query("realm"))
	}
	limit, ok := restQueryLimit(context)
	if !ok {
		return
	}
	filtered := make([]*types.JourneyConfiguration, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		if nameFilter != "" && !strings.Contains(strings.ToLower(item.Name), nameFilter) {
			continue
		}
		if realmFilter != "" && item.Realm != realmFilter {
			continue
		}
		if typeFilter != "" && types.NormalizeJourneyType(item.JourneyType) != typeFilter {
			continue
		}
		filtered = append(filtered, item)
		if limit > 0 && len(filtered) >= limit {
			break
		}
	}
	items = filtered
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	result := make([]any, 0, len(items))
	for _, item := range items {
		result = append(result, api.restJourneyResponse(item))
	}
	context.JSON(http.StatusOK, restQueryResponse[any]{Result: result, ResultCount: len(result)})
}

func (api *restAPI) saveJourney(context *gin.Context) {
	var journey types.JourneyConfiguration
	if _, ok := api.readJSON(context, &journey); !ok {
		return
	}
	if realm := restRequestRealm(context); realm != "" {
		journey.Realm = realm
	}
	created := true
	existing, duplicate := api.resolveJourneySaveTarget(&journey)
	if duplicate != nil {
		writeRESTError(context, http.StatusConflict, "duplicate_journey", fmt.Errorf("journey name %q already exists in realm %q", journey.Name, journey.Realm))
		return
	}
	if existing != nil {
		created = false
		journey.Metadata = existing.Metadata
	} else if journey.Metadata != nil && journey.Metadata.ID == "" {
		journey.Metadata = nil
	}
	if err := api.journeys.Save(&journey); err != nil {
		writeRESTError(context, http.StatusBadRequest, "invalid_journey", err)
		return
	}
	api.routeCache.Clear()
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	context.JSON(status, api.restJourneyResponse(&journey))
}

func (api *restAPI) getJourney(context *gin.Context) {
	journey, err := api.journeys.Load(context.Param("journeyId"))
	if err != nil || journey == nil || !restRealmMatches(context, journey.Realm) {
		api.invokeRouteJourney(context)
		return
	}
	context.JSON(http.StatusOK, api.restJourneyResponse(journey))
}

func (api *restAPI) deleteJourney(context *gin.Context) {
	id := context.Param("journeyId")
	journey, err := api.journeys.Load(id)
	if err != nil || journey == nil || !restRealmMatches(context, journey.Realm) {
		api.invokeRouteJourney(context)
		return
	}
	if err := api.journeys.Delete(id); err != nil {
		writeRESTError(context, http.StatusInternalServerError, "journey_delete_failed", err)
		return
	}
	api.routeCache.Clear()
	context.Status(http.StatusNoContent)
}

func (api *restAPI) restJourneyResponse(journey *types.JourneyConfiguration) any {
	if journey == nil || api.returnVars() {
		return journey
	}
	data, err := json.Marshal(journey)
	if err != nil {
		return journey
	}
	var response map[string]any
	if err := json.Unmarshal(data, &response); err != nil {
		return journey
	}
	stripRESTJourneyVars(response)
	return response
}

func (api *restAPI) returnVars() bool {
	if api.config != nil && api.config.ReturnVars {
		return true
	}
	return restConfigBool("rest_api.return_vars", false)
}

func stripRESTJourneyVars(journey map[string]any) {
	steps, _ := journey["steps"].(map[string]any)
	for _, rawStep := range steps {
		step, _ := rawStep.(map[string]any)
		config, _ := step["config"].(map[string]any)
		delete(config, "vars")
	}
}

func restRealmMatches(context *gin.Context, resourceRealm string) bool {
	realm := restRequestRealm(context)
	return realm == "" || resourceRealm == realm
}

func restRequestRealm(context *gin.Context) string {
	if realm := restContextRealm(context); realm != "" {
		return realm
	}
	return strings.TrimSpace(context.Param("realm"))
}

func restContextRealm(context *gin.Context) string {
	value, found := context.Get("realm")
	if !found || value == nil {
		return ""
	}
	switch realm := value.(type) {
	case string:
		return strings.TrimSpace(realm)
	case *types.Realm:
		if realm == nil {
			return ""
		}
		return strings.TrimSpace(realm.Name)
	case types.Realm:
		return strings.TrimSpace(realm.Name)
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func (api *restAPI) listScripts(context *gin.Context) {
	items := api.scripts.ListOfCache()
	nameFilter := strings.ToLower(strings.TrimSpace(context.Query("name")))
	typeFilter := steps.NormalizeScriptType(context.Query("type"))
	limit, ok := restQueryLimit(context)
	if !ok {
		return
	}
	filtered := make([]*jsrun.Script, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		if nameFilter != "" && !strings.Contains(strings.ToLower(item.Name), nameFilter) {
			continue
		}
		if typeFilter != "" && steps.NormalizeScriptType(item.Type) != typeFilter {
			continue
		}
		filtered = append(filtered, item)
		if limit > 0 && len(filtered) >= limit {
			break
		}
	}
	items = filtered
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	context.JSON(http.StatusOK, restQueryResponse[*jsrun.Script]{Result: items, ResultCount: len(items)})
}

func (api *restAPI) saveScript(context *gin.Context) {
	var script jsrun.Script
	if _, ok := api.readJSON(context, &script); !ok {
		return
	}
	script.Type = steps.NormalizeScriptType(script.Type)
	if err := validateRESTScript(&script); err != nil {
		writeRESTError(context, http.StatusBadRequest, "invalid_script", err)
		return
	}
	existing := api.findExistingScript(&script)
	if duplicate := api.findScriptByNameType(script.Name, script.Type, restMetadataID(script.Metadata)); duplicate != nil {
		writeRESTError(context, http.StatusConflict, "duplicate_script", fmt.Errorf("script name %q already exists for type %q", script.Name, script.Type))
		return
	}
	created := existing == nil
	if existing != nil {
		script.Metadata = existing.Metadata
	} else if script.Metadata != nil && script.Metadata.ID == "" {
		script.Metadata = nil
	}
	if api.scriptManager != nil && existing != nil && existing.Name != script.Name {
		api.scriptManager.DeleteFromCache(existing.Name)
	}
	if err := api.scripts.Save(&script); err != nil {
		writeRESTError(context, http.StatusBadRequest, "invalid_script", err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	context.JSON(status, &script)
}

func (api *restAPI) getScript(context *gin.Context) {
	script, err := api.scripts.Load(context.Param("scriptId"))
	if err != nil {
		writeRESTError(context, http.StatusNotFound, "script_not_found", err)
		return
	}
	context.JSON(http.StatusOK, script)
}

func (api *restAPI) deleteScript(context *gin.Context) {
	id := context.Param("scriptId")
	script, err := api.scripts.Load(id)
	if err != nil {
		writeRESTError(context, http.StatusNotFound, "script_not_found", err)
		return
	}
	if err := api.scripts.Delete(id); err != nil && !errors.Is(err, os.ErrNotExist) {
		writeRESTError(context, http.StatusInternalServerError, "script_delete_failed", err)
		return
	}
	if api.scriptManager != nil {
		api.scriptManager.DeleteFromCache(script.Name)
	}
	context.Status(http.StatusNoContent)
}

func (api *restAPI) listScriptTypeBindings(context *gin.Context) {
	scriptType := steps.NormalizeScriptType(context.Query("type"))
	if scriptType == "" {
		sets, err := steps.ScriptBindingSets(api.manager.cacheManager, restRequestRealm(context))
		if err != nil {
			writeRESTError(context, http.StatusInternalServerError, "script_bindings_failed", err)
			return
		}
		context.JSON(http.StatusOK, restQueryResponse[steps.ScriptBindingSetDescriptor]{Result: sets, ResultCount: len(sets)})
		return
	}
	descriptors, err := steps.ScriptBindingDescriptors(api.manager.cacheManager, restRequestRealm(context), scriptType, nil)
	if err != nil {
		writeRESTError(context, http.StatusInternalServerError, "script_bindings_failed", err)
		return
	}
	definition := steps.ScriptTypeDefinition{Type: scriptType}
	for _, candidate := range steps.ScriptTypeDefinitions() {
		if candidate.Type == scriptType {
			definition = candidate
			break
		}
	}
	sets := []steps.ScriptBindingSetDescriptor{{
		Type:        scriptType,
		Name:        definition.Name,
		Description: definition.Description,
		Runnable:    definition.Runnable,
		Bindings:    descriptors,
	}}
	context.JSON(http.StatusOK, restQueryResponse[steps.ScriptBindingSetDescriptor]{Result: sets, ResultCount: len(sets)})
}

func (api *restAPI) getScriptBindings(context *gin.Context) {
	script, err := api.scripts.Load(context.Param("scriptId"))
	if err != nil {
		writeRESTError(context, http.StatusNotFound, "script_not_found", err)
		return
	}
	descriptors, err := steps.ScriptBindingDescriptors(api.manager.cacheManager, restRequestRealm(context), script.Type, script)
	if err != nil {
		writeRESTError(context, http.StatusInternalServerError, "script_bindings_failed", err)
		return
	}
	context.JSON(http.StatusOK, restQueryResponse[steps.ScriptBindingDescriptor]{Result: descriptors, ResultCount: len(descriptors)})
}

func (api *restAPI) listSchemas(context *gin.Context) {
	nameFilter := strings.ToLower(strings.TrimSpace(context.Query("name")))
	limit, ok := restQueryLimit(context)
	if !ok {
		return
	}
	if limit == 0 {
		limit = 200
	}
	items := api.schemas.ListOfCache()
	filtered := make([]*types.DeveloperSchema, 0, len(items))
	realm := restRequestRealm(context)
	for _, item := range items {
		if item == nil || item.Realm != realm {
			continue
		}
		if nameFilter != "" && !strings.Contains(strings.ToLower(item.Name+" "+item.Description+" "+metadataID(item.Metadata)), nameFilter) {
			continue
		}
		filtered = append(filtered, item)
		if limit > 0 && len(filtered) >= limit {
			break
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Name < filtered[j].Name })
	context.JSON(http.StatusOK, restQueryResponse[*types.DeveloperSchema]{Result: filtered, ResultCount: len(filtered)})
}

func (api *restAPI) saveSchema(context *gin.Context) {
	var schema types.DeveloperSchema
	if _, ok := api.readJSON(context, &schema); !ok {
		return
	}
	realm := restRequestRealm(context)
	schema.Realm = realm
	if strings.TrimSpace(schema.Name) == "" {
		writeRESTError(context, http.StatusBadRequest, "invalid_schema", errors.New("schema name is required"))
		return
	}
	if len(schema.Schema) == 0 {
		writeRESTError(context, http.StatusBadRequest, "invalid_schema", errors.New("schema body is required"))
		return
	}
	created := schema.Metadata == nil || strings.TrimSpace(schema.Metadata.ID) == ""
	if err := validateSchemaUniqueness(api.schemas, &schema, realm); err != nil {
		writeRESTError(context, http.StatusConflict, "duplicate_schema", err)
		return
	}
	if err := api.schemas.Save(&schema); err != nil {
		writeRESTError(context, http.StatusBadRequest, "schema_save_failed", err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	context.JSON(status, schema)
}

func (api *restAPI) getSchema(context *gin.Context) {
	schema, err := api.schemas.Load(context.Param("schemaId"))
	if err != nil || schema.Realm != restRequestRealm(context) {
		writeRESTError(context, http.StatusNotFound, "schema_not_found", os.ErrNotExist)
		return
	}
	context.JSON(http.StatusOK, schema)
}

func (api *restAPI) deleteSchema(context *gin.Context) {
	schema, err := api.schemas.Load(context.Param("schemaId"))
	if err != nil || schema.Realm != restRequestRealm(context) {
		writeRESTError(context, http.StatusNotFound, "schema_not_found", os.ErrNotExist)
		return
	}
	if err := api.schemas.Delete(context.Param("schemaId")); err != nil && !errors.Is(err, os.ErrNotExist) {
		writeRESTError(context, http.StatusInternalServerError, "schema_delete_failed", err)
		return
	}
	context.Status(http.StatusNoContent)
}

func validateSchemaUniqueness(storage RESTSchemaStorage, schema *types.DeveloperSchema, realm string) error {
	for _, existing := range storage.ListOfCache() {
		if existing == nil || existing.Realm != realm || existing.Name != schema.Name {
			continue
		}
		if metadataID(existing.Metadata) != "" && metadataID(existing.Metadata) != metadataID(schema.Metadata) {
			return fmt.Errorf("schema %q already exists in realm %q", schema.Name, realm)
		}
	}
	return nil
}

func (api *restAPI) listSchedules(context *gin.Context) {
	nameFilter := strings.ToLower(strings.TrimSpace(context.Query("name")))
	targetFilter := strings.TrimSpace(context.Query("target_type"))
	limit, ok := restQueryLimit(context)
	if !ok {
		return
	}
	items := api.schedules.ListOfCache()
	realm := restRequestRealm(context)
	filtered := make([]*types.ScheduleConfiguration, 0, len(items))
	for _, item := range items {
		if item == nil || item.Realm != realm {
			continue
		}
		if nameFilter != "" && !strings.Contains(strings.ToLower(item.Name+" "+item.Description+" "+metadataID(item.Metadata)), nameFilter) {
			continue
		}
		if targetFilter != "" && item.Target.Type != targetFilter {
			continue
		}
		filtered = append(filtered, item)
		if limit > 0 && len(filtered) >= limit {
			break
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Name < filtered[j].Name })
	context.JSON(http.StatusOK, restQueryResponse[*types.ScheduleConfiguration]{Result: filtered, ResultCount: len(filtered)})
}

func (api *restAPI) saveSchedule(context *gin.Context) {
	var schedule types.ScheduleConfiguration
	if _, ok := api.readJSON(context, &schedule); !ok {
		return
	}
	realm := restRequestRealm(context)
	schedule.Realm = realm
	if err := validateScheduleUniqueness(api.schedules, &schedule, realm); err != nil {
		writeRESTError(context, http.StatusConflict, "duplicate_schedule", err)
		return
	}
	created := schedule.Metadata == nil || strings.TrimSpace(schedule.ID) == ""
	if err := api.schedules.Save(&schedule); err != nil {
		writeRESTError(context, http.StatusBadRequest, "schedule_save_failed", err)
		return
	}
	if api.manager.scheduler != nil {
		if err := api.manager.scheduler.Reload(&schedule); err != nil {
			writeRESTError(context, http.StatusBadRequest, "schedule_reload_failed", err)
			return
		}
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	context.JSON(status, schedule)
}

func (api *restAPI) getSchedule(context *gin.Context) {
	schedule, err := api.schedules.Load(context.Param("scheduleId"))
	if err != nil || schedule == nil || schedule.Realm != restRequestRealm(context) {
		writeRESTError(context, http.StatusNotFound, "schedule_not_found", os.ErrNotExist)
		return
	}
	context.JSON(http.StatusOK, schedule)
}

func (api *restAPI) deleteSchedule(context *gin.Context) {
	scheduleID := context.Param("scheduleId")
	schedule, err := api.schedules.Load(scheduleID)
	if err != nil || schedule == nil || schedule.Realm != restRequestRealm(context) {
		writeRESTError(context, http.StatusNotFound, "schedule_not_found", os.ErrNotExist)
		return
	}
	if api.manager.scheduler != nil {
		api.manager.scheduler.Unschedule(scheduleID)
	}
	if err := api.schedules.Delete(scheduleID); err != nil && !errors.Is(err, os.ErrNotExist) {
		writeRESTError(context, http.StatusInternalServerError, "schedule_delete_failed", err)
		return
	}
	context.Status(http.StatusNoContent)
}

func (api *restAPI) triggerSchedule(context *gin.Context) {
	var request types.ScheduleTriggerRequest
	if context.Request.Body != nil && context.Request.ContentLength != 0 {
		if _, ok := api.readJSON(context, &request); !ok {
			return
		}
	}
	if api.manager.scheduler == nil {
		writeRESTError(context, http.StatusServiceUnavailable, "scheduler_not_available", errors.New("scheduler is not available"))
		return
	}
	schedule, err := api.schedules.Load(context.Param("scheduleId"))
	if err != nil || schedule == nil || schedule.Realm != restRequestRealm(context) {
		writeRESTError(context, http.StatusNotFound, "schedule_not_found", os.ErrNotExist)
		return
	}
	result, err := api.manager.scheduler.Trigger(context.Request.Context(), context.Param("scheduleId"), &request)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, ErrJourneyNotFound) {
			status = http.StatusNotFound
		}
		writeRESTError(context, status, "schedule_trigger_failed", err)
		return
	}
	context.JSON(http.StatusOK, result)
}

func validateScheduleUniqueness(storage RESTScheduleStorage, schedule *types.ScheduleConfiguration, realm string) error {
	for _, existing := range storage.ListOfCache() {
		if existing == nil || existing.Realm != realm || existing.Name != schedule.Name {
			continue
		}
		if metadataID(existing.Metadata) != "" && metadataID(existing.Metadata) != metadataID(schedule.Metadata) {
			return fmt.Errorf("schedule %q already exists in realm %q", schedule.Name, realm)
		}
	}
	return nil
}

func metadataID(metadata interface{ GetMetadata() *nstore.Metadata }) string {
	if metadata == nil || metadata.GetMetadata() == nil {
		return ""
	}
	return metadata.GetMetadata().ID
}

type restStepSchema struct {
	StepType string          `json:"step_type"`
	Schema   json.RawMessage `json:"schema"`
}

type restInstancesResponse struct {
	Result      []jcache.CacheInstanceInfo `json:"result"`
	ResultCount int                        `json:"resultCount"`
	Caches      []jcache.CacheInfo         `json:"caches"`
}

func (api *restAPI) listInstances(context *gin.Context) {
	cacheKeyFilter := strings.TrimSpace(context.Query("cache"))
	if cacheKeyFilter == "" {
		cacheKeyFilter = strings.TrimSpace(context.Query("key"))
	}
	instanceFilter := strings.ToLower(strings.TrimSpace(context.Query("instance_id")))
	if instanceFilter == "" {
		instanceFilter = strings.ToLower(strings.TrimSpace(context.Query("name")))
	}
	limit, ok := restQueryLimit(context)
	if !ok {
		return
	}
	caches := api.manager.cacheManager.ListCaches()
	instances := api.manager.cacheManager.ListCacheInstances()
	configurableCaches := map[string]jcache.CacheInfo{}
	for _, item := range caches {
		if !item.UserConfigurable {
			continue
		}
		configurableCaches[item.Key] = item
	}

	cacheStats := map[string]struct {
		instances int
		sizeBytes int64
	}{}
	for _, item := range instances {
		if !item.Persisted {
			continue
		}
		if _, ok := configurableCaches[item.CacheKey]; !ok {
			continue
		}
		stats := cacheStats[item.CacheKey]
		stats.instances++
		stats.sizeBytes += item.SizeBytes
		cacheStats[item.CacheKey] = stats
	}

	filteredCaches := make([]jcache.CacheInfo, 0, len(configurableCaches))
	for _, item := range configurableCaches {
		if cacheKeyFilter != "" && item.Key != cacheKeyFilter {
			continue
		}
		stats := cacheStats[item.Key]
		item.Instances = stats.instances
		item.SizeBytes = stats.sizeBytes
		filteredCaches = append(filteredCaches, item)
	}
	filteredInstances := make([]jcache.CacheInstanceInfo, 0, len(instances))
	for _, item := range instances {
		if !item.Persisted {
			continue
		}
		if _, ok := configurableCaches[item.CacheKey]; !ok {
			continue
		}
		if cacheKeyFilter != "" && item.CacheKey != cacheKeyFilter {
			continue
		}
		if instanceFilter != "" && !strings.Contains(strings.ToLower(item.InstanceID), instanceFilter) {
			continue
		}
		filteredInstances = append(filteredInstances, item)
		if limit > 0 && len(filteredInstances) >= limit {
			break
		}
	}
	sort.Slice(filteredCaches, func(i, j int) bool { return filteredCaches[i].Key < filteredCaches[j].Key })
	sort.Slice(filteredInstances, func(i, j int) bool {
		if filteredInstances[i].CacheKey == filteredInstances[j].CacheKey {
			return filteredInstances[i].InstanceID < filteredInstances[j].InstanceID
		}
		return filteredInstances[i].CacheKey < filteredInstances[j].CacheKey
	})
	context.JSON(http.StatusOK, restInstancesResponse{Result: filteredInstances, ResultCount: len(filteredInstances), Caches: filteredCaches})
}

func (api *restAPI) getInstance(context *gin.Context) {
	cacheKey := context.Param("cacheKey")
	item, found := api.manager.cacheManager.CacheInstanceInfo(cacheKey, context.Param("instanceId"))
	cacheInfo, cacheFound := api.manager.cacheManager.CacheInfo(cacheKey)
	if !found || !item.Persisted || !cacheFound || !cacheInfo.UserConfigurable {
		writeRESTError(context, http.StatusNotFound, "instance_not_found", jcache.ErrInstanceNotFound)
		return
	}
	context.JSON(http.StatusOK, item)
}

func (api *restAPI) saveInstance(context *gin.Context) {
	cacheKey := strings.TrimSpace(context.Param("cacheKey"))
	instanceID := strings.TrimSpace(context.Param("instanceId"))
	if cacheKey == "" || instanceID == "" {
		writeRESTError(context, http.StatusBadRequest, "invalid_instance", errors.New("cache key and instance id are required"))
		return
	}
	cacheInfo, cacheFound := api.manager.cacheManager.CacheInfo(cacheKey)
	if !cacheFound || !cacheInfo.UserConfigurable {
		writeRESTError(context, http.StatusNotFound, "instance_cache_not_found", jcache.ErrCacheNotFound)
		return
	}
	_, existed := api.manager.cacheManager.CacheInstanceInfo(cacheKey, instanceID)
	var config json.RawMessage
	if _, ok := api.readJSON(context, &config); !ok {
		return
	}
	if err := api.manager.cacheManager.UpdateCacheInstance(cacheKey, instanceID, config); err != nil {
		writeCacheRESTError(context, err)
		return
	}
	item, _ := api.manager.cacheManager.CacheInstanceInfo(cacheKey, instanceID)
	status := http.StatusOK
	if !existed {
		status = http.StatusCreated
	}
	context.JSON(status, item)
}

func (api *restAPI) deleteInstance(context *gin.Context) {
	cacheKey := context.Param("cacheKey")
	item, found := api.manager.cacheManager.CacheInstanceInfo(cacheKey, context.Param("instanceId"))
	cacheInfo, cacheFound := api.manager.cacheManager.CacheInfo(cacheKey)
	if !found || !item.Persisted || !cacheFound || !cacheInfo.UserConfigurable {
		writeRESTError(context, http.StatusNotFound, "instance_not_found", jcache.ErrInstanceNotFound)
		return
	}
	if err := api.manager.cacheManager.RemoveCacheInstance(cacheKey, context.Param("instanceId")); err != nil {
		writeCacheRESTError(context, err)
		return
	}
	context.Status(http.StatusNoContent)
}

func (api *restAPI) listInstanceSchemas(context *gin.Context) {
	cacheKeyFilter := strings.TrimSpace(context.Query("cache_key"))
	if cacheKeyFilter == "" {
		cacheKeyFilter = strings.TrimSpace(context.Query("key"))
	}
	typeFilter := strings.ToLower(strings.TrimSpace(context.Query("type")))
	if typeFilter == "" {
		typeFilter = strings.ToLower(strings.TrimSpace(context.Query("name")))
	}
	limit, ok := restQueryLimit(context)
	if !ok {
		return
	}
	caches := api.manager.cacheManager.ListCaches()
	result := make([]jcache.CacheInfo, 0, len(caches))
	for _, item := range caches {
		if !item.UserConfigurable || len(item.Schema) == 0 {
			continue
		}
		if cacheKeyFilter != "" && item.Key != cacheKeyFilter {
			continue
		}
		if typeFilter != "" &&
			!strings.Contains(strings.ToLower(item.Key), typeFilter) &&
			!strings.Contains(strings.ToLower(item.Description), typeFilter) {
			continue
		}
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	context.JSON(http.StatusOK, restQueryResponse[jcache.CacheInfo]{Result: result, ResultCount: len(result)})
}

func (api *restAPI) getInstanceSchema(context *gin.Context) {
	cacheKey := strings.TrimSpace(context.Param("cacheKey"))
	item, found := api.manager.cacheManager.CacheInfo(cacheKey)
	if !found || !item.UserConfigurable || len(item.Schema) == 0 {
		writeRESTError(context, http.StatusNotFound, "instance_schema_not_found", errors.New("instance schema not found"))
		return
	}
	context.JSON(http.StatusOK, item)
}

func writeCacheRESTError(context *gin.Context, err error) {
	switch {
	case errors.Is(err, jcache.ErrCacheNotFound), errors.Is(err, jcache.ErrInstanceNotFound):
		writeRESTError(context, http.StatusNotFound, "instance_not_found", err)
	case errors.Is(err, jcache.ErrFactoryNotFound):
		writeRESTError(context, http.StatusBadRequest, "cache_not_persistable", err)
	case errors.Is(err, jcache.ErrMaxInstances), errors.Is(err, jcache.ErrMaxSize):
		writeRESTError(context, http.StatusConflict, "cache_limit_exceeded", err)
	default:
		writeRESTError(context, http.StatusBadRequest, "invalid_instance", err)
	}
}

func (api *restAPI) listStepSchemas(context *gin.Context) {
	stepTypeFilter := strings.TrimSpace(context.Query("type"))
	if stepTypeFilter == "" {
		stepTypeFilter = strings.TrimSpace(context.Query("name"))
	}
	limit, ok := restQueryLimit(context)
	if !ok {
		return
	}
	schemas := api.manager.steps.GetStepSchemasJSON()
	stepTypes := make([]string, 0, len(schemas))
	for stepType := range schemas {
		if stepTypeFilter != "" && !strings.Contains(strings.ToLower(stepType), strings.ToLower(stepTypeFilter)) {
			continue
		}
		stepTypes = append(stepTypes, stepType)
	}
	sort.Strings(stepTypes)
	if limit > 0 && len(stepTypes) > limit {
		stepTypes = stepTypes[:limit]
	}
	result := make([]restStepSchema, 0, len(stepTypes))
	for _, stepType := range stepTypes {
		result = append(result, restStepSchema{StepType: stepType, Schema: schemas[stepType]})
	}
	context.JSON(http.StatusOK, restQueryResponse[restStepSchema]{Result: result, ResultCount: len(result)})
}

func (api *restAPI) getStepSchema(context *gin.Context) {
	stepType := strings.TrimSpace(context.Param("stepType"))
	schema, ok := api.manager.steps.GetStepSchemasJSON()[stepType]
	if !ok {
		writeRESTError(context, http.StatusNotFound, "step_schema_not_found", errors.New("step schema not found"))
		return
	}
	context.Data(http.StatusOK, "application/json; charset=utf-8", schema)
}

func (api *restAPI) accessEventMiddleware() gin.HandlerFunc {
	return func(context *gin.Context) {
		startedAt := time.Now()
		context.Next()
		if api == nil || api.manager == nil || api.manager.observer == nil {
			return
		}
		eventType := types.EventFinished
		var eventErr error
		if value, found := context.Get(restEventErrorKey); found {
			eventErr, _ = value.(error)
		}
		if eventErr != nil || context.Writer.Status() >= http.StatusInternalServerError {
			eventType = types.EventFailed
		}
		attrs := map[string]any{
			"request": map[string]any{
				"method":       context.Request.Method,
				"uri":          context.Request.RequestURI,
				"path":         context.Request.URL.Path,
				"query_params": map[string][]string(context.Request.URL.Query()),
				"headers":      map[string][]string(context.Request.Header),
				"client_ip":    context.ClientIP(),
			},
			"response": map[string]any{
				"status": context.Writer.Status(),
			},
		}
		if code, found := context.Get(restEventCodeKey); found {
			attrs["error"] = map[string]any{"code": code}
		}
		types.EmitEvent(context.Request.Context(), api.manager.observer, &types.Event{
			Type:     eventType,
			Message:  "rest request",
			Duration: time.Since(startedAt),
			Error:    eventErr,
			Subject:  types.EventSubject{Type: "rest", ID: context.FullPath(), Name: context.Request.Method + " " + context.FullPath()},
			Attrs:    attrs,
		})
	}
}

func (api *restAPI) invokeJourney(context *gin.Context) {
	var payload types.JourneyPayloadReq
	body, ok := api.readJSON(context, &payload)
	if !ok {
		return
	}
	realm := restRequestRealm(context)
	if realm == "" {
		writeRESTError(context, http.StatusBadRequest, "invalid_payload", ErrJourneyInvalidPayload)
		return
	}
	payload.SetRealm(&types.Realm{Name: realm})
	responseWriter := newRESTResponseMutator(context)
	execution := &types.JourneyExecute{
		Context: context.Request.Context(), Request: types.NewHTTPRequestAccessorWithBody(context.Request, body, api.maxBodyBytes), Response: responseWriter, Payload: &payload,
	}
	if api.config.PrepareExecution != nil {
		if err := api.config.PrepareExecution(context, execution); err != nil {
			writeRESTError(context, http.StatusForbidden, "execution_rejected", err)
			return
		}
	}
	response, state, err := api.manager.InvokeJourney(execution)
	if err != nil {
		if api.config.OnFailure != nil {
			api.config.OnFailure(context, state, err)
		}
		if responseWriter.applyTerminal() {
			return
		}
		writeJourneyExecutionError(context, state, err)
		return
	}
	if response != nil {
		responseWriter.applyHeaders()
		context.JSON(http.StatusOK, response)
		return
	}
	if api.config.OnSuccess != nil {
		api.config.OnSuccess(context, state)
	}
	if responseWriter.applyTerminal() {
		return
	}
	responseWriter.applyHeaders()
	context.JSON(http.StatusOK, restTerminalResponse(state, false))
}

func (api *restAPI) invokeRouteJourney(context *gin.Context) {
	routePath, insideBasePath := api.resourceJourneyPath(context.Request.URL.Path)
	if !insideBasePath {
		context.Status(http.StatusNotFound)
		return
	}
	match, ok := api.resolveRouteJourney(routePath)
	if !ok {
		context.Status(http.StatusNotFound)
		return
	}
	clearRESTResourceRouteParams(context)
	realm := match.Journey.Realm
	payload := (&types.JourneyPayloadReq{JourneyID: match.Journey.ID}).SetRealm(&types.Realm{Name: realm})
	request := types.NewHTTPRequestAccessor(context.Request, api.maxBodyBytes)
	request.SetRoute(match.RoutePrefix, match.ResourcePath)
	responseWriter := newRESTResponseMutator(context)
	execution := &types.JourneyExecute{
		Context: context.Request.Context(), Request: request, Response: responseWriter, Payload: payload,
	}
	if api.config.PrepareExecution != nil {
		if err := api.config.PrepareExecution(context, execution); err != nil {
			writeRESTError(context, http.StatusForbidden, "execution_rejected", err)
			return
		}
	}
	response, state, err := api.manager.InvokeJourney(execution)
	if err != nil {
		if api.config.OnFailure != nil {
			api.config.OnFailure(context, state, err)
		}
		if responseWriter.applyTerminal() {
			return
		}
		if errors.Is(err, ErrJourneyNotFound) {
			context.Status(http.StatusNotFound)
			return
		}
		writeJourneyExecutionError(context, state, err)
		return
	}
	if response != nil {
		responseWriter.applyHeaders()
		context.JSON(http.StatusOK, response)
		return
	}
	if api.config.OnSuccess != nil {
		api.config.OnSuccess(context, state)
	}
	if responseWriter.applyTerminal() {
		return
	}
	responseWriter.applyHeaders()
	context.JSON(http.StatusOK, restTerminalResponse(state, false))
}

func clearRESTResourceRouteParams(context *gin.Context) {
	if context == nil || len(context.Params) == 0 {
		return
	}
	filtered := context.Params[:0]
	for _, param := range context.Params {
		switch param.Key {
		case "realm", "journeyId", "routePath":
			continue
		default:
			filtered = append(filtered, param)
		}
	}
	context.Params = filtered
}

func normalizeRESTBasePath(value string) string {
	value = "/" + strings.Trim(strings.TrimSpace(value), "/")
	if value == "/" {
		return ""
	}
	return value
}

func (api *restAPI) resourceJourneyPath(requestPath string) (string, bool) {
	basePath := strings.TrimRight(api.basePath, "/")
	if basePath == "" {
		return normalizeRoutePath(requestPath), true
	}
	if requestPath == basePath {
		return "", true
	}
	if !strings.HasPrefix(requestPath, basePath+"/") {
		return "", false
	}
	return normalizeRoutePath(strings.TrimPrefix(requestPath, basePath)), true
}

func newJourneyRouteCache() *journeyRouteCache {
	return &journeyRouteCache{entries: map[string]string{}}
}

func (cache *journeyRouteCache) Get(prefix string) (string, bool) {
	if cache == nil {
		return "", false
	}
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	id, ok := cache.entries[routeCacheKey(prefix)]
	return id, ok
}

func (cache *journeyRouteCache) Set(prefix string, journeyID string) {
	if cache == nil || journeyID == "" {
		return
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.entries[routeCacheKey(prefix)] = journeyID
}

func (cache *journeyRouteCache) Delete(prefix string) {
	if cache == nil {
		return
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	delete(cache.entries, routeCacheKey(prefix))
}

func (cache *journeyRouteCache) Clear() {
	if cache == nil {
		return
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.entries = map[string]string{}
}

func routeCacheKey(prefix string) string {
	return normalizeRoutePath(prefix)
}

func (api *restAPI) resolveRouteJourney(routePath string) (restRouteJourneyMatch, bool) {
	normalizedPath := normalizeRoutePath(routePath)
	if normalizedPath == "" {
		return restRouteJourneyMatch{}, false
	}
	parts := strings.Split(strings.TrimPrefix(normalizedPath, "/"), "/")
	prefix := "/" + parts[0]
	if journeyID, ok := api.routeCache.Get(prefix); ok {
		if journey, err := api.journeys.Load(journeyID); err == nil && api.routeJourneyMatches(journey, prefix) {
			return restRouteJourneyMatch{Journey: journey, RoutePrefix: prefix, ResourcePath: resourcePathForPrefix(normalizedPath, prefix)}, true
		}
		api.routeCache.Delete(prefix)
	}
	if journey, ok := api.findRouteJourneyByName(prefix); ok {
		api.routeCache.Set(prefix, journey.ID)
		return restRouteJourneyMatch{Journey: journey, RoutePrefix: prefix, ResourcePath: resourcePathForPrefix(normalizedPath, prefix)}, true
	}
	return restRouteJourneyMatch{}, false
}

func (api *restAPI) findRouteJourneyByName(prefix string) (*types.JourneyConfiguration, bool) {
	for _, journey := range api.journeys.ListOfCache() {
		if api.routeJourneyMatches(journey, prefix) {
			return journey, true
		}
	}
	return nil, false
}

func (api *restAPI) routeJourneyMatches(journey *types.JourneyConfiguration, prefix string) bool {
	if journey == nil || !journey.Active || normalizeRoutePath(journey.Name) != prefix {
		return false
	}
	return types.NormalizeJourneyType(journey.JourneyType) == types.ResourceJourney
}

func normalizeRoutePath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parts := make([]string, 0)
	for _, part := range strings.Split(value, "/") {
		if part == "" {
			continue
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return ""
	}
	return "/" + strings.Join(parts, "/")
}

func resourcePathForPrefix(routePath string, prefix string) string {
	routePath = normalizeRoutePath(routePath)
	prefix = normalizeRoutePath(prefix)
	if routePath == prefix {
		return "/"
	}
	resourcePath := strings.TrimPrefix(routePath, prefix)
	if resourcePath == "" {
		return "/"
	}
	if !strings.HasPrefix(resourcePath, "/") {
		resourcePath = "/" + resourcePath
	}
	return resourcePath
}

func (api *restAPI) resolveJourneySaveTarget(journey *types.JourneyConfiguration) (existing *types.JourneyConfiguration, duplicate *types.JourneyConfiguration) {
	if journey == nil {
		return nil, nil
	}
	id := restMetadataID(journey.Metadata)
	if id != "" {
		if current, err := api.journeys.Load(id); err == nil {
			existing = current
		}
	}
	duplicate = api.findJourneyByNameRealm(journey.Name, journey.Realm, id)
	return existing, duplicate
}

func (api *restAPI) findJourneyByNameRealm(name string, realm string, excludeID string) *types.JourneyConfiguration {
	name = strings.TrimSpace(name)
	realm = strings.TrimSpace(realm)
	if name == "" {
		return nil
	}
	for _, existing := range api.journeys.ListOfCache() {
		if existing != nil && existing.Name == name && existing.Realm == realm && restMetadataID(existing.Metadata) != excludeID {
			return existing
		}
	}
	return nil
}

func restMetadataID(metadata *nstore.Metadata) string {
	if metadata == nil {
		return ""
	}
	return strings.TrimSpace(metadata.ID)
}

func (api *restAPI) findExistingScript(script *jsrun.Script) *jsrun.Script {
	if script == nil {
		return nil
	}
	if script.Metadata != nil && strings.TrimSpace(script.Metadata.ID) != "" {
		if existing, err := api.scripts.Load(script.Metadata.ID); err == nil {
			return existing
		}
	}
	name := strings.TrimSpace(script.Name)
	if name == "" {
		return nil
	}
	for _, existing := range api.scripts.ListOfCache() {
		if existing != nil && existing.Name == name && steps.NormalizeScriptType(existing.Type) == steps.NormalizeScriptType(script.Type) {
			return existing
		}
	}
	return nil
}

func (api *restAPI) findScriptByNameType(name string, scriptType string, excludeID string) *jsrun.Script {
	name = strings.TrimSpace(name)
	scriptType = steps.NormalizeScriptType(scriptType)
	if name == "" {
		return nil
	}
	for _, existing := range api.scripts.ListOfCache() {
		if existing != nil && existing.Name == name && steps.NormalizeScriptType(existing.Type) == scriptType && restMetadataID(existing.Metadata) != excludeID {
			return existing
		}
	}
	return nil
}

func restQueryLimit(context *gin.Context) (int, bool) {
	raw := strings.TrimSpace(context.Query("limit"))
	if raw == "" {
		return 0, true
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 0 {
		writeRESTError(context, http.StatusBadRequest, "invalid_limit", errors.New("limit must be a non-negative integer"))
		return 0, false
	}
	return limit, true
}

func validateRESTScript(script *jsrun.Script) error {
	if script != nil {
		script.Type = steps.NormalizeScriptType(script.Type)
	}
	if script == nil || strings.TrimSpace(script.Name) == "" || !steps.IsAllowedType(script.Type) {
		return errors.New("script name and supported type are required")
	}
	if err := steps.NormalizeDeclaredScriptOutcomes(script); err != nil {
		return err
	}
	code, err := script.GetRawCode()
	if err != nil {
		return err
	}
	if strings.TrimSpace(code) == "" {
		return errors.New("script code is required")
	}
	if script.Type == steps.LibraryScript {
		code = "(function(exports, module) { " + code + "\n})"
	}
	_, err = goja.Compile(script.Name, code, false)
	return err
}

func restTerminalResponse(state *types.JourneyState, failed bool) gin.H {
	response := gin.H{}
	if state == nil {
		return response
	}
	if realm := state.GetRealm(); realm != "" {
		response["realm"] = realm
	}
	closedCtx := state.GetClosedCtx()
	if failed {
		if closedCtx.IsDefined(env.GetContextKey("failure_url")) {
			response["failure_url"] = closedCtx.Get(env.GetContextKey("failure_url")).AsStringOr("")
		}
		if closedCtx.IsDefined(env.GetContextKey("error_data")) {
			response["data"] = closedCtx.Get(env.GetContextKey("error_data")).AsAnyOr(nil)
		}
		return response
	}
	if closedCtx.IsDefined(env.GetContextKey("success_url")) {
		response["success_url"] = closedCtx.Get(env.GetContextKey("success_url")).AsStringOr("")
	}
	if closedCtx.IsDefined(env.GetContextKey("data")) {
		response["data"] = closedCtx.Get(env.GetContextKey("data")).AsAnyOr(nil)
	}
	return response
}

func hasFailureTerminalResponse(state *types.JourneyState) bool {
	if state == nil {
		return false
	}
	closedCtx := state.GetClosedCtx()
	return closedCtx.IsDefined(env.GetContextKey("failure_url")) || closedCtx.IsDefined(env.GetContextKey("error_data"))
}

func writeJourneyExecutionError(context *gin.Context, state *types.JourneyState, err error) {
	switch {
	case errors.Is(err, ErrJourneyNotFound):
		writeRESTError(context, http.StatusNotFound, "journey_not_found", err)
	case errors.Is(err, ErrInvalidJourneyToken):
		writeRESTError(context, http.StatusUnauthorized, "invalid_journey_token", err)
	case errors.Is(err, ErrJourneyInvalidPayload):
		writeRESTError(context, http.StatusBadRequest, "invalid_payload", err)
	case errors.Is(err, ErrJourneyFailure):
		if hasFailureTerminalResponse(state) {
			context.AbortWithStatusJSON(http.StatusUnauthorized, restTerminalResponse(state, true))
			return
		}
		writeRESTError(context, http.StatusUnauthorized, "journey_failed", err)
	default:
		writeRESTError(context, http.StatusInternalServerError, "journey_execution_failed", err)
	}
}

func writeRESTError(context *gin.Context, status int, code string, err error) {
	context.Set(restEventErrorKey, err)
	context.Set(restEventCodeKey, code)
	context.AbortWithStatusJSON(status, restErrorBody{Error: restError{Code: code, Message: err.Error()}})
}
