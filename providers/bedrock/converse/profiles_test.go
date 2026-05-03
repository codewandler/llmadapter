package converse

import "testing"

func TestComputeRegionPrefix(t *testing.T) {
	tests := map[string]string{
		"us-east-1":      PrefixUS,
		"eu-central-1":   PrefixEU,
		"ap-southeast-2": PrefixAPAC,
		"ca-central-1":   PrefixGlobal,
		"":               PrefixGlobal,
	}
	for region, want := range tests {
		t.Run(region, func(t *testing.T) {
			if got := computeRegionPrefix(region); got != want {
				t.Fatalf("computeRegionPrefix(%q) = %q, want %q", region, got, want)
			}
		})
	}
}

func TestResolveModelAppliesRegionalInferenceProfilePrefix(t *testing.T) {
	client, err := NewClient(WithRegion("us-east-1"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.resolveModel(ModelClaudeSonnet46)
	if err != nil {
		t.Fatal(err)
	}
	if want := PrefixUS + "." + ModelClaudeSonnet46; got != want {
		t.Fatalf("resolved model = %q, want %q", got, want)
	}
}

func TestResolveModelPreservesExplicitInferenceProfilePrefix(t *testing.T) {
	client, err := NewClient(WithRegion("eu-central-1"))
	if err != nil {
		t.Fatal(err)
	}
	prefixed := PrefixUS + "." + ModelClaudeSonnet46
	got, err := client.resolveModel(prefixed)
	if err != nil {
		t.Fatal(err)
	}
	if got != prefixed {
		t.Fatalf("resolved model = %q, want passthrough %q", got, prefixed)
	}
}

func TestResolveModelFallsBackToGlobalProfile(t *testing.T) {
	client, err := NewClient(WithRegion("ca-central-1"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.resolveModel(ModelClaudeSonnet46)
	if err != nil {
		t.Fatal(err)
	}
	if want := PrefixGlobal + "." + ModelClaudeSonnet46; got != want {
		t.Fatalf("resolved model = %q, want %q", got, want)
	}
}

func TestStripInferenceProfilePrefix(t *testing.T) {
	got := stripInferenceProfilePrefix(PrefixEU + "." + ModelClaudeOpus47)
	if got != ModelClaudeOpus47 {
		t.Fatalf("stripped model = %q, want %q", got, ModelClaudeOpus47)
	}
	if got := stripInferenceProfilePrefix("anthropic.other"); got != "anthropic.other" {
		t.Fatalf("unexpected strip for unprefixed model: %q", got)
	}
}
