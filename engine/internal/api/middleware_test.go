package api

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestStatusRecorderHijackProxiesUnderlyingWriter pins the
// WebSocket upgrade path: the statusRecorder wraps every response
// writer, and the gorilla/websocket Upgrade call requires
// http.Hijacker. Without proxying Hijack the wrapper hides the
// underlying writer's Hijack support and /ws returns HTTP 500
// instantly (the symptom the user actually hit).
func TestStatusRecorderHijackProxiesUnderlyingWriter(t *testing.T) {
	// httptest.NewServer's response writer IS hijackable; wrap it
	// in statusRecorder and assert the proxy works.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w}
		hj, ok := http.ResponseWriter(rec).(http.Hijacker)
		if !ok {
			http.Error(w, "not hijackable", http.StatusInternalServerError)
			return
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			http.Error(w, "hijack failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		// Write a minimal HTTP response by hand to confirm we own
		// the connection now.
		_, _ = conn.Write([]byte("HTTP/1.1 101 Switching Protocols\r\n\r\n"))
		_ = conn.Close()
	}))
	defer server.Close()

	// Open a raw TCP connection and request /, expect the hijacked
	// 101 response back.
	addr := server.Listener.Addr().String()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("GET / HTTP/1.1\r\nHost: localhost\r\n\r\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	br := bufio.NewReader(conn)
	line, err := br.ReadString('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if want := "HTTP/1.1 101"; !contains(line, want) {
		t.Fatalf("expected status line containing %q, got %q", want, line)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || (len(s) > len(sub) && (s[:len(sub)] == sub || contains(s[1:], sub))))
}
