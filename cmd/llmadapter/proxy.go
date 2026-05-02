package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/spf13/cobra"
)

const (
	defaultProxyBind       = "127.0.0.1:0"
	defaultProxyBodyBytes  = 256 * 1024
	defaultClaudeUpstream  = "https://api.anthropic.com"
	defaultClaudeCLI       = "claude"
	proxyStreamLineMaxSize = 256 * 1024
)

type proxyParams struct {
	bind         string
	upstream     string
	analyze      string
	command      string
	maxBodyBytes int64
}

func newProxyCommand() *cobra.Command {
	params := proxyParams{
		bind:         defaultProxyBind,
		command:      defaultClaudeCLI,
		maxBodyBytes: defaultProxyBodyBytes,
	}
	cmd := &cobra.Command{
		Use:   "proxy",
		Short: "Inspect proxied provider HTTP headers and stream messages",
		Long:  "Run a small reverse proxy that logs redacted request/response headers and streamed provider messages. In analyze mode it starts the proxy and runs a provider CLI through it.",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if params.analyze != "" {
				return runProxyAnalyze(cmd.Context(), cmd.ErrOrStderr(), params, args)
			}
			if len(args) > 0 {
				return fmt.Errorf("unexpected arguments %q; use --analyze claude -- <claude args> for CLI passthrough", strings.Join(args, " "))
			}
			if params.upstream == "" {
				return fmt.Errorf("--upstream is required unless --analyze claude is used")
			}
			return runProxyServe(cmd.Context(), cmd.ErrOrStderr(), params)
		},
	}
	cmd.Flags().StringVar(&params.bind, "bind", params.bind, "local bind address")
	cmd.Flags().StringVar(&params.upstream, "upstream", "", "upstream base URL, for example https://api.anthropic.com")
	cmd.Flags().StringVar(&params.analyze, "analyze", "", "run a provider CLI through the proxy; supported value: claude")
	cmd.Flags().StringVar(&params.command, "command", params.command, "CLI executable for --analyze claude")
	cmd.Flags().Int64Var(&params.maxBodyBytes, "max-body-bytes", params.maxBodyBytes, "maximum request/response body bytes to buffer for pretty logging")
	return cmd
}

func runProxyServe(ctx context.Context, logw io.Writer, params proxyParams) error {
	upstream, err := parseProxyUpstream(params.upstream)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", params.bind)
	if err != nil {
		return err
	}
	fmt.Fprintf(logw, "llmadapter proxy listening on http://%s -> %s\n", ln.Addr().String(), upstream.String())
	return serveProxy(ctx, ln, upstream, logw, proxyLogOptions{MaxBodyBytes: params.maxBodyBytes})
}

func runProxyAnalyze(ctx context.Context, logw io.Writer, params proxyParams, args []string) error {
	if params.analyze != "claude" {
		return fmt.Errorf("unsupported --analyze value %q; supported value: claude", params.analyze)
	}
	upstream := params.upstream
	if upstream == "" {
		upstream = defaultClaudeUpstream
	}
	parsedUpstream, err := parseProxyUpstream(upstream)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", params.bind)
	if err != nil {
		return err
	}
	proxyURL := "http://" + ln.Addr().String()
	fmt.Fprintf(logw, "llmadapter proxy analyzing claude on %s -> %s\n", proxyURL, parsedUpstream.String())

	serverErr := make(chan error, 1)
	serverCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		serverErr <- serveProxy(serverCtx, ln, parsedUpstream, logw, proxyLogOptions{MaxBodyBytes: params.maxBodyBytes})
	}()

	child := exec.CommandContext(ctx, params.command, args...)
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	child.Env = claudeProxyEnv(os.Environ(), proxyURL)
	err = child.Run()
	cancel()
	select {
	case serverRunErr := <-serverErr:
		if serverRunErr != nil && err == nil {
			err = serverRunErr
		}
	case <-time.After(5 * time.Second):
		if err == nil {
			err = fmt.Errorf("proxy shutdown timed out")
		}
	}
	return err
}

func claudeProxyEnv(env []string, proxyURL string) []string {
	return appendWithoutEnvKeys(env, map[string]string{
		"ANTHROPIC_BASE_URL":       proxyURL,
		"ANTHROPIC_API_URL":        proxyURL,
		"CLAUDE_CODE_API_BASE_URL": proxyURL,
	})
}

func appendWithoutEnvKeys(env []string, values map[string]string) []string {
	out := make([]string, 0, len(env)+len(values))
	for _, entry := range env {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			out = append(out, entry)
			continue
		}
		if _, exists := values[key]; exists {
			continue
		}
		out = append(out, entry)
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		out = append(out, key+"="+values[key])
	}
	return out
}

