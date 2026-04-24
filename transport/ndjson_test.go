package transport

import (
	"io"
	"strings"
	"testing"
)

func TestNDJSONReader(t *testing.T) {
	r := NewNDJSONReader(strings.NewReader("\n {\"a\":1} \n{\"b\":2}\n"))
	line, err := r.Next()
	if err != nil || string(line) != `{"a":1}` {
		t.Fatalf("line = %q err = %v", line, err)
	}
	line, err = r.Next()
	if err != nil || string(line) != `{"b":2}` {
		t.Fatalf("line = %q err = %v", line, err)
	}
	if _, err := r.Next(); err != io.EOF {
		t.Fatalf("err = %v, want EOF", err)
	}
}
