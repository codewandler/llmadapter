package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProvidersJSONCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"providers", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"type": "openai_responses"`) {
		t.Fatalf("unexpected output:\n%s", out.String())
	}
}

func TestRoutesCommandWithConfig(t *testing.T) {
	path := writeTestConfig(t)
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"routes", "--config", path})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"SOURCE_API", "public-fast", "openai", "gpt-test"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
}

func TestModelsCommandWithConfigAndQuery(t *testing.T) {
	path := writeTestConfig(t)
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"models", "--config", path, "--query", "public"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "public-fast") || !strings.Contains(got, "gpt-test") {
		t.Fatalf("unexpected output:\n%s", got)
	}
}

func TestResolveCommandWithConfig(t *testing.T) {
	path := writeTestConfig(t)
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"resolve", "public-fast", "--config", path})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"Matched as:   public_model", "Provider API: openai.responses", "Native model: gpt-test", "Capabilities:"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
}

func TestResolveCommandJSONWithConfig(t *testing.T) {
	path := writeTestConfig(t)
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"resolve", "gpt-test", "--config", path, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{`"matched_as": "native_model"`, `"provider_type": "openai_responses"`, `"family": "openai.responses"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
}

func writeTestConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "llmadapter.json")
	data := []byte(`{
		"providers":[{"name":"openai","type":"openai_responses","api_key":"test","model":"gpt-test"}],
		"routes":[{"source_api":"openai.responses","model":"public-fast","provider":"openai","native_model":"gpt-test","weight":100}]
	}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
