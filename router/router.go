package router

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/unified"
)

type Route struct {
	SourceAPI            adapt.ApiKind
	TargetAPI            adapt.ApiKind
	TargetFamily         adapt.ApiFamily
	ProviderName         string
	PublicModel          string
	NativeModel          string
	Client               unified.Client
	Capabilities         CapabilitySet
	ConsumerContinuation unified.ContinuationMode
	InternalContinuation unified.ContinuationMode
	Transport            unified.TransportKind
	ModelMetadata        *unified.ResolvedModelMetadata
}

type Router interface {
	Route(ctx context.Context, req adapt.Request) (Route, error)
}

type CandidateRouter interface {
	Router
	Routes(ctx context.Context, req adapt.Request) ([]Route, error)
}

type ProviderEndpoint struct {
	ProviderName         string
	APIKind              adapt.ApiKind
	Family               adapt.ApiFamily
	Client               unified.Client
	Capabilities         CapabilitySet
	ConsumerContinuation unified.ContinuationMode
	InternalContinuation unified.ContinuationMode
	Transport            unified.TransportKind
	Priority             int
	Tags                 map[string]string
}

type CapabilitySet struct {
	Streaming bool

	Tools         bool
	ParallelTools bool

	Vision      bool
	AudioInput  bool
	AudioOutput bool

	JSONMode   bool
	JSONSchema bool

	Reasoning       bool
	ReasoningDeltas bool

	Citations bool

	BuiltInWebSearch  bool
	BuiltInFileSearch bool
	CodeExecution     bool
	ComputerUse       bool

	ServerSideState bool
	PromptCaching   bool

	MaxInputTokens  int
	MaxOutputTokens int
}

type StaticRouter struct {
	routes []StaticRoute
}

type StaticRoute struct {
	SourceAPI          adapt.ApiKind
	Model              string
	NativeModel        string
	DynamicModels      bool
	Weight             int
	Endpoint           ProviderEndpoint
	CapabilityResolver CapabilityResolver
	ModelResolver      ModelResolver
	ModelMetadata      *unified.ResolvedModelMetadata
}

type CapabilityResolver func(context.Context, adapt.Request, ProviderEndpoint) CapabilitySet

type ModelResolution struct {
	NativeModel   string
	Capabilities  *CapabilitySet
	ModelMetadata *unified.ResolvedModelMetadata
}

type ModelResolver func(context.Context, adapt.Request, ProviderEndpoint) (ModelResolution, bool)

func NewStaticRouter(routes ...StaticRoute) *StaticRouter {
	return &StaticRouter{routes: append([]StaticRoute(nil), routes...)}
}

func (r *StaticRouter) Route(ctx context.Context, req adapt.Request) (Route, error) {
	routes, err := r.Routes(ctx, req)
	if err != nil {
		return Route{}, err
	}
	return routes[0], nil
}

func (r *StaticRouter) Routes(ctx context.Context, req adapt.Request) ([]Route, error) {
	var skipped []string
	var candidates []resolvedStaticRoute
	hasExactRoute := r.hasExactRoute(req)
	for _, route := range r.routes {
		if hasExactRoute && route.DynamicModels {
			continue
		}
		if req.SourceAPI != "" && route.SourceAPI != "" && route.SourceAPI != req.SourceAPI {
			continue
		}
		if route.Model != "" && route.Model != req.Unified.Model {
			continue
		}
		if route.Endpoint.Client == nil {
			return nil, fmt.Errorf("route for model %q has no client", req.Unified.Model)
		}
		caps := routeCapabilities(ctx, route, req)
		modelMetadata := route.ModelMetadata
		nativeModel := route.NativeModel
		if nativeModel == "" {
			nativeModel = req.Unified.Model
		}
		if route.ModelResolver != nil {
			resolution, ok := route.ModelResolver(ctx, req, route.Endpoint)
			if !ok {
				skipped = append(skipped, fmt.Sprintf("%s/%s: model unavailable", route.Endpoint.ProviderName, route.Endpoint.APIKind))
				continue
			}
			if resolution.NativeModel != "" {
				nativeModel = resolution.NativeModel
			}
			if resolution.Capabilities != nil {
				caps = *resolution.Capabilities
			}
			if resolution.ModelMetadata != nil {
				modelMetadata = resolution.ModelMetadata
			}
		}
		if reason := capabilityMismatch(req.Unified, caps, modelMetadata); reason != "" {
			skipped = append(skipped, fmt.Sprintf("%s/%s: %s", route.Endpoint.ProviderName, route.Endpoint.APIKind, reason))
			continue
		}
		candidates = append(candidates, resolvedStaticRoute{route: route, nativeModel: nativeModel, capabilities: caps, modelMetadata: modelMetadata})
	}
	if len(candidates) > 0 {
		sort.SliceStable(candidates, func(i, j int) bool {
			return routeRank(req, candidates[i].route, candidates[j].route) > 0
		})
		routes := make([]Route, 0, len(candidates))
		for _, candidate := range candidates {
			routes = append(routes, routeFromStatic(ctx, candidate, req))
		}
		return routes, nil
	}
	if len(skipped) > 0 {
		return nil, fmt.Errorf("no route for api %q model %q satisfies required capabilities: %s", req.SourceAPI, req.Unified.Model, strings.Join(skipped, "; "))
	}
	return nil, fmt.Errorf("no route for api %q model %q", req.SourceAPI, req.Unified.Model)
}

