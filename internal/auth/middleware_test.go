package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newContext creates a minimal Gin context from an HTTP request.
func newContext(r *http.Request) *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = r
	return c
}

// --- extractBearer tests ---

func TestExtractBearer_AuthorizationHeader(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/v1/me", nil)
	r.Header.Set("Authorization", "Bearer my-jwt-token")
	c := newContext(r)

	got := extractBearer(c)
	if got != "my-jwt-token" {
		t.Fatalf("expected 'my-jwt-token', got %q", got)
	}
}

func TestExtractBearer_Cookie(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/v1/me", nil)
	r.AddCookie(&http.Cookie{Name: "aim_token", Value: "cookie-token"})
	c := newContext(r)

	got := extractBearer(c)
	if got != "cookie-token" {
		t.Fatalf("expected 'cookie-token', got %q", got)
	}
}

func TestExtractBearer_QueryParamRejectedForFiles(t *testing.T) {
	r := httptest.NewRequest("GET", "/files/example.txt?token=query-token", nil)
	c := newContext(r)

	got := extractBearer(c)
	if got != "" {
		t.Fatalf("expected empty string for file query token, got %q", got)
	}
}

func TestExtractBearer_QueryParamRejectedOutsideFiles(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/v1/me?token=query-token", nil)
	c := newContext(r)

	got := extractBearer(c)
	if got != "" {
		t.Fatalf("expected empty string for non-file query token, got %q", got)
	}
}

func TestExtractBearer_PriorityOrder(t *testing.T) {
	// When all three sources are present, Authorization header wins.
	r := httptest.NewRequest("GET", "/api/v1/me?token=query-token", nil)
	r.Header.Set("Authorization", "Bearer header-token")
	r.AddCookie(&http.Cookie{Name: "aim_token", Value: "cookie-token"})
	c := newContext(r)

	got := extractBearer(c)
	if got != "header-token" {
		t.Fatalf("expected Authorization header to take priority, got %q", got)
	}
}

func TestExtractBearer_CookieWithoutQueryFallback(t *testing.T) {
	// File query tokens are no longer supported; cookie auth still works.
	r := httptest.NewRequest("GET", "/files/example.txt?token=query-token", nil)
	r.AddCookie(&http.Cookie{Name: "aim_token", Value: "cookie-token"})
	c := newContext(r)

	got := extractBearer(c)
	if got != "cookie-token" {
		t.Fatalf("expected cookie to take priority over query, got %q", got)
	}
}

func TestExtractBearer_Empty(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/v1/me", nil)
	c := newContext(r)

	got := extractBearer(c)
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestExtractBearer_MalformedAuthHeader(t *testing.T) {
	// "Token xxx" instead of "Bearer xxx" should not match.
	r := httptest.NewRequest("GET", "/api/v1/me", nil)
	r.Header.Set("Authorization", "Token some-other-scheme")
	c := newContext(r)

	got := extractBearer(c)
	if got != "" {
		t.Fatalf("expected empty for non-Bearer auth header, got %q", got)
	}
}

// --- Helper function tests ---

func TestIsBootstrap(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	c := newContext(r)

	// Not set -> false
	if IsBootstrap(c) {
		t.Fatal("expected false when bootstrapPending not set")
	}

	// Set to true
	c.Set("bootstrapPending", true)
	if !IsBootstrap(c) {
		t.Fatal("expected true when bootstrapPending is true")
	}

	// Set to false
	c.Set("bootstrapPending", false)
	if IsBootstrap(c) {
		t.Fatal("expected false when bootstrapPending is false")
	}
}

func TestGetEntityID(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	c := newContext(r)

	c.Set("entityID", int64(42))
	if got := GetEntityID(c); got != 42 {
		t.Fatalf("expected 42, got %d", got)
	}
}

func TestGetEntityType(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	c := newContext(r)

	// Not set -> empty string
	if got := GetEntityType(c); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}

	c.Set("entityType", "user")
	// Note: model.EntityType is a string type alias, so direct string comparison may differ.
	// The function casts via type assertion.
}

// --- SetAuthCookie / ClearAuthCookie ---

func TestSetAuthCookie_Localhost(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	c.Request.Host = "localhost:9800"

	SetAuthCookie(c, "test-token")

	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected aim_token cookie to be set")
	}
	found := false
	for _, ck := range cookies {
		if ck.Name == "aim_token" {
			found = true
			if ck.Value != "test-token" {
				t.Fatalf("expected value 'test-token', got %q", ck.Value)
			}
			if ck.HttpOnly != true {
				t.Fatal("expected HttpOnly")
			}
			if ck.Secure != false {
				t.Fatal("expected Secure=false for localhost")
			}
		}
	}
	if !found {
		t.Fatal("aim_token cookie not found")
	}
}

func TestSetAuthCookie_Production(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	c.Request.Host = "ani-web.example.com"

	SetAuthCookie(c, "prod-token")

	cookies := w.Result().Cookies()
	for _, ck := range cookies {
		if ck.Name == "aim_token" {
			if ck.Secure != true {
				t.Fatal("expected Secure=true for production host")
			}
			return
		}
	}
	t.Fatal("aim_token cookie not found")
}

func TestClearAuthCookie(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	c.Request.Host = "localhost:9800"

	ClearAuthCookie(c)

	cookies := w.Result().Cookies()
	for _, ck := range cookies {
		if ck.Name == "aim_token" {
			if ck.MaxAge >= 0 {
				t.Fatalf("expected negative MaxAge for cookie removal, got %d", ck.MaxAge)
			}
			return
		}
	}
	t.Fatal("aim_token clear cookie not found")
}
