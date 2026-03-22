package handler

import (
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wzfukui/agent-native-im/internal/auth"
	"github.com/wzfukui/agent-native-im/internal/model"
)

const maxUploadSize = 32 << 20 // 32 MB

// allowedMIMEPrefixes defines the MIME type prefixes accepted for upload.
var allowedMIMEPrefixes = []string{
	"image/",
	"audio/",
	"video/",
}

var allowedExactMIMEs = map[string]bool{
	"text/plain":         true,
	"text/markdown":      true,
	"text/csv":           true,
	"application/json":   true,
	"application/pdf":    true,
	"application/zip":    true,
	"application/x-tar":  true,
	"application/gzip":   true,
	"application/msword": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	"application/vnd.ms-excel": true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         true,
	"application/vnd.ms-powerpoint":                                             true,
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": true,
}

var blockedActiveMIMEs = map[string]bool{
	"text/html":             true,
	"application/xhtml+xml": true,
	"image/svg+xml":         true,
	"text/xml":              true,
	"application/xml":       true,
}

func normalizeMIME(mimeType string) string {
	return strings.TrimSpace(strings.Split(mimeType, ";")[0])
}

func isAllowedMIME(mimeType string) bool {
	mimeType = normalizeMIME(mimeType)
	if mimeType == "" || blockedActiveMIMEs[mimeType] {
		return false
	}
	if allowedExactMIMEs[mimeType] {
		return true
	}
	for _, prefix := range allowedMIMEPrefixes {
		if strings.HasPrefix(mimeType, prefix) {
			if mimeType == "image/svg+xml" {
				return false
			}
			return true
		}
	}
	return false
}

func detectUploadMIME(filename string, file multipart.File) (string, bool) {
	extMime := normalizeMIME(mime.TypeByExtension(filepath.Ext(filename)))
	buf := make([]byte, 512)
	n, _ := io.ReadFull(file, buf)
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", false
	}
	sniffed := normalizeMIME(http.DetectContentType(buf[:n]))

	candidates := []string{extMime, sniffed}
	for _, candidate := range candidates {
		if isAllowedMIME(candidate) {
			return candidate, true
		}
	}
	return "", false
}

func shouldServeInline(mimeType string) bool {
	mimeType = normalizeMIME(mimeType)
	switch {
	case strings.HasPrefix(mimeType, "image/") && mimeType != "image/svg+xml":
		return true
	case strings.HasPrefix(mimeType, "audio/"):
		return true
	case strings.HasPrefix(mimeType, "video/"):
		return true
	case mimeType == "application/pdf":
		return true
	case mimeType == "text/plain":
		return true
	case mimeType == "application/json":
		return true
	default:
		return false
	}
}

// HandleFileUpload handles multipart file uploads.
func (s *Server) HandleFileUpload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize)

	// Parse the entire multipart form first so both file and form fields are available
	if err := c.Request.ParseMultipartForm(maxUploadSize); err != nil {
		Fail(c, http.StatusBadRequest, "file is required (max 32MB)")
		return
	}

	// Validate conversation_id early (before processing file)
	var conversationID *int64
	if cidStr := c.Request.FormValue("conversation_id"); cidStr != "" {
		cid, err := strconv.ParseInt(cidStr, 10, 64)
		if err != nil {
			Fail(c, http.StatusBadRequest, "invalid conversation_id")
			return
		}
		conversationID = &cid
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		Fail(c, http.StatusBadRequest, "file is required (max 32MB)")
		return
	}
	defer file.Close()

	mimeType, ok := detectUploadMIME(header.Filename, file)
	if !ok {
		Fail(c, http.StatusBadRequest, "file type not allowed")
		return
	}

	if s.FileStore == nil {
		Fail(c, http.StatusInternalServerError, "file upload not configured")
		return
	}

	url, err := s.FileStore.Save(header.Filename, file)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to save file")
		return
	}

	// Extract stored filename from URL (last path segment)
	storedName := filepath.Base(url)

	// Create file record in database
	entityID := auth.GetEntityID(c)
	record := &model.FileRecord{
		StoredName:     storedName,
		OriginalName:   header.Filename,
		MimeType:       mimeType,
		Size:           header.Size,
		UploaderID:     entityID,
		ConversationID: conversationID,
	}
	if err := s.Store.CreateFileRecord(c.Request.Context(), record); err != nil {
		slog.Warn("failed to create file record", "stored_name", storedName, "error", err)
	}

	OK(c, http.StatusCreated, gin.H{
		"url":       url,
		"filename":  header.Filename,
		"size":      header.Size,
		"mime_type": mimeType,
	})
}

