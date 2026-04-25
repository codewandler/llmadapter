package routeattempt

import (
	"context"
	"fmt"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/router"
	"github.com/codewandler/llmadapter/unified"
)

func Candidates(ctx context.Context, r router.Router, req adapt.Request) ([]router.Route, error) {
	if candidateRouter, ok := r.(router.CandidateRouter); ok {
		return candidateRouter.Routes(ctx, req)
	}
	route, err := r.Route(ctx, req)
	if err != nil {
		return nil, err
	}
	return []router.Route{route}, nil
}

func RequestForRoute(req unified.Request, route router.Route) unified.Request {
	attempt := req
	if route.NativeModel != "" {
		attempt.Model = route.NativeModel
	}
	return attempt
}

func Error(route router.Route, err error) error {
	return fmt.Errorf("provider %s/%s failed: %w", route.ProviderName, route.TargetAPI, err)
}
