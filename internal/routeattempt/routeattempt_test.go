package routeattempt

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/router"
	"github.com/codewandler/llmadapter/unified"
)

type singleRouter struct {
	route router.Route
	err   error
}

func (r singleRouter) Route(context.Context, adapt.Request) (router.Route, error) {
	return r.route, r.err
}

type candidateRouter struct {
	singleRouter
	routes []router.Route
}

func (r candidateRouter) Routes(context.Context, adapt.Request) ([]router.Route, error) {
	return r.routes, r.err
}

func TestCandidatesUsesCandidateRouterWhenAvailable(t *testing.T) {
	routes := []router.Route{
		{ProviderName: "primary"},
		{ProviderName: "fallback"},
	}
	got, err := Candidates(context.Background(), candidateRouter{routes: routes}, adapt.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ProviderName != "primary" || got[1].ProviderName != "fallback" {
		t.Fatalf("unexpected routes: %+v", got)
	}
}

func TestCandidatesWrapsSingleRouteRouter(t *testing.T) {
	got, err := Candidates(context.Background(), singleRouter{route: router.Route{ProviderName: "only"}}, adapt.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ProviderName != "only" {
		t.Fatalf("unexpected routes: %+v", got)
	}
}

func TestRequestForRouteRewritesNativeModelWithoutMutatingOriginal(t *testing.T) {
	req := unified.Request{Model: "public"}
	got := RequestForRoute(req, router.Route{NativeModel: "native"})
	if got.Model != "native" {
		t.Fatalf("model = %q, want native", got.Model)
	}
	if req.Model != "public" {
		t.Fatalf("original request mutated: %+v", req)
	}
}

func TestErrorIncludesProviderAndAPI(t *testing.T) {
	err := Error(router.Route{ProviderName: "primary", TargetAPI: adapt.ApiOpenAIResponses}, errors.New("down"))
	if !strings.Contains(err.Error(), "provider primary/openai.responses failed: down") {
		t.Fatalf("unexpected error: %v", err)
	}
}
