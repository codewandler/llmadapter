package transport

import (
	"context"
	"io"
	"sync"
)

type FakeByteStreamTransport struct {
	Frames [][]byte
	Err    error
	// ErrAtFrame controls when Err is returned. Use -1 to return Err after all frames.
	ErrAtFrame int
	OpenErr    error

	mu   sync.Mutex
	Seen []*Request
}

func (t *FakeByteStreamTransport) Open(ctx context.Context, req *Request) (ByteStream, error) {
	t.mu.Lock()
	t.Seen = append(t.Seen, req)
	t.mu.Unlock()
	if t.OpenErr != nil {
		return nil, t.OpenErr
	}
	frames := make([][]byte, len(t.Frames))
	for i := range t.Frames {
		frames[i] = append([]byte(nil), t.Frames[i]...)
	}
	return &fakeByteStream{frames: frames, err: t.Err, errAtFrame: t.ErrAtFrame}, nil
}

type fakeByteStream struct {
	frames     [][]byte
	err        error
	errAtFrame int
	index      int
	closed     bool
}

func (s *fakeByteStream) Recv(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if s.err != nil && s.errAtFrame >= 0 && s.index == s.errAtFrame {
		return nil, s.err
	}
	if s.index >= len(s.frames) {
		if s.err != nil && s.errAtFrame < 0 {
			return nil, s.err
		}
		return nil, io.EOF
	}
	frame := append([]byte(nil), s.frames[s.index]...)
	s.index++
	return frame, nil
}

func (s *fakeByteStream) Close() error {
	s.closed = true
	return nil
}
