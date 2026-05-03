package converse

import "strings"

const (
	RegionUSEast1 = "us-east-1"
	DefaultRegion = RegionUSEast1

	PrefixEU     = "eu"
	PrefixUS     = "us"
	PrefixAPAC   = "apac"
	PrefixGlobal = "global"
)

var regionPrefixes = map[string]string{
	"eu-": PrefixEU,
	"us-": PrefixUS,
	"ap-": PrefixAPAC,
}

var validPrefixes = []string{
	PrefixEU + ".",
	PrefixUS + ".",
	PrefixAPAC + ".",
	PrefixGlobal + ".",
}

type inferenceProfile struct {
	Prefixes []string
}

var inferenceProfiles = map[string]inferenceProfile{
	ModelClaudeSonnet46: {Prefixes: []string{PrefixEU, PrefixUS, PrefixGlobal}},
	ModelClaudeHaiku45:  {Prefixes: []string{PrefixEU, PrefixUS, PrefixGlobal}},
	ModelClaudeOpus47:   {Prefixes: []string{PrefixEU, PrefixUS, PrefixGlobal}},
	ModelClaudeOpus46:   {Prefixes: []string{PrefixEU, PrefixUS, PrefixGlobal}},
	ModelClaudeSonnet45: {Prefixes: []string{PrefixEU, PrefixUS, PrefixGlobal}},
}

func computeRegionPrefix(region string) string {
	for regionPrefix, profilePrefix := range regionPrefixes {
		if strings.HasPrefix(region, regionPrefix) {
			return profilePrefix
		}
	}
	return PrefixGlobal
}

func hasInferenceProfilePrefix(model string) bool {
	for _, prefix := range validPrefixes {
		if strings.HasPrefix(model, prefix) && len(model) > len(prefix) {
			return true
		}
	}
	return false
}

func stripInferenceProfilePrefix(model string) string {
	for _, prefix := range validPrefixes {
		if strings.HasPrefix(model, prefix) && len(model) > len(prefix) {
			return model[len(prefix):]
		}
	}
	return model
}

func containsPrefix(prefixes []string, prefix string) bool {
	for _, p := range prefixes {
		if p == prefix {
			return true
		}
	}
	return false
}
