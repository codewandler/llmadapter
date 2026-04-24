package messages

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLocalTokenStoreLoadAndSavePreservesCredentialFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".credentials.json")
	if err := os.WriteFile(path, []byte(`{
		"other":"keep",
		"claudeAiOauth":{
			"accessToken":"old-access",
			"refreshToken":"old-refresh",
			"expiresAt":1893456000000,
			"scope":"keep"
		}
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := NewLocalTokenStoreWithPath(path)
	if err != nil {
		t.Fatal(err)
	}
	token, err := store.Load(context.Background(), claudeLocalTokenKey)
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "old-access" || token.RefreshToken != "old-refresh" || token.ExpiresAt.Year() != 2030 {
		t.Fatalf("unexpected token: %+v", token)
	}

	next := &Token{
		AccessToken:  "new-access",
		RefreshToken: "new-refresh",
		ExpiresAt:    time.Unix(2000000000, 0),
	}
	if err := store.Save(context.Background(), claudeLocalTokenKey, next); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	if root["other"] != "keep" {
		t.Fatalf("top-level credential data was not preserved: %+v", root)
	}
	oauth, ok := root["claudeAiOauth"].(map[string]any)
	if !ok {
		t.Fatalf("missing oauth data: %+v", root)
	}
	if oauth["accessToken"] != "new-access" || oauth["refreshToken"] != "new-refresh" || oauth["scope"] != "keep" {
		t.Fatalf("unexpected oauth data: %+v", oauth)
	}
}
