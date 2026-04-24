package router

import (
	"context"
	"fmt"

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
	Endpoint    ProviderEndpoint
}

func NewStaticRouter(routes ...StaticRoute) *StaticRouter {
	return &StaticRouter{routes: append([]StaticRoute(nil), routes...)}
}

func (r *StaticRouter) Route(ctx context.Context, req adapt.Request) (Route, error) {
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
		nativeModel := route.NativeModel
		if nativeModel == "" {
			nativeModel = req.Unified.Model
		}
		return Route{
			SourceAPI:    req.SourceAPI,
			TargetAPI:    route.Endpoint.APIKind,
			TargetFamily: route.Endpoint.Family,
			ProviderName: route.Endpoint.ProviderName,
			PublicModel:  req.Unified.Model,
			NativeModel:  nativeModel,
			Client:       route.Endpoint.Client,
			Capabilities: route.Endpoint.Capabilities,
		}, nil
	}
	return Route{}, fmt.Errorf("no route for api %q model %q", req.SourceAPI, req.Unified.Model)
}
