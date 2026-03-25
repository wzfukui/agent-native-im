package middleware

import "testing"

func TestBuildAuditAction(t *testing.T) {
	if got := buildAuditAction("GET", "/api/v1/me"); got != "GET /api/v1/me" {
		t.Fatalf("unexpected short action: %q", got)
	}

	got := buildAuditAction("POST", "/api/v1/conversations/1234567890123456/participants")
	if len(got) > 50 {
		t.Fatalf("expected action length <= 50, got %d (%q)", len(got), got)
	}
	if got[:5] != "POST " {
		t.Fatalf("expected method prefix preserved, got %q", got)
	}
}
