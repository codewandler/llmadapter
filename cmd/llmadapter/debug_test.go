package main

import "testing"

func TestNormalizeInferArgsAcceptsSeparatedDebugScopes(t *testing.T) {
	params := inferParams{debugScopes: []string{"all"}}
	args, err := normalizeInferArgs(&params, []string{"request,response", "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 1 || args[0] != "hello" {
		t.Fatalf("unexpected args: %+v", args)
	}
	if len(params.debugScopes) != 1 || params.debugScopes[0] != "request,response" {
		t.Fatalf("debug scopes not rewritten: %+v", params.debugScopes)
	}
}
