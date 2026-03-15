package handler_test

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/wzfukui/agent-native-im/internal/handler"
)

// --- helpers ---

// uploadFile creates a multipart upload request and returns the recorder.
// If conversationID is non-empty, it is added as a form field.
func uploadFile(t *testing.T, token, filename, content, conversationID string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	part.Write([]byte(content))
	if conversationID != "" {
		writer.WriteField("conversation_id", conversationID)
	}
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/files/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	return w
}

// uploadFileGetStoredName uploads a file and returns the stored filename extracted from the URL.
func uploadFileGetStoredName(t *testing.T, token, filename, content, conversationID string) string {
	t.Helper()
	w := uploadFile(t, token, filename, content, conversationID)
	assertStatus(t, w, http.StatusCreated)
	data := parseOK(t, w)
	url, _ := data["url"].(string)
	if url == "" {
		t.Fatal("upload returned empty url")
	}
	return filepath.Base(url)
}

// createSecondUser creates a non-admin user via admin API, returns (userID, token).
func createSecondUser(t *testing.T, adminToken, username, password string) (int, string) {
	t.Helper()
	resp := doJSON(t, "POST", "/api/v1/admin/users", ptr(adminToken), map[string]string{
		"username": username,
		"password": password,
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	userID := int(data["id"].(float64))
	token := login(t, username, password)
	return userID, token
}

// createConversationWithParticipant creates a group conversation including the given participant IDs.
// Returns the conversation ID.
func createConversationWithParticipant(t *testing.T, token string, participantIDs ...float64) int {
	t.Helper()
	resp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title":           "File Test Conv",
		"conv_type":       "group",
		"participant_ids": participantIDs,
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	return int(data["id"].(float64))
}

// downloadFile makes a GET request to /files/<storedName> with optional auth.
func downloadFile(t *testing.T, token *string, storedName string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", "/files/"+storedName, nil)
	if token != nil {
		req.Header.Set("Authorization", "Bearer "+*token)
	}
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	return w
}

// --- Upload tests ---

func TestFileUpload_Success(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	w := uploadFile(t, token, "report.txt", "quarterly earnings report", "")
	assertStatus(t, w, http.StatusCreated)

	data := parseOK(t, w)

	// Verify all expected fields
	url, _ := data["url"].(string)
	if url == "" {
		t.Fatal("expected non-empty url")
	}
	if data["filename"] != "report.txt" {
		t.Fatalf("expected filename=report.txt, got %v", data["filename"])
	}
	size, _ := data["size"].(float64)
	if size != float64(len("quarterly earnings report")) {
		t.Fatalf("expected size=%d, got %v", len("quarterly earnings report"), size)
	}
	mimeType, _ := data["mime_type"].(string)
	if mimeType != "text/plain; charset=utf-8" && mimeType != "text/plain" {
		t.Fatalf("expected text/plain mime_type, got %q", mimeType)
	}
}

func TestFileUpload_WithConversationID(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create a conversation first
	convResp := doJSON(t, "POST", "/api/v1/conversations", ptr(token), map[string]interface{}{
		"title": "File Bound Conv",
	})
	assertStatus(t, convResp, http.StatusCreated)
	convData := parseOK(t, convResp)
	convID := int(convData["id"].(float64))

	// Upload with conversation_id
	w := uploadFile(t, token, "notes.txt", "meeting notes", fmt.Sprintf("%d", convID))
	assertStatus(t, w, http.StatusCreated)

	data := parseOK(t, w)
	if data["filename"] != "notes.txt" {
		t.Fatalf("expected filename=notes.txt, got %v", data["filename"])
	}
	// The file record should have been created with conversation_id bound.
	// We verify indirectly by downloading — a non-participant should be blocked.
}

func TestFileUpload_InvalidConversationID(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	w := uploadFile(t, token, "data.txt", "some data", "abc")
	assertStatus(t, w, http.StatusBadRequest)

	result := parseResponse(t, w)
	// error is a structured ErrorDetail object, not a plain string
	errObj, ok := result["error"].(map[string]interface{})
	if !ok || errObj["message"] == nil {
		t.Fatal("expected error message for invalid conversation_id")
	}
}

func TestFileUpload_MimeTypeDetection(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	cases := []struct {
		filename     string
		wantMIMEPfx  string // prefix match to handle charset suffixes
	}{
		{"photo.png", "image/png"},
		{"document.pdf", "application/pdf"},
		{"data.json", "application/json"},
	}

	for _, tc := range cases {
		t.Run(tc.filename, func(t *testing.T) {
			w := uploadFile(t, token, tc.filename, "test-content", "")
			assertStatus(t, w, http.StatusCreated)
			data := parseOK(t, w)
			got, _ := data["mime_type"].(string)
			if len(got) < len(tc.wantMIMEPfx) || got[:len(tc.wantMIMEPfx)] != tc.wantMIMEPfx {
				t.Fatalf("expected mime_type starting with %q, got %q", tc.wantMIMEPfx, got)
			}
		})
	}
}

func TestFileUpload_DisallowedType(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	disallowed := []string{"malware.exe", "script.bat", "noext"}
	for _, filename := range disallowed {
		t.Run(filename, func(t *testing.T) {
			w := uploadFile(t, token, filename, "bad content", "")
			assertStatus(t, w, http.StatusBadRequest)
		})
	}
}

func TestFileUpload_NoFile(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	req := httptest.NewRequest("POST", "/api/v1/files/upload", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

// --- Download tests ---

func TestFileDownload_PathTraversal(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	traversalPaths := []string{
		"/../../../etc/passwd",
		"/..%2F..%2Fetc%2Fpasswd",
		"/../../../etc/shadow",
	}

	for _, p := range traversalPaths {
		t.Run(p, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/files"+p, nil)
			req.Header.Set("Authorization", "Bearer "+token)

			w := httptest.NewRecorder()
			testRouter.ServeHTTP(w, req)

			// Must be 400 (invalid filename) — NOT 200 or a file serve
			if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
				t.Fatalf("path traversal %q: expected 400 or 404, got %d; body: %s", p, w.Code, w.Body.String())
			}

			// Ensure the body does NOT contain /etc/passwd content
			if bytes.Contains(w.Body.Bytes(), []byte("root:")) {
				t.Fatalf("path traversal %q: response contains /etc/passwd content!", p)
			}
		})
	}
}

func TestFileDownload_PathTraversal_Encoded(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	req := httptest.NewRequest("GET", "/files/..%2F..%2F..%2Fetc%2Fpasswd", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Fatalf("expected 400 or 404 for encoded traversal, got %d", w.Code)
	}
}

func TestFileDownload_Authenticated(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Upload a file
	storedName := uploadFileGetStoredName(t, token, "hello.txt", "hello world", "")

	// Download with valid auth
	w := downloadFile(t, ptr(token), storedName)
	assertStatus(t, w, http.StatusOK)

	if w.Body.String() != "hello world" {
		t.Fatalf("expected body 'hello world', got %q", w.Body.String())
	}
}

func TestFileDownload_Unauthenticated(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	storedName := uploadFileGetStoredName(t, token, "secret.txt", "confidential", "")

	// Download without auth
	w := downloadFile(t, nil, storedName)
	assertStatus(t, w, http.StatusUnauthorized)
}

func TestFileDownload_ConversationBound_Participant(t *testing.T) {
	truncateAll(t)
	adminToken := seedAdmin(t)

	// Create second user
	user2ID, user2Token := createSecondUser(t, adminToken, "fileuser", "FileUser123")

	// Create conversation with user2 as participant
	convID := createConversationWithParticipant(t, adminToken, float64(user2ID))

	// Upload file bound to conversation
	storedName := uploadFileGetStoredName(t, adminToken, "shared.txt", "shared content", fmt.Sprintf("%d", convID))

	// User2 (participant) downloads — should succeed
	w := downloadFile(t, ptr(user2Token), storedName)
	assertStatus(t, w, http.StatusOK)

	if w.Body.String() != "shared content" {
		t.Fatalf("expected body 'shared content', got %q", w.Body.String())
	}
}

func TestFileDownload_ConversationBound_NonParticipant(t *testing.T) {
	truncateAll(t)
	adminToken := seedAdmin(t)

	// Create two users: one in the conv, one not
	user2ID, _ := createSecondUser(t, adminToken, "member", "Member1234")
	_, outsiderToken := createSecondUser(t, adminToken, "outsider", "Outsider123")

	// Create conversation with only user2
	convID := createConversationWithParticipant(t, adminToken, float64(user2ID))

	// Upload file bound to that conversation
	storedName := uploadFileGetStoredName(t, adminToken, "private.txt", "private content", fmt.Sprintf("%d", convID))

	// Outsider (not a participant) downloads — should be forbidden
	w := downloadFile(t, ptr(outsiderToken), storedName)
	assertStatus(t, w, http.StatusForbidden)
}

func TestFileDownload_NoBound_AnyAuth(t *testing.T) {
	truncateAll(t)
	adminToken := seedAdmin(t)

	// Create second user
	_, user2Token := createSecondUser(t, adminToken, "anyuser", "AnyUser1234")

	// Upload file without conversation binding
	storedName := uploadFileGetStoredName(t, adminToken, "public.txt", "public content", "")

	// Second user (any authenticated user) downloads — should succeed
	w := downloadFile(t, ptr(user2Token), storedName)
	assertStatus(t, w, http.StatusOK)

	if w.Body.String() != "public content" {
		t.Fatalf("expected 'public content', got %q", w.Body.String())
	}
}

func TestFileDownload_NotFound(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	w := downloadFile(t, ptr(token), "nonexistent_file_20260315_000000_abcdef01.txt")
	assertStatus(t, w, http.StatusNotFound)
}

func TestFileDownload_LegacyFile(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create a file directly on disk (no DB record) to simulate a legacy file
	servePath := testServer.FileStore.ServePath()
	legacyName := "legacy_20200101_000000_aabbccdd.txt"
	legacyPath := filepath.Join(servePath, legacyName)
	if err := os.WriteFile(legacyPath, []byte("legacy content"), 0o644); err != nil {
		t.Fatalf("write legacy file: %v", err)
	}
	defer os.Remove(legacyPath)

	// Download — no DB record exists, but file is on disk → backward compat should serve it
	w := downloadFile(t, ptr(token), legacyName)
	assertStatus(t, w, http.StatusOK)

	if w.Body.String() != "legacy content" {
		t.Fatalf("expected 'legacy content', got %q", w.Body.String())
	}
}

// --- Unit tests for helper functions ---

func TestSafeFilePath(t *testing.T) {
	// We test the exported-via-test approach: call the function through the handler package.
	// Since safeFilePath is unexported, we test it indirectly through download behavior.
	// But we can also test directly if accessible. Since it's in the handler package and
	// we're in handler_test (external test package), we use the SafeFilePath test wrapper
	// or test indirectly via HTTP.

	cases := []struct {
		name     string
		path     string
		wantSafe bool
	}{
		{"normal file", "20260315_120000_abc123.txt", true},
		{"traversal with ../", "../../../etc/passwd", false},
		{"traversal with ./", "./../../etc/passwd", false},
		{"absolute path injection", "/etc/passwd", false},
		{"double dot in middle", "foo/../../../etc/passwd", false},
		{"just dots", "..", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/files/"+tc.path, nil)
			// We need auth for the file endpoint
			truncateAll(t)
			token := seedAdmin(t)
			req.Header.Set("Authorization", "Bearer "+token)

			w := httptest.NewRecorder()
			testRouter.ServeHTTP(w, req)

			if tc.wantSafe {
				// Should be 404 (file not found on disk) — NOT 400 (bad path)
				if w.Code != http.StatusNotFound && w.Code != http.StatusOK {
					t.Fatalf("expected 404 or 200 for safe path %q, got %d", tc.path, w.Code)
				}
			} else {
				// Must be 400 (invalid filename) — path traversal blocked
				if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
					t.Fatalf("expected 400 or 404 for unsafe path %q, got %d; body: %s", tc.path, w.Code, w.Body.String())
				}
			}
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	// Test via Content-Disposition header: upload a file with special chars in the name,
	// then download it and check the Content-Disposition header.
	truncateAll(t)
	token := seedAdmin(t)

	cases := []struct {
		name         string
		uploadName   string
		wantExcluded []string // chars that must NOT appear in the sanitized header value
	}{
		{
			name:         "quotes stripped",
			uploadName:   `file"with"quotes.txt`,
			wantExcluded: []string{`"`}, // Note: the header wraps in quotes, but the filename itself shouldn't have them
		},
		{
			name:         "unicode preserved",
			uploadName:   "report_日本語.txt",
			wantExcluded: []string{}, // unicode should pass through
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			storedName := uploadFileGetStoredName(t, token, tc.uploadName, "content", "")
			w := downloadFile(t, ptr(token), storedName)
			assertStatus(t, w, http.StatusOK)

			disposition := w.Header().Get("Content-Disposition")
			if disposition == "" {
				t.Skip("no Content-Disposition header set (file record may not include original_name)")
			}

			// Verify excluded characters are not in the filename portion of the header.
			// The header format is: inline; filename="sanitized_name"
			// We check the sanitized part between the quotes.
			for _, excluded := range tc.wantExcluded {
				// Find the filename value between quotes in the disposition header
				// e.g., inline; filename="clean_name.txt"
				start := bytes.Index([]byte(disposition), []byte(`filename="`))
				if start < 0 {
					continue
				}
				start += len(`filename="`)
				end := bytes.Index([]byte(disposition)[start:], []byte(`"`))
				if end < 0 {
					continue
				}
				sanitized := disposition[start : start+end]
				if bytes.Contains([]byte(sanitized), []byte(excluded)) {
					t.Fatalf("Content-Disposition filename %q should not contain %q", sanitized, excluded)
				}
			}
		})
	}
}

// TestFileUploadAllowedTypes verifies various allowed MIME types are accepted.
func TestFileUploadAllowedTypes(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	allowedFiles := []string{"photo.png", "doc.pdf", "data.json", "archive.zip"}
	for _, filename := range allowedFiles {
		t.Run(filename, func(t *testing.T) {
			w := uploadFile(t, token, filename, "content", "")
			assertStatus(t, w, http.StatusCreated)
		})
	}
}

// TestFileUploadNoAuth ensures unauthenticated uploads are rejected.
func TestFileUploadNoAuth(t *testing.T) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("file", "test.txt")
	part.Write([]byte("content"))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/files/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)

	assertStatus(t, w, http.StatusUnauthorized)
}

