package handler

import (
	"net/http/httptest"
	"testing"
)

func TestExtractWebSocketTokenFromSubprotocol(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/ws", nil)
	req.Header.Set("Sec-WebSocket-Protocol", "ani.bearer.jwt-token-123")

	token, protocol := extractWebSocketToken(req)
	if token != "jwt-token-123" {
		t.Fatalf("expected token from subprotocol, got %q", token)
	}
	if protocol != "ani.bearer.jwt-token-123" {
		t.Fatalf("expected selected subprotocol to round-trip, got %q", protocol)
	}
}

func TestAllowedBrowserOrigin(t *testing.T) {
	cases := []struct {
		origin string
		want   bool
	}{
		{origin: "https://agent-native.im", want: true},
		{origin: "https://demo.example.com", want: true},
		{origin: "http://localhost:3000", want: true},
		{origin: "http://127.0.0.1:5173", want: true},
		{origin: "http://192.168.44.43:8081", want: true},
		{origin: "http://evil.example.com", want: false},
		{origin: "ftp://demo.example.com", want: false},
		{origin: "not-an-origin", want: false},
	}

	for _, tc := range cases {
		if got := isAllowedBrowserOrigin(tc.origin); got != tc.want {
			t.Fatalf("origin %q: want %v, got %v", tc.origin, tc.want, got)
		}
	}
}