func (r *StaticRouter) hasExactRoute(req adapt.Request) bool {
	for _, route := range r.routes {
		if req.SourceAPI != "" && route.SourceAPI != "" && route.SourceAPI != req.SourceAPI {
			continue
		}
		if route.Model == req.Unified.Model || route.NativeModel == req.Unified.Model {
			return true
		}
	}
	return false
}

type resolvedStaticRoute struct {
	route         StaticRoute
	nativeModel   string
	capabilities  CapabilitySet
	modelMetadata *unified.ResolvedModelMetadata
}

func routeCapabilities(ctx context.Context, route StaticRoute, req adapt.Request) CapabilitySet {
	if route.CapabilityResolver == nil {
		return route.Endpoint.Capabilities
	}
	return route.CapabilityResolver(ctx, req, route.Endpoint)
}

func routeFromStatic(_ context.Context, resolved resolvedStaticRoute, req adapt.Request) Route {
	route := resolved.route
	sourceAPI := req.SourceAPI
	if sourceAPI == "" {
		sourceAPI = route.SourceAPI
	}
	return Route{
		SourceAPI:            sourceAPI,
		TargetAPI:            route.Endpoint.APIKind,
		TargetFamily:         route.Endpoint.Family,
		ProviderName:         route.Endpoint.ProviderName,
		PublicModel:          req.Unified.Model,
		NativeModel:          resolved.nativeModel,
		Client:               route.Endpoint.Client,
		Capabilities:         resolved.capabilities,
		ConsumerContinuation: route.Endpoint.ConsumerContinuation,
		InternalContinuation: route.Endpoint.InternalContinuation,
		Transport:            route.Endpoint.Transport,
		ModelMetadata:        resolved.modelMetadata,
	}
}

func routeRank(req adapt.Request, candidate, current StaticRoute) int {
	if candidate.Weight != current.Weight {
		return candidate.Weight - current.Weight
	}
	if req.SourceAPI == "" {
		if candidateSourceRank, currentSourceRank := sourceAPIRank(candidate.SourceAPI), sourceAPIRank(current.SourceAPI); candidateSourceRank != currentSourceRank {
			return currentSourceRank - candidateSourceRank
		}
	}
	return candidate.Endpoint.Priority - current.Endpoint.Priority
}

func sourceAPIRank(api adapt.ApiKind) int {
	switch api {
	case adapt.ApiAnthropicMessages:
		return 0
	case adapt.ApiOpenAIResponses:
		return 1
	case adapt.ApiOpenAIChatCompletions:
		return 2
	default:
		return 10
	}
}

func capabilityMismatch(req unified.Request, caps CapabilitySet, meta *unified.ResolvedModelMetadata) string {
	if req.Stream && !caps.Streaming {
		return "streaming required"
	}
	if requiresTools(req) && !caps.Tools {
		return "tools required"
	}
	if req.ResponseFormat != nil {
		switch req.ResponseFormat.Kind {
		case unified.ResponseFormatJSON:
			if !caps.JSONMode {
				return "json mode required"
			}
		case unified.ResponseFormatJSONSchema:
			if !caps.JSONSchema {
				return "json schema required"
			}
		}
	}
	if req.Reasoning != nil && !caps.Reasoning {
		return "reasoning required"
	}
	if req.Reasoning != nil && req.Reasoning.Effort != "" && !supportsModelMetadataValue(meta, "reasoning_effort", string(req.Reasoning.Effort)) {
		return "reasoning effort unsupported"
	}
	for _, msg := range req.Messages {
		for _, part := range msg.Content {
			switch part.(type) {
			case unified.ImagePart:
				if !caps.Vision {
					return "vision required"
				}
			case unified.AudioPart:
				if !caps.AudioInput {
					return "audio input required"
				}
			}
		}
	}
	return ""
}

func supportsModelMetadataValue(meta *unified.ResolvedModelMetadata, parameter, value string) bool {
	if meta == nil {
		return true
	}
	if parameter == "reasoning_effort" && containsMetadataValue(meta.ReasoningEfforts, value) {
		return true
	}
	if mappedMetadataValue(meta, parameter, value) != "" {
		return true
	}
	values, ok := meta.ParameterValues[parameter]
	if !ok || len(values) == 0 {
		return true
	}
	if containsMetadataValue(values, value) {
		return true
	}
	return false
}

func mappedMetadataValue(meta *unified.ResolvedModelMetadata, parameter, value string) string {
	if meta == nil || len(meta.ParameterValueMappings) == 0 {
		return ""
	}
	values := meta.ParameterValueMappings[parameter]
	if len(values) == 0 {
		return ""
	}
	return values[value]
}

func containsMetadataValue(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func requiresTools(req unified.Request) bool {
	if len(req.Tools) > 0 || req.ToolChoice != nil {
		return true
	}
	for _, msg := range req.Messages {
		if len(msg.ToolCalls) > 0 || len(msg.ToolResults) > 0 {
			return true
		}
	}
	return false
}