// safeFilePath validates that the resolved path stays within baseDir to prevent path traversal.
func safeFilePath(baseDir, filename string) (string, bool) {
	// Reject empty, absolute paths, and traversal attempts
	if filename == "" || filepath.IsAbs(filename) || strings.Contains(filename, "..") {
		return "", false
	}
	base := filepath.Clean(baseDir)
	joined := filepath.Clean(filepath.Join(base, filename))
	// Ensure the resolved path is strictly within the base directory
	if !strings.HasPrefix(joined, base+string(os.PathSeparator)) {
		return "", false
	}
	return joined, true
}

// sanitizeFilename strips control characters and quotes from a filename for use in headers.
func sanitizeFilename(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r == '"' || r == '\\' || r < 32 {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// HandleFileDownload serves files with authentication and access control.
func (s *Server) HandleFileDownload(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" {
		Fail(c, http.StatusBadRequest, "filename required")
		return
	}

	// Clean the filename (remove leading slashes)
	filename = strings.TrimPrefix(filename, "/")

	// Prevent path traversal attacks
	filePath, safe := safeFilePath(s.FileStore.ServePath(), filename)
	if !safe {
		Fail(c, http.StatusBadRequest, "invalid filename")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	// Look up file record
	record, err := s.Store.GetFileRecordByStoredName(ctx, filename)
	if err != nil {
		if _, statErr := os.Stat(filePath); statErr != nil {
			Fail(c, http.StatusNotFound, "file not found")
			return
		}
		Fail(c, http.StatusForbidden, "file metadata missing")
		return
	}

	// If file is bound to a conversation, check participation
	if record.ConversationID != nil {
		ok, err := s.Store.IsParticipant(ctx, *record.ConversationID, entityID)
		if err != nil || !ok {
			Fail(c, http.StatusForbidden, "you do not have access to this file")
			return
		}
	} else if record.UploaderID != entityID {
		Fail(c, http.StatusForbidden, "you do not have access to this file")
		return
	}

	// Verify file exists on disk
	if _, err := os.Stat(filePath); err != nil {
		Fail(c, http.StatusNotFound, "file not found")
		return
	}

	// Serve the file with proper headers
	if record.MimeType != "" {
		c.Header("Content-Type", record.MimeType)
	}
	if record.OriginalName != "" {
		disposition := "attachment"
		if shouldServeInline(record.MimeType) {
			disposition = "inline"
		}
		c.Header("Content-Disposition", fmt.Sprintf(`%s; filename="%s"`, disposition, sanitizeFilename(record.OriginalName)))
	}
	c.File(filePath)
}

// HandleAvatarDownload serves profile avatars through a stable, cacheable URL.
// Avatars are still backed by the same files directory, but access is based on
// whether the filename is referenced by an entity profile instead of generic
// attachment auth rules.
func (s *Server) HandleAvatarDownload(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" {
		Fail(c, http.StatusBadRequest, "filename required")
		return
	}

	filename = strings.TrimPrefix(filename, "/")
	filePath, safe := safeFilePath(s.FileStore.ServePath(), filename)
	if !safe {
		Fail(c, http.StatusBadRequest, "invalid filename")
		return
	}

	referenced, err := s.Store.IsAvatarStoredNameReferenced(c.Request.Context(), filename)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to resolve avatar")
		return
	}
	if !referenced {
		Fail(c, http.StatusNotFound, "avatar not found")
		return
	}

	if _, err := os.Stat(filePath); err != nil {
		Fail(c, http.StatusNotFound, "avatar file not found")
		return
	}

	if contentType := mime.TypeByExtension(filepath.Ext(filename)); contentType != "" {
		c.Header("Content-Type", contentType)
	}
	c.Header("Cache-Control", "public, max-age=604800, immutable")
	c.File(filePath)
}
