package adapterconfig

import (
	"os"
	"strings"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/router"
	"github.com/codewandler/modeldb"
)

func resolveRuntimeNativeModel(endpoint router.ProviderEndpoint, catalog modeldb.Catalog, serviceID string, wireModelID string) string {
	if endpoint.Family != adapt.FamilyBedrockConverse || serviceID != "bedrock" || wireModelID == "" || hasBedrockInferenceProfilePrefix(wireModelID) {
		return wireModelID
	}
	runtimeID := bedrockRuntimeIDFromEnv()
	for _, candidateRuntimeID := range runtimeFallbacks(runtimeID) {
		access, ok := catalog.RuntimeAccess[modeldb.RuntimeAccessKey{
			RuntimeID:   candidateRuntimeID,
			ServiceID:   serviceID,
			WireModelID: wireModelID,
		}]
		if ok && access.Routable && access.ResolvedWireID != "" {
			return access.ResolvedWireID
		}
	}
	return wireModelID
}

func bedrockRuntimeIDFromEnv() string {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}
	if region == "" {
		region = "us-east-1"
	}
	return "bedrock-" + bedrockRuntimePrefix(region)
}

func bedrockRuntimePrefix(region string) string {
	switch {
	case strings.HasPrefix(region, "us-"):
		return "us"
	case strings.HasPrefix(region, "eu-"):
		return "eu"
	case strings.HasPrefix(region, "ap-"):
		return "apac"
	default:
		return "global"
	}
}

func runtimeFallbacks(runtimeID string) []string {
	if runtimeID == "" || runtimeID == "bedrock-global" {
		return []string{"bedrock-global"}
	}
	return []string{runtimeID, "bedrock-global"}
}

func hasBedrockInferenceProfilePrefix(model string) bool {
	for _, prefix := range []string{"global.", "us.", "eu.", "apac."} {
		if strings.HasPrefix(model, prefix) && len(model) > len(prefix) {
			return true
		}
	}
	return false
}
