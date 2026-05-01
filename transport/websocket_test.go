package transport

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestWebSocketByteStreamTransportSendsBodyAndReceivesTextFrames(t *testing.T) {
	upgrader := websocket.Upgrader{}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("local listener unavailable: %v", err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		messageType, body, err := conn.ReadMessage()
		if err != nil {
			t.Errorf("read message: %v", err)
			return
		}
		if messageType != websocket.TextMessage || string(body) != `{"type":"response.create"}` {
			t.Errorf("message = type %d body %s", messageType, body)
			return
		}
		if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.created"}`)); err != nil {
			t.Errorf("write message: %v", err)
		}
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	stream, err := NewWebSocketByteStreamTransport(WebSocketTransportConfig{}).Open(context.Background(), &Request{
		URL:  wsURL,
		Body: bytes.NewReader([]byte(`{"type":"response.create"}`)),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	got, err := stream.Recv(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"type":"response.created"}` {
		t.Fatalf("frame = %s", got)
	}
}

func TestWebSocketByteStreamTransportRespondsToPingWhileCallerIsIdle(t *testing.T) {
	upgrader := websocket.Upgrader{}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("local listener unavailable: %v", err)
	}
	pongReceived := make(chan struct{})
	serverErr := make(chan error, 1)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			serverErr <- fmt.Errorf("upgrade: %w", err)
			return
		}
		defer conn.Close()
		messageType, body, err := conn.ReadMessage()
		if err != nil {
			serverErr <- fmt.Errorf("read message: %w", err)
			return
		}
		if messageType != websocket.TextMessage || string(body) != `{"type":"response.create"}` {
			serverErr <- fmt.Errorf("message = type %d body %s", messageType, body)
			return
		}
		if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.created"}`)); err != nil {
			serverErr <- fmt.Errorf("write message: %w", err)
			return
		}
		var closePongOnce sync.Once
		conn.SetPongHandler(func(appData string) error {
			if appData == "keepalive" {
				closePongOnce.Do(func() {
					close(pongReceived)
				})
			}
			return nil
		})
		if err := conn.WriteControl(websocket.PingMessage, []byte("keepalive"), time.Now().Add(time.Second)); err != nil {
			serverErr <- fmt.Errorf("write ping: %w", err)
			return
		}
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	stream, err := NewWebSocketByteStreamTransport(WebSocketTransportConfig{}).Open(context.Background(), &Request{
		URL:  wsURL,
		Body: bytes.NewReader([]byte(`{"type":"response.create"}`)),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	got, err := stream.Recv(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"type":"response.created"}` {
		t.Fatalf("frame = %s", got)
	}
	select {
	case <-pongReceived:
	case err := <-serverErr:
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for websocket pong while caller was idle")
	}
}

func TestWebSocketByteStreamTransportCanEnableCompression(t *testing.T) {
	upgrader := websocket.Upgrader{EnableCompression: true}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("local listener unavailable: %v", err)
	}
	seenExtension := make(chan string, 1)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenExtension <- r.Header.Get("Sec-Websocket-Extensions")
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.created"}`)); err != nil {
			t.Errorf("write message: %v", err)
		}
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	stream, err := NewWebSocketByteStreamTransport(WebSocketTransportConfig{EnableCompression: true}).Open(context.Background(), &Request{URL: wsURL})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	if _, err := stream.Recv(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := <-seenExtension; !strings.Contains(strings.ToLower(got), "permessage-deflate") {
		t.Fatalf("Sec-Websocket-Extensions = %q", got)
	}
}

func TestWebSocketByteStreamTransportCanForceIPv4(t *testing.T) {
	upgrader := websocket.Upgrader{}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("local ipv4 listener unavailable: %v", err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.created"}`)); err != nil {
			t.Errorf("write message: %v", err)
		}
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	stream, err := NewWebSocketByteStreamTransport(WebSocketTransportConfig{ForceIPv4: true}).Open(context.Background(), &Request{URL: wsURL})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	got, err := stream.Recv(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"type":"response.created"}` {
		t.Fatalf("frame = %s", got)
	}
}
