package adapt

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/codewandler/llmadapter/unified"
)

type HTTPRequestInfo struct {
	Method  string      `json:"method,omitempty"`
	Path    string      `json:"path,omitempty"`
	Query   url.Values  `json:"query,omitempty"`
	Headers http.Header `json:"headers,omitempty"`
	Remote  string      `json:"remote,omitempty"`
}

type Request struct {
	SourceAPI   ApiKind                    `json:"source_api,omitempty"`
	HTTP        *HTTPRequestInfo           `json:"http,omitempty"`
	RawBody     []byte                     `json:"raw_body,omitempty"`
	Raw         any                        `json:"raw,omitempty"`
	Unified     unified.Request            `json:"unified"`
	MappingMode MappingMode                `json:"mapping_mode,omitempty"`
	Warnings    []Warning                  `json:"warnings,omitempty"`
	Extensions  map[string]json.RawMessage `json:"extensions,omitempty"`
}
