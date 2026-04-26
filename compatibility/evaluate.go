package compatibility

import "fmt"

func Evaluate(candidate Candidate, profile Profile) Evaluation {
	evidence := map[Feature]FeatureEvidence{}
	for _, item := range candidate.Features {
		evidence[item.Feature] = item
	}

	out := Evaluation{
		Candidate: candidate,
		UseCase:   profile.UseCase,
		Status:    StatusApproved,
	}

	for _, feature := range sortedFeatures(profile.Requirements) {
		requirement := profile.Requirements[feature]
		item, ok := evidence[feature]
		if !ok {
			item = FeatureEvidence{Feature: feature, Evidence: EvidenceUntested}
		}
		result := FeatureResult{
			Feature:     feature,
			Requirement: requirement,
			Supported:   item.Supported,
			Evidence:    item.Evidence,
			Detail:      item.Detail,
		}
		out.Features = append(out.Features, result)
		switch requirement {
		case RequirementRequired:
			if !item.Supported {
				if item.Evidence == EvidenceUntested {
					out.UntestedRequired = append(out.UntestedRequired, feature)
				} else {
					out.MissingRequired = append(out.MissingRequired, feature)
				}
			}
		case RequirementPreferred:
			if !item.Supported || item.Evidence == EvidenceUntested {
				out.DegradedPreferred = append(out.DegradedPreferred, feature)
			}
		case RequirementOptional:
			if !item.Supported && item.Evidence == EvidenceUnsupported {
				out.UnsupportedOptional = append(out.UnsupportedOptional, feature)
			}
		}
	}

	switch {
	case len(out.MissingRequired) > 0:
		out.Status = StatusFailed
	case len(out.UntestedRequired) > 0:
		out.Status = StatusUntested
	case len(out.DegradedPreferred) > 0:
		out.Status = StatusDegraded
	default:
		out.Status = StatusApproved
	}

	return out
}

func EvaluateMany(candidates []Candidate, profile Profile) []Evaluation {
	out := make([]Evaluation, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out, Evaluate(candidate, profile))
	}
	return out
}

func Filter(evaluations []Evaluation, statuses ...Status) []Evaluation {
	allowed := map[Status]bool{}
	for _, status := range statuses {
		allowed[status] = true
	}
	out := make([]Evaluation, 0, len(evaluations))
	for _, evaluation := range evaluations {
		if allowed[evaluation.Status] {
			out = append(out, evaluation)
		}
	}
	return out
}

func ParseUseCase(value string) (UseCase, error) {
	useCase := UseCase(value)
	if _, ok := BuiltinProfile(useCase); !ok {
		return "", fmt.Errorf("unknown use case %q", value)
	}
	return useCase, nil
}
