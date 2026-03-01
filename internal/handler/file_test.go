package handler_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFileUpload(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", "test-doc.txt")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	part.Write([]byte("Hello, this is a test file content."))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/files/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)

	assertStatus(t, w, http.StatusCreated)

	data := parseOK(t, w)
	url, _ := data["url"].(string)
	if url == "" {
		t.Fatal("expected non-empty URL")
	}
	if data["filename"] != "test-doc.txt" {
		t.Fatalf("expected filename=test-doc.txt, got %v", data["filename"])
	}
}

func TestFileUploadNoFile(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	req := httptest.NewRequest("POST", "/api/v1/files/upload", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

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

func TestFileUploadBlockedMIME(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Try uploading a file with no recognized extension
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

func TestFileUploadAllowedTypes(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	allowedFiles := []string{"photo.png", "doc.pdf", "data.json", "archive.zip"}
	for _, filename := range allowedFiles {
		t.Run(filename, func(t *testing.T) {
			var buf bytes.Buffer
			writer := multipart.NewWriter(&buf)
			part, _ := writer.CreateFormFile("file", filename)
			part.Write([]byte("content"))
			writer.Close()

			req := httptest.NewRequest("POST", "/api/v1/files/upload", &buf)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			req.Header.Set("Authorization", "Bearer "+token)

			w := httptest.NewRecorder()
			testRouter.ServeHTTP(w, req)

			assertStatus(t, w, http.StatusCreated)
		})
	}
}
