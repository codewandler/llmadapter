package adapterconfig

import (
	"testing"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/stretchr/testify/require"
)

func TestAutoResultRouteSummary(t *testing.T) {
	result := AutoResult{
		Config: Config{Routes: []RouteConfig{{
			SourceAPI:   adapt.ApiOpenAIResponses,
			Model:       "default",
			Provider:    "openai_responses",
			ProviderAPI: adapt.ApiOpenAIResponses,
			NativeModel: "gpt-test",
		}}},
		Enabled: []AutoProvider{{Name: "openai_responses", Type: "openai_responses", Reason: "env:OPENAI_API_KEY"}},
	}

	summary, ok := result.RouteSummary(adapt.ApiOpenAIResponses, "default")
	require.True(t, ok)
	require.Equal(t, "openai_responses", summary.Provider)
	require.Equal(t, "gpt-test", summary.NativeModel)
	require.Equal(t, "env:OPENAI_API_KEY", summary.EnabledReason)
}

func TestAutoResultRouteSummaryDefaultsSourceAPI(t *testing.T) {
	result := AutoResult{Config: Config{Routes: []RouteConfig{{
		SourceAPI: adapt.ApiOpenAIResponses,
		Model:     "default",
		Provider:  "openai_responses",
	}}}}

	summary, ok := result.RouteSummary("", "")
	require.True(t, ok)
	require.Equal(t, adapt.ApiOpenAIResponses, summary.SourceAPI)
	require.Equal(t, "default", summary.NativeModel)
}