func parseProxyUpstream(raw string) (*url.URL, error) {
	if raw == "" {
		return nil, fmt.Errorf("upstream URL is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse upstream URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("upstream URL must use http or https")
	}
	if u.Host == "" {
		return nil, fmt.Errorf("upstream URL must include a host")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	return u, nil
}

type proxyLogOptions struct {
	MaxBodyBytes int64
}

func serveProxy(ctx context.Context, ln net.Listener, upstream *url.URL, logw io.Writer, opts proxyLogOptions) error {
	if opts.MaxBodyBytes <= 0 {
		opts.MaxBodyBytes = defaultProxyBodyBytes
	}
	logger := newProxyLogger(logw, opts)
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			requestID := logger.nextRequestID()
			pr.Out = pr.Out.WithContext(context.WithValue(pr.Out.Context(), proxyRequestIDKey{}, requestID))
			pr.SetURL(upstream)
			pr.Out.Host = upstream.Host
			pr.Out.Header.Del("Accept-Encoding")
			if pr.Out.Body != nil {
				pr.Out.Body = newProxyBodyLogger(pr.Out.Body, logger, requestID, ">>> request body", pr.Out.Header.Get("Content-Type"))
			}
			logger.logRequest(requestID, pr.Out)
		},
		ModifyResponse: func(resp *http.Response) error {
			requestID := requestIDFromContext(resp.Request.Context())
			logger.logResponse(requestID, resp)
			if resp.Body != nil {
				resp.Body = newProxyBodyLogger(resp.Body, logger, requestID, "<<< response body", resp.Header.Get("Content-Type"))
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			requestID := requestIDFromContext(r.Context())
			if requestID == 0 {
				requestID = logger.nextRequestID()
			}
			logger.logf(requestID, "!!! proxy error: %v\n", err)
			http.Error(w, "proxy error", http.StatusBadGateway)
		},
		ErrorLog: log.New(io.Discard, "", 0),
	}

	server := &http.Server{Handler: proxy}
	done := make(chan error, 1)
	go func() {
		done <- server.Serve(ln)
	}()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	err := <-done
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

type proxyRequestIDKey struct{}

func requestIDFromContext(ctx context.Context) uint64 {
	id, _ := ctx.Value(proxyRequestIDKey{}).(uint64)
	return id
}

type proxyLogger struct {
	w    io.Writer
	opts proxyLogOptions
	seq  atomic.Uint64
}

func newProxyLogger(w io.Writer, opts proxyLogOptions) *proxyLogger {
	return &proxyLogger{w: w, opts: opts}
}

func (l *proxyLogger) nextRequestID() uint64 {
	return l.seq.Add(1)
}

func (l *proxyLogger) logRequest(id uint64, req *http.Request) {
	l.logf(id, ">>> request %s %s\n", req.Method, req.URL.String())
	writeProxyHeaders(l.w, id, req.Header)
}

func (l *proxyLogger) logResponse(id uint64, resp *http.Response) {
	l.logf(id, "<<< response %s\n", resp.Status)
	writeProxyHeaders(l.w, id, resp.Header)
}

func (l *proxyLogger) logBody(id uint64, label string, contentType string, data []byte, truncated bool) {
	if len(data) == 0 && !truncated {
		return
	}
	body := formatProxyBody(data, contentType)
	if truncated {
		body += "\n... truncated"
	}
	l.logf(id, "%s:\n%s\n", label, indentProxyBlock(body))
}

func (l *proxyLogger) logStreamLine(id uint64, label string, line []byte) {
	if len(bytes.TrimSpace(line)) == 0 {
		return
	}
	l.logf(id, "%s: %s\n", label, redactProxyText(string(line)))
}

func (l *proxyLogger) logf(id uint64, format string, args ...any) {
	ts := time.Now().Format(time.RFC3339Nano)
	prefix := fmt.Sprintf("[%s #%d] ", ts, id)
	fmt.Fprintf(l.w, prefix+format, args...)
}

func writeProxyHeaders(w io.Writer, id uint64, headers http.Header) {
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		values := headers.Values(key)
		if isSensitiveProxyKey(key) {
			values = []string{"[redacted]"}
		}
		fmt.Fprintf(w, "  #%d %s: %s\n", id, key, strings.Join(values, ", "))
	}
}

type proxyBodyLogger struct {
	rc          io.ReadCloser
	logger      *proxyLogger
	requestID   uint64
	label       string
	contentType string
	limit       int64
	buffer      bytes.Buffer
	lineBuffer  bytes.Buffer
	truncated   bool
	done        bool
	isStream    bool
}

func newProxyBodyLogger(rc io.ReadCloser, logger *proxyLogger, requestID uint64, label string, contentType string) io.ReadCloser {
	return &proxyBodyLogger{
		rc:          rc,
		logger:      logger,
		requestID:   requestID,
		label:       label,
		contentType: contentType,
		limit:       logger.opts.MaxBodyBytes,
		isStream:    isProxyStreamContent(contentType),
	}
}

func (l *proxyBodyLogger) Read(p []byte) (int, error) {
	n, err := l.rc.Read(p)
	if n > 0 {
		l.observe(p[:n])
	}
	if err == io.EOF {
		l.finish()
	}
	return n, err
}

func (l *proxyBodyLogger) Close() error {
	l.finish()
	return l.rc.Close()
}

func (l *proxyBodyLogger) observe(chunk []byte) {
	if l.isStream {
		l.observeStream(chunk)
		return
	}
	if l.truncated {
		return
	}
	remaining := l.limit - int64(l.buffer.Len())
	if remaining <= 0 {
		l.truncated = true
		return
	}
	if int64(len(chunk)) > remaining {
		l.buffer.Write(chunk[:remaining])
		l.truncated = true
		return
	}
	l.buffer.Write(chunk)
}

func (l *proxyBodyLogger) observeStream(chunk []byte) {
	l.lineBuffer.Write(chunk)
	for {
		line, err := l.lineBuffer.ReadBytes('\n')
		if err != nil {
			if len(line) > 0 {
				l.lineBuffer.Reset()
				l.lineBuffer.Write(line)
			}
			if l.lineBuffer.Len() > proxyStreamLineMaxSize {
				l.logger.logStreamLine(l.requestID, l.label+" stream", l.lineBuffer.Bytes()[:proxyStreamLineMaxSize])
				l.lineBuffer.Reset()
			}
			return
		}
		l.logger.logStreamLine(l.requestID, l.label+" stream", bytes.TrimRight(line, "\r\n"))
	}
}

func (l *proxyBodyLogger) finish() {
	if l.done {
		return
	}
	l.done = true
	if l.isStream {
		if l.lineBuffer.Len() > 0 {
			l.logger.logStreamLine(l.requestID, l.label+" stream", l.lineBuffer.Bytes())
		}
		return
	}
	l.logger.logBody(l.requestID, l.label, l.contentType, l.buffer.Bytes(), l.truncated)
}

func isProxyStreamContent(contentType string) bool {
	contentType = strings.ToLower(contentType)
	return strings.Contains(contentType, "text/event-stream") || strings.Contains(contentType, "application/x-ndjson")
}

func formatProxyBody(data []byte, contentType string) string {
	if len(data) == 0 {
		return ""
	}
	contentType = strings.ToLower(contentType)
	if strings.Contains(contentType, "json") || json.Valid(data) {
		var v any
		if err := json.Unmarshal(data, &v); err == nil {
			redacted := redactProxyJSON(v)
			pretty, err := json.MarshalIndent(redacted, "", "  ")
			if err == nil {
				return string(pretty)
			}
		}
	}
	return redactProxyText(string(data))
}

func redactProxyJSON(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			if isSensitiveProxyKey(key) {
				out[key] = "[redacted]"
				continue
			}
			out[key] = redactProxyJSON(value)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, value := range typed {
			out[i] = redactProxyJSON(value)
		}
		return out
	default:
		return v
	}
}

