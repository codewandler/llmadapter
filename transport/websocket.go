package transport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type WebSocketTransportConfig struct {
	Dialer            *websocket.Dialer
	HandshakeTimeout  time.Duration
	EnableCompression bool
	ForceIPv4         bool
}

type WebSocketByteStreamTransport struct {
	dialer *websocket.Dialer
}

func NewWebSocketByteStreamTransport(cfg WebSocketTransportConfig) *WebSocketByteStreamTransport {
	dialer := cfg.Dialer
	if dialer == nil {
		dialer = websocket.DefaultDialer
		copy := *dialer
		dialer = &copy
	}
	if cfg.HandshakeTimeout > 0 {
		copy := *dialer
		copy.HandshakeTimeout = cfg.HandshakeTimeout
		dialer = &copy
	}
	if cfg.EnableCompression {
		copy := *dialer
		copy.EnableCompression = true
		dialer = &copy
	}
	if cfg.ForceIPv4 {
		copy := *dialer
		netDialer := &net.Dialer{}
		copy.NetDialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return netDialer.DialContext(ctx, "tcp4", addr)
		}
		dialer = &copy
	}
	return &WebSocketByteStreamTransport{dialer: dialer}
}

func (t *WebSocketByteStreamTransport) Open(ctx context.Context, req *Request) (ByteStream, error) {
	header := http.Header(nil)
	if req.Header != nil {
		header = req.Header.Clone()
	}
	conn, resp, err := t.dialer.DialContext(ctx, req.URL, header)
	if err != nil {
		if resp != nil && resp.Body != nil {
			defer resp.Body.Close()
			body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
			return nil, apiErrorFromHTTP(resp.StatusCode, resp.Header, body)
		}
		return nil, err
	}
	body, err := requestBodyBytes(req.Body)
	if err != nil {
		conn.Close()
		return nil, err
	}
	stream := newWebSocketByteStream(conn, resp.Header.Clone())
	if len(body) > 0 {
		if err := stream.Write(ctx, body); err != nil {
			conn.Close()
			return nil, err
		}
	}
	return stream, nil
}

func requestBodyBytes(body io.Reader) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	if reader, ok := body.(*bytes.Reader); ok {
		pos, err := reader.Seek(0, io.SeekCurrent)
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(reader)
		if _, seekErr := reader.Seek(pos, io.SeekStart); seekErr != nil && err == nil {
			err = seekErr
		}
		return data, err
	}
	return io.ReadAll(body)
}

type webSocketByteStream struct {
	conn      *websocket.Conn
	header    http.Header
	writeMu   sync.Mutex
	closeOnce sync.Once
	done      chan struct{}
	readCh    chan webSocketRead
}

type webSocketRead struct {
	messageType int
	data        []byte
	err         error
}

func newWebSocketByteStream(conn *websocket.Conn, header http.Header) *webSocketByteStream {
	conn.SetPingHandler(func(appData string) error {
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(10*time.Second))
	})
	s := &webSocketByteStream{
		conn:   conn,
		header: header,
		done:   make(chan struct{}),
		readCh: make(chan webSocketRead, 32),
	}
	go s.readLoop()
	return s
}

func (s *webSocketByteStream) readLoop() {
	defer close(s.readCh)
	for {
		messageType, data, err := s.conn.ReadMessage()
		if err != nil {
			s.sendRead(webSocketRead{err: err})
			return
		}
		switch messageType {
		case websocket.TextMessage, websocket.BinaryMessage, websocket.CloseMessage:
			if !s.sendRead(webSocketRead{
				messageType: messageType,
				data:        append([]byte(nil), data...),
			}) {
				return
			}
		default:
			// Control frames are handled by gorilla while ReadMessage runs. Keep
			// the read pump alive so idle pooled connections answer server pings.
			continue
		}
	}
}

func (s *webSocketByteStream) sendRead(read webSocketRead) bool {
	select {
	case s.readCh <- read:
		return true
	case <-s.done:
		return false
	}
}

func (s *webSocketByteStream) Header() http.Header {
	return s.header.Clone()
}

func (s *webSocketByteStream) Write(ctx context.Context, frame []byte) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if len(frame) == 0 {
		return nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if deadline, ok := ctx.Deadline(); ok {
		_ = s.conn.SetWriteDeadline(deadline)
	} else {
		_ = s.conn.SetWriteDeadline(time.Time{})
	}
	return s.conn.WriteMessage(websocket.TextMessage, frame)
}

func (s *webSocketByteStream) Recv(ctx context.Context) ([]byte, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case read, ok := <-s.readCh:
			if !ok {
				return nil, io.EOF
			}
			if read.err != nil {
				return nil, webSocketReadError(read.err)
			}
			switch read.messageType {
			case websocket.TextMessage:
				return read.data, nil
			case websocket.BinaryMessage:
				return nil, fmt.Errorf("websocket binary messages are unsupported")
			case websocket.CloseMessage:
				return nil, io.EOF
			default:
				continue
			}
		}
	}
}

func webSocketReadError(err error) error {
	var closeErr *websocket.CloseError
	if errors.As(err, &closeErr) && closeErr.Text != "" {
		return fmt.Errorf("websocket closed: code=%d text=%s", closeErr.Code, closeErr.Text)
	}
	if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
		return io.EOF
	}
	return err
}

func (s *webSocketByteStream) Close() error {
	var err error
	s.closeOnce.Do(func() {
		close(s.done)
		err = s.conn.Close()
	})
	return err
}
