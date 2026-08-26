package env

import goconf "github.com/nitsugaro/go-conf"

const JourneyPrefixConfig = "journey."

var clientInputsConfigKey string
var contextPrefixKey string
var extendJourneyExpKey string
var suspendJourneyKey string

func GetOptionalJourneyField[T any](name string, defaultVal T) T {
	return goconf.GetOpField(JourneyPrefixConfig+name, defaultVal)
}

func GetJourneyField[T any](name string) (T, error) {
	return goconf.GetField[T](JourneyPrefixConfig + name)
}

func SetEnvironment() {
	clientInputsConfigKey = GetOptionalJourneyField("client_inputs_key", "client_inputs")
	contextPrefixKey = GetOptionalJourneyField("context_prefix_key", "journey_")
	extendJourneyExpKey = contextPrefixKey + "extend_journey_exp"
	suspendJourneyKey = contextPrefixKey + "suspend_journey"
}

/* ####### GETTERS ####### */

func GetClientInputsKey() string {
	if clientInputsConfigKey == "" {
		return "client_inputs"
	}
	return clientInputsConfigKey
}

func GetContextPrefixKey() string {
	if contextPrefixKey == "" {
		return "journey_"
	}
	return contextPrefixKey
}

func GetMaxTickCount() int64 {
	return 500
}

func GetMinTickWindowMs() int64 {
	return 2000
}

// GetContextKey returns an internal context key using the configured prefix.
func GetContextKey(name string) string {
	return GetContextPrefixKey() + name
}

func GetExtendJourneyExpKey() string {
	if extendJourneyExpKey == "" {
		return GetContextKey("extend_journey_exp")
	}
	return extendJourneyExpKey
}

func GetSuspendJourneyKey() string {
	if suspendJourneyKey == "" {
		return GetContextKey("suspend_journey")
	}
	return suspendJourneyKey
}
