package router

import (
	"context"
	"fmt"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/unified"
)

type Route struct {
	Client      unified.Client
	NativeModel string
}

type Router interface {
	Route(ctx context.Context, req adapt.Request) (Route, error)
}

type StaticRouter struct {
	routes []StaticRoute
}

type StaticRoute struct {
	SourceAPI   adapt.ApiKind
	Model       string
	NativeModel string
	Client      unified.Client
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
		if route.Client == nil {
			return Route{}, fmt.Errorf("route for model %q has no client", req.Unified.Model)
		}
		nativeModel := route.NativeModel
		if nativeModel == "" {
			nativeModel = req.Unified.Model
		}
		return Route{Client: route.Client, NativeModel: nativeModel}, nil
	}
	return Route{}, fmt.Errorf("no route for api %q model %q", req.SourceAPI, req.Unified.Model)
}
