package responsesws

import (
	"context"
	"sync"

	"github.com/codewandler/llmadapter/transport"
)

type Session struct {
	mu        sync.Mutex
	sessionID string
	stream    transport.ByteStream
}

func NewSession() *Session {
	return &Session{}
}

func (s *Session) Acquire() func() {
	s.mu.Lock()
	var once sync.Once
	return func() {
		once.Do(s.mu.Unlock)
	}
}

// OpenOrWrite requires the caller to hold the lock returned by Acquire.
func (s *Session) OpenOrWrite(ctx context.Context, sessionID string, body []byte, open func(context.Context) (transport.ByteStream, error)) (transport.ByteStream, error) {
	if sessionID != "" && s.stream != nil && s.sessionID == sessionID {
		writer, ok := s.stream.(transport.ByteStreamWriter)
		if !ok {
			s.CloseLocked()
		} else if err := writer.Write(ctx, body); err != nil {
			s.CloseLocked()
			return nil, err
		} else {
			return s.stream, nil
		}
	}
	if s.stream != nil {
		s.CloseLocked()
	}
	stream, err := open(ctx)
	if err != nil {
		return nil, err
	}
	if sessionID != "" {
		s.stream = stream
		s.sessionID = sessionID
	}
	return stream, nil
}

// CloseLocked requires the caller to hold the lock returned by Acquire.
func (s *Session) CloseLocked() {
	if s.stream != nil {
		_ = s.stream.Close()
	}
	s.stream = nil
	s.sessionID = ""
}
