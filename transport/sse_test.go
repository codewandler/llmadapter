package transport

import (
	"io"
	"strings"
	"testing"
)

func TestSSEReaderAndParseFrame(t *testing.T) {
	r := NewSSEReader(strings.NewReader(":ignore\nid: 1\nevent: message_start\ndata: {\"a\":1}\ndata: {\"b\":2}\nretry: 10\n\n"))
	raw, err := r.Next()
	if err != nil {
		t.Fatal(err)
	}
	frame, err := ParseSSEFrame(raw)
	if err != nil {
		t.Fatal(err)
	}
	if frame.ID != "1" || frame.Event != "message_start" || frame.Retry != 10 || string(frame.Data) != "{\"a\":1}\n{\"b\":2}" {
		t.Fatalf("unexpected frame: %+v data=%q", frame, frame.Data)
	}
	if _, err := r.Next(); err != io.EOF {
		t.Fatalf("Next err = %v, want EOF", err)
	}
}

func TestSSEReaderMixedLineEndings(t *testing.T) {
	r := NewSSEReader(strings.NewReader("event: a\rdata: 1\r\n\r\nevent: b\ndata: 2\n\n"))
	raw, err := r.Next()
	if err != nil {
		t.Fatal(err)
	}
	frame, _ := ParseSSEFrame(raw)
	if frame.Event != "a" || string(frame.Data) != "1" {
		t.Fatalf("unexpected first frame: %+v", frame)
	}
	raw, err = r.Next()
	if err != nil {
		t.Fatal(err)
	}
	frame, _ = ParseSSEFrame(raw)
	if frame.Event != "b" || string(frame.Data) != "2" {
		t.Fatalf("unexpected second frame: %+v", frame)
	}
}
