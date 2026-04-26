package adapterconfig

import (
	"fmt"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/compatibility"
)

func CompatibilityCandidates(cfg Config, model string, sourceAPI adapt.ApiKind) ([]compatibility.Candidate, error) {
	resolutions, err := ResolveModelCandidates(cfg, model, sourceAPI)
	if err != nil {
		return nil, err
	}
	out := make([]compatibility.Candidate, 0, len(resolutions))
	for _, resolution := range resolutions {
		out = append(out, CompatibilityCandidate(resolution))
	}
	return out, nil
}

func EvaluateCompatibilityCandidates(cfg Config, model string, sourceAPI adapt.ApiKind, useCase compatibility.UseCase) ([]compatibility.Evaluation, error) {
	profile, ok := compatibility.BuiltinProfile(useCase)
	if !ok {
		return nil, compatibilityUnknownUseCaseError(useCase)
	}
	candidates, err := CompatibilityCandidates(cfg, model, sourceAPI)
	if err != nil {
		return nil, err
	}
	return compatibility.EvaluateMany(candidates, profile), nil
}

func CompatibleCandidates(cfg Config, model string, sourceAPI adapt.ApiKind, useCase compatibility.UseCase, includeDegraded bool) ([]compatibility.Candidate, error) {
	evaluations, err := EvaluateCompatibilityCandidates(cfg, model, sourceAPI, useCase)
	if err != nil {
		return nil, err
	}
	out := make([]compatibility.Candidate, 0, len(evaluations))
	for _, evaluation := range evaluations {
		if evaluation.Status == compatibility.StatusApproved || (includeDegraded && evaluation.Status == compatibility.StatusDegraded) {
			out = append(out, evaluation.Candidate)
		}
	}
	return out, nil
}

func CompatibilityCandidate(resolution ModelResolutionCandidate) compatibility.Candidate {
	return compatibility.Candidate{
		Input:                resolution.Input,
		MatchedAs:            resolution.MatchedAs,
		SourceAPI:            resolution.SourceAPI,
		PublicModel:          resolution.PublicModel,
		NativeModel:          resolution.NativeModel,
		Provider:             resolution.Provider,
		ProviderType:         resolution.ProviderType,
		ProviderAPI:          resolution.ProviderAPI,
		Family:               resolution.Family,
		Weight:               resolution.Weight,
		Priority:             resolution.Priority,
		ModelDBService:       resolution.ModelDBService,
		Capabilities:         resolution.Capabilities,
		CapabilitySource:     resolution.CapabilitySource,
		ConsumerContinuation: resolution.ConsumerContinuation,
		InternalContinuation: resolution.InternalContinuation,
		Transport:            resolution.Transport,
		Features: compatibility.CandidateFeatures(
			resolution.Capabilities,
			resolution.CapabilitySource,
			resolution.ModelDBService,
		),
	}
}

func compatibilityUnknownUseCaseError(useCase compatibility.UseCase) error {
	return fmt.Errorf("unknown use case %q", useCase)
}