func redactProxyText(text string) string {
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 0, 64*1024), proxyStreamLineMaxSize)
	var out strings.Builder
	first := true
	for scanner.Scan() {
		line := scanner.Text()
		if !first {
			out.WriteByte('\n')
		}
		first = false
		out.WriteString(redactProxyHeaderLikeLine(line))
	}
	if err := scanner.Err(); err != nil {
		return text
	}
	if first {
		return text
	}
	return out.String()
}

func redactProxyHeaderLikeLine(line string) string {
	name, value, ok := strings.Cut(line, ":")
	if !ok || !isSensitiveProxyKey(name) {
		return line
	}
	if strings.TrimSpace(value) == "" {
		return name + ":"
	}
	return name + ": [redacted]"
}

func isSensitiveProxyKey(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	if k == "" {
		return false
	}
	normalized := strings.NewReplacer("-", "", "_", "", " ", "").Replace(k)
	for _, part := range []string{
		"authorization",
		"api-key",
		"apikey",
		"access-token",
		"accesstoken",
		"refresh-token",
		"refreshtoken",
		"id-token",
		"idtoken",
		"user-id",
		"userid",
		"organization",
		"organization-id",
		"organizationid",
		"project",
		"project-id",
		"projectid",
		"cookie",
		"secret",
		"password",
		"session",
		"account",
	} {
		if strings.Contains(k, part) || strings.Contains(normalized, part) {
			return true
		}
	}
	return false
}

func indentProxyBlock(text string) string {
	if text == "" {
		return "  "
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n")
}
