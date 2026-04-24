package unified

import (
	"errors"
	"fmt"
	"testing"
)

func TestAPIError(t *testing.T) {
	err := &APIError{StatusCode: 429, Type: "rate_limit", Code: "too_many", Message: "slow down"}
	if err.Error() == "" {
		t.Fatalf("Error returned empty string")
	}
	var target *APIError
	if !errors.As(fmt.Errorf("wrapped: %w", err), &target) {
		t.Fatalf("errors.As did not extract APIError")
	}
}
