package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestProxyCommandRegistered(t *testing.T) {
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"proxy", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"--bind", "--upstream", "--analyze"} {
		if !strings.Contains(got, want) {
			t.Fatalf("proxy help missing %q:\n%s", want, got)
		}
	}
}

func TestClaudeProxyEnvOverridesBaseURLs(t *testing.T) {
	got := claudeProxyEnv([]string{
		"KEEP=value",
		"ANTHROPIC_BASE_URL=https://old.example",
		"ANTHROPIC_API_URL=https://old.example",
		"CLAUDE_CODE_API_BASE_URL=https://old.example",
	}, "http://127.0.0.1:1234")

	joined := "\n" + strings.Join(got, "\n") + "\n"
	for _, want := range []string{
		"\nKEEP=value\n",
		"\nANTHROPIC_BASE_URL=http://127.0.0.1:1234\n",
		"\nANTHROPIC_API_URL=http://127.0.0.1:1234\n",
		"\nCLAUDE_CODE_API_BASE_URL=http://127.0.0.1:1234\n",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("env missing %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "https://old.example") {
		t.Fatalf("old base URL was not replaced:\n%s", joined)
	}
}

func TestProxyRedactsSensitiveHeadersAndJSON(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":"hi"}],"api_key":"secret-value","metadata":{"user_id":"device-value"},"nested":{"access_token":"access-value"}}`)
	got := formatProxyBody(body, "application/json")
	for _, forbidden := range []string{"secret-value", "access-value", "device-value"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("sensitive JSON value leaked in:\n%s", got)
		}
	}
	for _, want := range []string{`"api_key": "[redacted]"`, `"access_token": "[redacted]"`, `"user_id": "[redacted]"`, `"content": "hi"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatted JSON missing %q:\n%s", want, got)
		}
	}
}

func TestServeProxyForwardsAndLogsHeadersAndBodies(t *testing.T) {
	var upstreamPath, upstreamBody, upstreamAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.String()
		upstreamAuth = r.Header.Get("Authorization")
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read upstream body: %v", err)
		}
		upstreamBody = string(data)
		w.Header().Set("X-Upstream", "seen")
		w.Header().Set("Anthropic-Organization-Id", "org-secret")
		w.Header().Set("Set-Cookie", "session=secret")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var logs bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- serveProxy(ctx, ln, upstreamURL, &logs, proxyLogOptions{MaxBodyBytes: defaultProxyBodyBytes})
	}()

	req, err := http.NewRequest(http.MethodPost, "http://"+ln.Addr().String()+"/v1/messages?beta=true", strings.NewReader(`{"prompt":"hello"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("proxy did not shut down")
	}

	if resp.StatusCode != http.StatusOK || !strings.Contains(string(respBody), `"ok"`) {
		t.Fatalf("unexpected response status/body: %s %s", resp.Status, string(respBody))
	}
	if upstreamPath != "/v1/messages?beta=true" || upstreamBody != `{"prompt":"hello"}` || upstreamAuth != "Bearer secret" {
		t.Fatalf("unexpected upstream request: path=%q auth=%q body=%q", upstreamPath, upstreamAuth, upstreamBody)
	}
	got := logs.String()
	for _, want := range []string{
		">>> request POST",
		"<<< response 200 OK",
		"Authorization: [redacted]",
		"Anthropic-Organization-Id: [redacted]",
		"Set-Cookie: [redacted]",
		`"prompt": "hello"`,
		`"ok": "true"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("proxy logs missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Bearer secret") || strings.Contains(got, "session=secret") || strings.Contains(got, "org-secret") {
		t.Fatalf("proxy logs leaked secret:\n%s", got)
	}
}

func TestServeProxyLogsSSELines(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.(http.Flusher).Flush()
		fmtLine := func(line string) {
			_, _ = w.Write([]byte(line))
			w.(http.Flusher).Flush()
		}
		fmtLine("event: content_block_delta\n")
		fmtLine(`data: {"type":"content_block_delta","delta":{"text":"hi"}}` + "\n\n")
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var logs bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- serveProxy(ctx, ln, upstreamURL, &logs, proxyLogOptions{MaxBodyBytes: defaultProxyBodyBytes})
	}()

	resp, err := http.Get("http://" + ln.Addr().String() + "/stream")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(resp.Body); err != nil {
		t.Fatal(err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("proxy did not shut down")
	}
	got := logs.String()
	for _, want := range []string{
		"<<< response body stream: event: content_block_delta",
		`<<< response body stream: data: {"type":"content_block_delta","delta":{"text":"hi"}}`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("proxy logs missing %q:\n%s", want, got)
		}
	}
}
