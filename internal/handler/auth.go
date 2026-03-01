package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wzfukui/agent-native-im/internal/auth"
	"github.com/wzfukui/agent-native-im/internal/model"
	"golang.org/x/crypto/bcrypt"
)

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (s *Server) HandleLogin(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "username and password required")
		return
	}

	entity, err := s.Store.GetEntityByName(c.Request.Context(), req.Username, model.EntityUser)
	if err != nil {
		Fail(c, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Look up password credential for this entity
	creds, err := s.Store.GetCredentialsByEntity(c.Request.Context(), entity.ID, model.CredPassword)
	if err != nil || len(creds) == 0 {
		Fail(c, http.StatusUnauthorized, "invalid credentials")
		return
	}

	var matched bool
	for _, cred := range creds {
		if err := bcrypt.CompareHashAndPassword([]byte(cred.SecretHash), []byte(req.Password)); err == nil {
			matched = true
			break
		}
	}

	if !matched {
		Fail(c, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := s.Auth.GenerateToken(entity.ID, entity.EntityType)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to generate token")
		return
	}

	OK(c, http.StatusOK, gin.H{
		"token":  token,
		"entity": entity,
	})
}

// HandleMe returns the authenticated entity's info.
func (s *Server) HandleMe(c *gin.Context) {
	entityID := auth.GetEntityID(c)
	entity, err := s.Store.GetEntityByID(c.Request.Context(), entityID)
	if err != nil {
		Fail(c, http.StatusNotFound, "entity not found")
		return
	}
	OK(c, http.StatusOK, entity)
}