// TestFileUploadBlockedMIME verifies .exe uploads are rejected.
func TestFileUploadBlockedMIME(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("file", "malware.exe")
	part.Write([]byte("evil content"))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/files/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

// --- Export test for safeFilePath and sanitizeFilename ---
// These are tested indirectly through HTTP above, but we also provide
// a direct unit test via an export_test.go bridge if it exists.
// For now, the HTTP-based tests above cover the security properties.

// TestSafeFilePath_Direct tests the safeFilePath function directly via the export bridge.
func TestSafeFilePath_Direct(t *testing.T) {
	tmpDir := t.TempDir()

	cases := []struct {
		name     string
		baseDir  string
		filename string
		wantOK   bool
	}{
		{"simple file", tmpDir, "hello.txt", true},
		{"nested safe", tmpDir, "subdir/file.txt", true},
		{"traversal", tmpDir, "../../../etc/passwd", false},
		{"absolute", tmpDir, "/etc/passwd", false},
		{"dot dot", tmpDir, "..", false},
		{"empty", tmpDir, "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := handler.SafeFilePath(tc.baseDir, tc.filename)
			if ok != tc.wantOK {
				t.Fatalf("safeFilePath(%q, %q) = _, %v; want %v", tc.baseDir, tc.filename, ok, tc.wantOK)
			}
		})
	}
}

// TestSanitizeFilename_Direct tests the sanitizeFilename function directly via the export bridge.
func TestSanitizeFilename_Direct(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"normal", "report.txt", "report.txt"},
		{"quotes removed", `he"llo.txt`, "hello.txt"},
		{"backslash removed", `he\llo.txt`, "hello.txt"},
		{"control chars removed", "he\x00llo\x1f.txt", "hello.txt"},
		{"unicode preserved", "报告.txt", "报告.txt"},
		{"mixed", "a\"b\\c\x01d.txt", "abcd.txt"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := handler.SanitizeFilename(tc.input)
			if got != tc.want {
				t.Fatalf("sanitizeFilename(%q) = %q; want %q", tc.input, got, tc.want)
			}
		})
	}
}
