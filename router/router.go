package router

import (
	"context"
	"fmt"
	"strings"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/unified"
)

type Route struct {
	SourceAPI    adapt.ApiKind
	TargetAPI    adapt.ApiKind
	TargetFamily adapt.ApiFamily
	ProviderName string
	PublicModel  string
	NativeModel  string
	Client       unified.Client
	Capabilities CapabilitySet
}

type Router interface {
	Route(ctx context.Context, req adapt.Request) (Route, error)
}

type ProviderEndpoint struct {
	ProviderName string
	APIKind      adapt.ApiKind
	Family       adapt.ApiFamily
	Client       unified.Client
	Capabilities CapabilitySet
	Priority     int
	Tags         map[string]string
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
	SourceAPI   adapt.ApiKind
	Model       string
	NativeModel string
	Weight      int
	Endpoint    ProviderEndpoint
}

func NewStaticRouter(routes ...StaticRoute) *StaticRouter {
	return &StaticRouter{routes: append([]StaticRoute(nil), routes...)}
}

func (r *StaticRouter) Route(ctx context.Context, req adapt.Request) (Route, error) {
	var skipped []string
	var selected StaticRoute
	selectedSet := false
	for _, route := range r.routes {
		if route.SourceAPI != "" && route.SourceAPI != req.SourceAPI {
			continue
		}
		if route.Model != "" && route.Model != req.Unified.Model {
			continue
		}
		if route.Endpoint.Client == nil {
			return Route{}, fmt.Errorf("route for model %q has no client", req.Unified.Model)
		}
		if reason := capabilityMismatch(req.Unified, route.Endpoint.Capabilities); reason != "" {
			skipped = append(skipped, fmt.Sprintf("%s/%s: %s", route.Endpoint.ProviderName, route.Endpoint.APIKind, reason))
			continue
		}
		if !selectedSet || routeRank(route, selected) > 0 {
			selected = route
			selectedSet = true
		}
	}
	if selectedSet {
		nativeModel := selected.NativeModel
		if nativeModel == "" {
			nativeModel = req.Unified.Model
		}
		return Route{
			SourceAPI:    req.SourceAPI,
			TargetAPI:    selected.Endpoint.APIKind,
			TargetFamily: selected.Endpoint.Family,
			ProviderName: selected.Endpoint.ProviderName,
			PublicModel:  req.Unified.Model,
			NativeModel:  nativeModel,
			Client:       selected.Endpoint.Client,
			Capabilities: selected.Endpoint.Capabilities,
		}, nil
	}
	if len(skipped) > 0 {
		return Route{}, fmt.Errorf("no route for api %q model %q satisfies required capabilities: %s", req.SourceAPI, req.Unified.Model, strings.Join(skipped, "; "))
	}
	return Route{}, fmt.Errorf("no route for api %q model %q", req.SourceAPI, req.Unified.Model)
}

func routeRank(candidate, current StaticRoute) int {
	if candidate.Weight != current.Weight {
		return candidate.Weight - current.Weight
	}
	return candidate.Endpoint.Priority - current.Endpoint.Priority
}

func capabilityMismatch(req unified.Request, caps CapabilitySet) string {
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
