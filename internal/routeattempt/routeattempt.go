package routeattempt

import (
	"context"
	"errors"
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

func Retryable(err error) bool {
	var unsupported *adapt.UnsupportedFieldError
	if errors.As(err, &unsupported) {
		return false
	}
	var apiErr *unified.APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode != 400 && apiErr.StatusCode != 422
	}
	return true
}

func ReachedMaxAttempts(attempts, maxAttempts int) bool {
	return maxAttempts > 0 && attempts >= maxAttempts
}
