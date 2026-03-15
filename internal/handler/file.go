package handler

import (
	"fmt"
	"log"
	"mime"
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
	"text/",
	"application/pdf",
	"application/json",
	"application/zip",
	"application/x-tar",
	"application/gzip",
	"application/msword",
	"application/vnd.openxmlformats",
	"application/vnd.ms-excel",
	"application/vnd.ms-powerpoint",
}

func isAllowedMIME(filename string) bool {
	ext := filepath.Ext(filename)
	if ext == "" {
		return false
	}
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		return false
	}
	for _, prefix := range allowedMIMEPrefixes {
		if strings.HasPrefix(mimeType, prefix) {
			return true
		}
	}
	return false
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

	if !isAllowedMIME(header.Filename) {
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

	ext := filepath.Ext(header.Filename)
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = "application/octet-stream"
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
		log.Printf("WARN: failed to create file record for %s: %v", storedName, err)
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
		// Fallback: if no record exists (legacy files uploaded before this feature),
		// allow access if the user is authenticated (backward compatibility)
		if _, statErr := os.Stat(filePath); statErr != nil {
			Fail(c, http.StatusNotFound, "file not found")
			return
		}
		c.File(filePath)
		return
	}

	// If file is bound to a conversation, check participation
	if record.ConversationID != nil {
		ok, err := s.Store.IsParticipant(ctx, *record.ConversationID, entityID)
		if err != nil || !ok {
			Fail(c, http.StatusForbidden, "you do not have access to this file")
			return
		}
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
		c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, sanitizeFilename(record.OriginalName)))
	}
	c.File(filePath)
}
