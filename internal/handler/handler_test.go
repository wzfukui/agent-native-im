package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wzfukui/agent-native-im/internal/config"
	"github.com/wzfukui/agent-native-im/internal/filestore"
	"github.com/wzfukui/agent-native-im/internal/handler"
	"github.com/wzfukui/agent-native-im/internal/store/postgres"
	"github.com/wzfukui/agent-native-im/internal/webhook"
	"github.com/wzfukui/agent-native-im/internal/ws"
)

var (
	testStore  *postgres.PGStore
	testHub    *ws.Hub
	testServer *handler.Server
	testRouter *gin.Engine
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)

	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://chris@localhost/agent_im_test?sslmode=disable"
	}

	var err error
	testStore, err = postgres.New(dbURL)
	if err != nil {
		log.Fatalf("failed to connect to test db: %v", err)
	}
	defer testStore.Close()

	testHub = ws.NewHub(testStore)
	go testHub.Run()

	tmpDir, _ := os.MkdirTemp("", "aim-test-files-*")

	fs, _ := filestore.NewLocalStore(tmpDir, "/files")

	cfg := &config.Config{
		Port:      "0",
		JWTSecret: "test-secret",
		AdminUser: "testadmin",
		AdminPass: "testpass",
		ServerURL: "http://localhost:9800",
	}

	testServer = &handler.Server{
		Config:    cfg,
		Store:     testStore,
		Hub:       testHub,
		Webhook:   webhook.NewDeliverer(testStore),
		Auth:      &handler.AuthHelper{Secret: cfg.JWTSecret, TokenTTL: 24 * time.Hour},
		FileStore: fs,
	}

	testRouter = handler.NewRouter(testServer)

	os.Exit(m.Run())
}

// truncateAll clears all tables in dependency order.
func truncateAll(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	for _, table := range []string{"reactions", "audit_logs", "conversation_change_requests", "conversation_memories", "tasks", "invite_links", "webhooks", "messages", "participants", "conversations", "credentials", "entities"} {
		_, err := testStore.DB.NewRaw(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table)).Exec(ctx)
		if err != nil {
			// Table might not exist yet (e.g. reactions before migration 10)
			t.Logf("truncate %s: %v (ignored)", table, err)
		}
	}
}

// seedAdmin creates the admin user and returns its JWT token.
func seedAdmin(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	err := testStore.SeedAdmin(ctx, "testadmin", "testpass")
	if err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	return login(t, "testadmin", "testpass")
}

// login performs a login and returns the JWT token.
func login(t *testing.T, username, password string) string {
	t.Helper()
	resp := doJSON(t, "POST", "/api/v1/auth/login", nil, map[string]string{
		"username": username,
		"password": password,
	})
	assertStatus(t, resp, http.StatusOK)
	data := parseOK(t, resp)
	token, ok := data["token"].(string)
	if !ok || token == "" {
		t.Fatal("login: missing token in response")
	}
	return token
}

// doJSON makes an HTTP request with optional JSON body and auth token.
func doJSON(t *testing.T, method, path string, token *string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != nil {
		req.Header.Set("Authorization", "Bearer "+*token)
	}

	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	return w
}

// ptr returns a pointer to the string value (for optional token param).
func ptr(s string) *string {
	return &s
}

// assertStatus checks the response status code.
func assertStatus(t *testing.T, resp *httptest.ResponseRecorder, expected int) {
	t.Helper()
	if resp.Code != expected {
		t.Fatalf("expected status %d, got %d; body: %s", expected, resp.Code, resp.Body.String())
	}
}

// parseResponse parses the response body as a generic JSON map.
func parseResponse(t *testing.T, resp *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &result); err != nil {
		t.Fatalf("parse response: %v; body: %s", err, resp.Body.String())
	}
	return result
}

// parseOK parses the response body and returns the "data" field.
func parseOK(t *testing.T, resp *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	result := parseResponse(t, resp)
	ok, _ := result["ok"].(bool)
	if !ok {
		t.Fatalf("expected ok=true; body: %s", resp.Body.String())
	}
	data, _ := result["data"].(map[string]interface{})
	return data
}
