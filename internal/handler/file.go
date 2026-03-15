package handler

import (
	"fmt"
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

	// Parse optional conversation_id
	var conversationID *int64
	if cidStr := c.PostForm("conversation_id"); cidStr != "" {
		cid, err := strconv.ParseInt(cidStr, 10, 64)
		if err == nil {
			conversationID = &cid
		}
	}

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
		// Log but don't fail the upload — the file is already saved
		// Legacy behavior still works without the record
		_ = err
	}

	OK(c, http.StatusCreated, gin.H{
		"url":       url,
		"filename":  header.Filename,
		"size":      header.Size,
		"mime_type": mimeType,
	})
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

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	// Look up file record
	record, err := s.Store.GetFileRecordByStoredName(ctx, filename)
	if err != nil {
		// Fallback: if no record exists (legacy files uploaded before this feature),
		// allow access if the user is authenticated (backward compatibility)
		filePath := filepath.Join(s.FileStore.ServePath(), filename)
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

	// Serve the file
	filePath := filepath.Join(s.FileStore.ServePath(), filename)
	if record.MimeType != "" {
		c.Header("Content-Type", record.MimeType)
	}
	if record.OriginalName != "" {
		c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, record.OriginalName))
	}
	c.File(filePath)
}
