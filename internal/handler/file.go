package handler

import (
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
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

	OK(c, http.StatusCreated, gin.H{
		"url":      url,
		"filename": header.Filename,
		"size":     header.Size,
	})
}
