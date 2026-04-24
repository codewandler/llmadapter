package adapt

import "testing"

func TestUnsupportedFieldError(t *testing.T) {
	var err error = &UnsupportedFieldError{APIKind: ApiAnthropicMessages, Field: "seed", Reason: "not supported"}
	if err.Error() == "" {
		t.Fatalf("Error returned empty string")
	}
}
