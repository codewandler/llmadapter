package citations

import (
	"encoding/json"

	"github.com/codewandler/llmadapter/unified"
)

type Spec struct {
	TextKeys       []string
	TitleKeys      []string
	DocumentIDKeys []string
	StartKeys      []string
	EndKeys        []string
}

func FromMap(values map[string]any, spec Spec) unified.Citation {
	known := map[string]bool{"type": true, "url": true}
	markKnown(known, spec.TextKeys...)
	markKnown(known, spec.TitleKeys...)
	markKnown(known, spec.DocumentIDKeys...)
	markKnown(known, spec.StartKeys...)
	markKnown(known, spec.EndKeys...)

	return unified.Citation{
		Type:        StringValue(values["type"]),
		Text:        FirstStringValue(values, spec.TextKeys...),
		URL:         StringValue(values["url"]),
		Title:       FirstStringValue(values, spec.TitleKeys...),
		StartOffset: IntValue(FirstPresent(values, spec.StartKeys...)),
		EndOffset:   IntValue(FirstPresent(values, spec.EndKeys...)),
		DocumentID:  FirstStringValue(values, spec.DocumentIDKeys...),
		Meta:        ExtraMeta(values, known),
	}
}

func FirstStringValue(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := StringValue(values[key]); value != "" {
			return value
		}
	}
	return ""
}

func FirstPresent(values map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			return value
		}
	}
	return nil
}

func StringValue(value any) string {
	text, _ := value.(string)
	return text
}

func IntValue(value any) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		i, _ := v.Int64()
		return int(i)
	default:
		return 0
	}
}

func ExtraMeta(values map[string]any, known map[string]bool) map[string]any {
	var meta map[string]any
	for key, value := range values {
		if known[key] {
			continue
		}
		if meta == nil {
			meta = make(map[string]any)
		}
		meta[key] = value
	}
	return meta
}

func markKnown(known map[string]bool, keys ...string) {
	for _, key := range keys {
		known[key] = true
	}
}
