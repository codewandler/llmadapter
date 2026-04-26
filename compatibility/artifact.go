package compatibility

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/codewandler/llmadapter/adapt"
)

type Artifact struct {
	UseCase              UseCase   `json:"use_case"`
	ResultDate           string    `json:"result_date"`
	Command              string    `json:"command,omitempty"`
	TotalDurationSeconds float64   `json:"total_duration_seconds,omitempty"`
	RequiredFeatures     []Feature `json:"required_features"`
	PreferredFeatures    []Feature `json:"preferred_features,omitempty"`
	Rows                 []Row     `json:"rows"`
	Notes                []string  `json:"notes,omitempty"`
}

type Row struct {
	Candidate        string          `json:"candidate,omitempty"`
	PublicModel      string          `json:"public_model,omitempty"`
	NativeModel      string          `json:"native_model,omitempty"`
	Provider         string          `json:"provider"`
	ProviderAPI      adapt.ApiKind   `json:"provider_api"`
	Family           adapt.ApiFamily `json:"family,omitempty"`
	RequiredStatus   string          `json:"required_status,omitempty"`
	Status           Status          `json:"status"`
	DurationSeconds  float64         `json:"duration_seconds,omitempty"`
	Text             string          `json:"text,omitempty"`
	Tools            string          `json:"tools,omitempty"`
	ToolContinuation string          `json:"tool_continuation,omitempty"`
	StructuredOutput string          `json:"structured_output,omitempty"`
	Reasoning        string          `json:"reasoning,omitempty"`
	PromptCaching    string          `json:"prompt_caching,omitempty"`
	Usage            string          `json:"usage,omitempty"`
	CacheAccounting  string          `json:"cache_accounting,omitempty"`
}

func NewArtifact(useCase UseCase, command string, rows []Row, totalDurationSeconds float64, notes []string) (Artifact, error) {
	profile, ok := BuiltinProfile(useCase)
	if !ok {
		return Artifact{}, fmt.Errorf("unknown use case %q", useCase)
	}
	required, preferred := artifactRequirements(profile)
	return Artifact{
		UseCase:              useCase,
		ResultDate:           time.Now().Format("2006-01-02"),
		Command:              command,
		TotalDurationSeconds: totalDurationSeconds,
		RequiredFeatures:     required,
		PreferredFeatures:    preferred,
		Rows:                 rows,
		Notes:                notes,
	}, nil
}

func LoadArtifact(path string) (Artifact, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Artifact{}, fmt.Errorf("load compatibility artifact %q: %w", path, err)
	}
	var artifact Artifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		return Artifact{}, fmt.Errorf("decode compatibility artifact %q: %w", path, err)
	}
	return artifact, nil
}

func SaveArtifact(path string, artifact Artifact) error {
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return fmt.Errorf("encode compatibility artifact: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write compatibility artifact %q: %w", path, err)
	}
	return nil
}

func artifactRequirements(profile Profile) ([]Feature, []Feature) {
	var required []Feature
	var preferred []Feature
	for feature, level := range profile.Requirements {
		switch level {
		case RequirementRequired:
			required = append(required, feature)
		case RequirementPreferred:
			preferred = append(preferred, feature)
		}
	}
	sortFeatures(required)
	sortFeatures(preferred)
	return required, preferred
}

func sortFeatures(features []Feature) {
	sort.Slice(features, func(i, j int) bool {
		return features[i] < features[j]
	})
}
