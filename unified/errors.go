package unified

import (
	"encoding/json"
	"fmt"
	"time"
)

type APIError struct {
	StatusCode  int             `json:"status_code,omitempty"`
	Type        string          `json:"type,omitempty"`
	Code        string          `json:"code,omitempty"`
	Message     string          `json:"message,omitempty"`
	Param       string          `json:"param,omitempty"`
	RetryAfter  time.Duration   `json:"retry_after,omitempty"`
	ProviderRaw json.RawMessage `json:"provider_raw,omitempty"`
}

func (e *APIError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.StatusCode != 0 && e.Message != "" {
		return fmt.Sprintf("llmadapter API error: status=%d type=%s code=%s message=%s", e.StatusCode, e.Type, e.Code, e.Message)
	}
	if e.Message != "" {
		return fmt.Sprintf("llmadapter API error: type=%s code=%s message=%s", e.Type, e.Code, e.Message)
	}
	if e.StatusCode != 0 {
		return fmt.Sprintf("llmadapter API error: status=%d", e.StatusCode)
	}
	return "llmadapter API error"
}
