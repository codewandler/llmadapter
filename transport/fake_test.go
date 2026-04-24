package transport

import (
	"context"
	"errors"
	"io"
	"testing"
)

func TestFakeByteStreamTransport(t *testing.T) {
	ft := &FakeByteStreamTransport{Frames: [][]byte{[]byte("a"), []byte("b")}}
	stream, err := ft.Open(context.Background(), &Request{Method: "POST"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"a", "b"} {
		got, err := stream.Recv(context.Background())
		if err != nil || string(got) != want {
			t.Fatalf("Recv = %q,%v want %q,nil", got, err, want)
		}
	}
	if _, err := stream.Recv(context.Background()); err != io.EOF {
		t.Fatalf("err = %v, want EOF", err)
	}
	if len(ft.Seen) != 1 || ft.Seen[0].Method != "POST" {
		t.Fatalf("Seen = %+v", ft.Seen)
	}
}

func TestFakeByteStreamTransportErrors(t *testing.T) {
	want := errors.New("open")
	ft := &FakeByteStreamTransport{OpenErr: want}
	if _, err := ft.Open(context.Background(), &Request{}); !errors.Is(err, want) {
		t.Fatalf("Open err = %v, want %v", err, want)
	}

	want = errors.New("frame")
	ft = &FakeByteStreamTransport{Frames: [][]byte{[]byte("a")}, Err: want, ErrAtFrame: 0}
	stream, err := ft.Open(context.Background(), &Request{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Recv(context.Background()); !errors.Is(err, want) {
		t.Fatalf("Recv err = %v, want %v", err, want)
	}
}
