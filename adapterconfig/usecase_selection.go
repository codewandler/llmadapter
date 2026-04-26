package adapterconfig

import (
	"fmt"
	"strings"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/compatibility"
	"github.com/codewandler/modeldb"
)

type UseCaseSelectionOptions struct {
	UseCase       compatibility.UseCase
	Evidence      CompatibilityEvidence
	AllowDegraded bool
	AllowUntested bool
}

type UseCaseModelSelection struct {
	Resolution ModelResolutionCandidate `json:"resolution"`
	Evaluation compatibility.Evaluation `json:"evaluation"`
	Evidence   CompatibilityRowEvidence `json:"evidence,omitempty"`
	RuntimeID  string                   `json:"runtime_id,omitempty"`
	Item       modeldb.Item             `json:"-"`
}

func SelectModelForUseCase(cfg Config, model string, sourceAPI adapt.ApiKind, opts UseCaseSelectionOptions) (UseCaseModelSelection, error) {
	selections, err := SelectModelsForUseCase(cfg, model, sourceAPI, opts)
	if err != nil {
		return UseCaseModelSelection{}, err
	}
	return selections[0], nil
}

func SelectModelsForUseCase(cfg Config, model string, sourceAPI adapt.ApiKind, opts UseCaseSelectionOptions) ([]UseCaseModelSelection, error) {
	useCase := opts.UseCase
	if useCase == "" {
		useCase = compatibility.UseCaseAgenticCoding
	}
	profile, ok := compatibility.BuiltinProfile(useCase)
	if !ok {
		return nil, compatibilityUnknownUseCaseError(useCase)
	}
	if opts.Evidence.UseCase == "" || len(opts.Evidence.Rows) == 0 {
		return nil, fmt.Errorf("use-case selection for %q requires compatibility evidence", useCase)
	}
	if opts.Evidence.UseCase != useCase {
		return nil, fmt.Errorf("compatibility evidence use case %q does not match requested use case %q", opts.Evidence.UseCase, useCase)
	}
	viewCandidates, err := useCaseRuntimeViewCandidates(cfg, model, sourceAPI)
	if err != nil {
		return nil, err
	}
	var out []UseCaseModelSelection
	var rejected []string
	for _, candidate := range viewCandidates {
		evidence, ok := opts.Evidence.match(candidate.Resolution)
		if !ok {
			rejected = append(rejected, useCaseRejectReason(candidate.Resolution, "no_live_evidence"))
			continue
		}
		compatCandidate := CompatibilityCandidate(candidate.Resolution)
		compatCandidate.Features = applyCompatibilityEvidence(compatCandidate.Features, evidence)
		evaluation := compatibility.Evaluate(compatCandidate, profile)
		evaluation.Status = evidence.Status
		if !useCaseStatusAllowed(evidence.Status, opts) {
			rejected = append(rejected, useCaseRejectReason(candidate.Resolution, string(evidence.Status)))
			continue
		}
		out = append(out, UseCaseModelSelection{
			Resolution: candidate.Resolution,
			Evaluation: evaluation,
			Evidence:   evidence,
			RuntimeID:  candidate.RuntimeID,
			Item:       candidate.Item,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("model %q has no candidates approved for use case %q: %s", model, useCase, strings.Join(rejected, ", "))
	}
	return out, nil
}

func (r AutoResult) SelectModelForUseCase(model string, sourceAPI adapt.ApiKind, opts UseCaseSelectionOptions) (UseCaseModelSelection, error) {
	return SelectModelForUseCase(r.Config, model, sourceAPI, opts)
}

func (r AutoResult) SelectModelsForUseCase(model string, sourceAPI adapt.ApiKind, opts UseCaseSelectionOptions) ([]UseCaseModelSelection, error) {
	return SelectModelsForUseCase(r.Config, model, sourceAPI, opts)
}

type runtimeViewCandidate struct {
	Resolution ModelResolutionCandidate
	RuntimeID  string
	Item       modeldb.Item
}

func useCaseRuntimeViewCandidates(cfg Config, model string, sourceAPI adapt.ApiKind) ([]runtimeViewCandidate, error) {
	resolutions, err := ResolveModelCandidates(cfg, model, sourceAPI)
	if err != nil {
		return nil, err
	}
	catalog, modelDBEnabled, err := modelDBCatalog(cfg)
	if err != nil {
		return nil, err
	}
	if !modelDBEnabled {
		return nil, fmt.Errorf("use-case selection requires modeldb-backed config")
	}
	resolved := modeldb.NewResolvedCatalog(catalog)
	for _, resolution := range resolutions {
		if resolution.ModelDBService == "" || resolution.NativeModel == "" {
			continue
		}
		ref := modeldb.OfferingRef{ServiceID: resolution.ModelDBService, WireModelID: resolution.NativeModel}
		if _, ok := catalog.OfferingByRef(ref); !ok {
			continue
		}
		runtimeID := modelDBRuntimeID(resolution)
		resolved.Runtimes[runtimeID] = modeldb.Runtime{
			ID:        runtimeID,
			ServiceID: resolution.ModelDBService,
			Name:      resolution.Provider,
		}
		resolved.RuntimeAccess[modeldb.RuntimeAccessKey{
			RuntimeID:   runtimeID,
			ServiceID:   ref.ServiceID,
			WireModelID: ref.WireModelID,
		}] = modeldb.RuntimeAccess{
			RuntimeID:      runtimeID,
			Offering:       ref,
			Routable:       true,
			ResolvedWireID: resolution.NativeModel,
			Reason:         string(resolution.ProviderAPI),
		}
		resolved.RuntimeAcquisition[modeldb.RuntimeAcquisitionKey{
			RuntimeID:   runtimeID,
			ServiceID:   ref.ServiceID,
			WireModelID: ref.WireModelID,
		}] = modeldb.RuntimeAcquisition{
			RuntimeID: runtimeID,
			Offering:  ref,
			Known:     true,
			Status:    "configured",
		}
	}
	out := make([]runtimeViewCandidate, 0, len(resolutions))
	for _, resolution := range resolutions {
		runtimeID := modelDBRuntimeID(resolution)
		view := modeldb.RuntimeView(resolved, runtimeID, modeldb.ViewOptions{
			RoutableOnly: true,
			AliasOverlay: modelDBAliasOverlay(cfg.ModelDB),
		})
		item, ok := resolveRuntimeViewItem(view, resolution)
		if !ok {
			continue
		}
		out = append(out, runtimeViewCandidate{Resolution: resolution, RuntimeID: runtimeID, Item: item})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("model %q has no modeldb runtime-view candidates", model)
	}
	return out, nil
}

func resolveRuntimeViewItem(view modeldb.View, resolution ModelResolutionCandidate) (modeldb.Item, bool) {
	for _, name := range []string{resolution.Input, resolution.PublicModel, resolution.NativeModel} {
		if name == "" {
			continue
		}
		if item, ok := view.Resolve(name); ok {
			return item, true
		}
		if normalized := normalizeModelDBAlias(name); normalized != name {
			if item, ok := view.Resolve(normalized); ok {
				return item, true
			}
		}
	}
	ref := modeldb.OfferingRef{ServiceID: resolution.ModelDBService, WireModelID: resolution.NativeModel}
	if item, ok := view.Find(func(item modeldb.Item) bool {
		return item.Offering.ServiceID == ref.ServiceID && item.Offering.WireModelID == ref.WireModelID
	}); ok {
		return item, true
	}
	return modeldb.Item{}, false
}

func modelDBRuntimeID(resolution ModelResolutionCandidate) string {
	return "llmadapter-" + modelDBRuntimeIDPart(string(resolution.ProviderAPI)) + "-" + modelDBRuntimeIDPart(resolution.Provider)
}

func modelDBRuntimeIDPart(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var out strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			out.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			out.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(out.String(), "-")
}

func (e CompatibilityEvidence) match(resolution ModelResolutionCandidate) (CompatibilityRowEvidence, bool) {
	for _, row := range e.Rows {
		if row.Provider != resolution.Provider {
			continue
		}
		if row.ProviderAPI != "" && row.ProviderAPI != resolution.ProviderAPI {
			continue
		}
		if row.NativeModel != "" && !sameWireModel(row.NativeModel, resolution.NativeModel) {
			continue
		}
		if row.PublicModel != "" && resolution.PublicModel != "" && row.PublicModel != resolution.PublicModel && row.PublicModel != resolution.Input {
			continue
		}
		return row, true
	}
	return CompatibilityRowEvidence{}, false
}

func sameWireModel(left, right string) bool {
	if left == right {
		return true
	}
	_, leftRest, leftQualified := modelDBServiceQualifiedName(left)
	_, rightRest, rightQualified := modelDBServiceQualifiedName(right)
	switch {
	case leftQualified && leftRest == right:
		return true
	case rightQualified && rightRest == left:
		return true
	case leftQualified && rightQualified && leftRest == rightRest:
		return true
	default:
		return false
	}
}

func useCaseStatusAllowed(status compatibility.Status, opts UseCaseSelectionOptions) bool {
	switch status {
	case compatibility.StatusApproved:
		return true
	case compatibility.StatusDegraded:
		return opts.AllowDegraded
	case compatibility.StatusUntested:
		return opts.AllowUntested
	default:
		return false
	}
}

func useCaseRejectReason(resolution ModelResolutionCandidate, reason string) string {
	return fmt.Sprintf("%s/%s/%s:%s", resolution.Provider, resolution.ProviderAPI, resolution.NativeModel, reason)
}

func applyCompatibilityEvidence(features []compatibility.FeatureEvidence, row CompatibilityRowEvidence) []compatibility.FeatureEvidence {
	out := make([]compatibility.FeatureEvidence, len(features))
	copy(out, features)
	live := map[compatibility.Feature]bool{
		compatibility.FeatureStreamingText:    row.Text == "live",
		compatibility.FeatureTools:            row.Tools == "live",
		compatibility.FeatureToolContinuation: row.ToolContinuation == "live",
		compatibility.FeatureStructuredOutput: row.StructuredOutput == "live",
		compatibility.FeatureReasoning:        row.Reasoning == "live",
		compatibility.FeaturePromptCaching:    row.PromptCaching == "live",
		compatibility.FeatureUsage:            row.Usage == "live",
		compatibility.FeatureCacheAccounting:  row.CacheAccounting == "live",
	}
	for i, feature := range out {
		if live[feature.Feature] {
			out[i].Supported = true
			out[i].Evidence = compatibility.EvidenceLive
			out[i].Detail = "live compatibility evidence"
		}
	}
	return out
}
