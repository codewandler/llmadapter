package unified

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestExtensionsRoundtrip(t *testing.T) {
	var e Extensions
	if err := e.Set("string", "value"); err != nil {
		t.Fatal(err)
	}
	got, ok, err := GetExtension[string](e, "string")
	if err != nil || !ok || got != "value" {
		t.Fatalf("got (%q,%v,%v), want value,true,nil", got, ok, err)
	}
}

func TestExtensionsMissingAndKeys(t *testing.T) {
	var e Extensions
	_ = e.Set("b", 1)
	_ = e.Set("a", nil)
	if _, ok, err := GetExtension[string](e, "missing"); err != nil || ok {
		t.Fatalf("missing = ok %v err %v, want false nil", ok, err)
	}
	if !e.Has("a") || e.Has("missing") {
		t.Fatalf("Has returned unexpected result")
	}
	if got := e.Keys(); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("Keys = %v", got)
	}
}

func TestExtensionsTypeMismatch(t *testing.T) {
	var e Extensions
	_ = e.Set("value", "abc")
	if _, ok, err := GetExtension[int](e, "value"); !ok || err == nil {
		t.Fatalf("expected present key with unmarshal error")
	}
}

func TestExtensionsRawRoundtrip(t *testing.T) {
	var e Extensions
	if err := e.SetRaw("raw", json.RawMessage(`{"a":1}`)); err != nil {
		t.Fatal(err)
	}
	if got := string(e.Raw("raw")); got != `{"a":1}` {
		t.Fatalf("Raw = %s", got)
	}
	if err := e.SetRaw("bad", json.RawMessage(`{`)); err == nil {
		t.Fatalf("expected invalid raw JSON error")
	}
}

func TestOpenRouterRawExtensionsRoundtrip(t *testing.T) {
	var e Extensions
	err := SetOpenRouterRawExtensions(&e, OpenRouterRawExtensions{
		Provider:  json.RawMessage(`{"order":["openai"]}`),
		Debug:     json.RawMessage(`true`),
		SessionID: json.RawMessage(`"sess_1"`),
	})
	if err != nil {
		t.Fatal(err)
	}
	raw := OpenRouterRawExtensionsFrom(e)
	if string(raw.Provider) != `{"order":["openai"]}` || string(raw.Debug) != `true` || string(raw.SessionID) != `"sess_1"` {
		t.Fatalf("unexpected OpenRouter extensions: %+v", raw)
	}
}
