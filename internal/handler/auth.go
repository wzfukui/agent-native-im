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

// HandleRefreshToken issues a new JWT token for the authenticated entity.
func (s *Server) HandleRefreshToken(c *gin.Context) {
	entityID := auth.GetEntityID(c)
	entityType := auth.GetEntityType(c)

	token, err := s.Auth.GenerateToken(entityID, entityType)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to generate token")
		return
	}

	OK(c, http.StatusOK, gin.H{"token": token})
}

// HandleUpdateProfile updates the authenticated entity's display name and/or avatar.
func (s *Server) HandleUpdateProfile(c *gin.Context) {
	var req struct {
		DisplayName *string `json:"display_name"`
		AvatarURL   *string `json:"avatar_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	if req.DisplayName == nil && req.AvatarURL == nil {
		Fail(c, http.StatusBadRequest, "nothing to update")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	entity, err := s.Store.GetEntityByID(ctx, entityID)
	if err != nil {
		Fail(c, http.StatusNotFound, "entity not found")
		return
	}

	if req.DisplayName != nil {
		entity.DisplayName = *req.DisplayName
	}
	if req.AvatarURL != nil {
		entity.AvatarURL = *req.AvatarURL
	}

	if err := s.Store.UpdateEntity(ctx, entity); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to update profile")
		return
	}

	OK(c, http.StatusOK, entity)
}

// HandleChangePassword changes the authenticated user's password.
func (s *Server) HandleChangePassword(c *gin.Context) {
	if auth.GetEntityType(c) != model.EntityUser {
		Fail(c, http.StatusForbidden, "only users can change passwords")
		return
	}

	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "old_password and new_password required")
		return
	}

	if len(req.NewPassword) < 6 {
		Fail(c, http.StatusBadRequest, "new password must be at least 6 characters")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	// Verify old password
	creds, err := s.Store.GetCredentialsByEntity(ctx, entityID, model.CredPassword)
	if err != nil || len(creds) == 0 {
		Fail(c, http.StatusInternalServerError, "no password credential found")
		return
	}

	var matchedCred *model.Credential
	for _, cred := range creds {
		if err := bcrypt.CompareHashAndPassword([]byte(cred.SecretHash), []byte(req.OldPassword)); err == nil {
			matchedCred = cred
			break
		}
	}
	if matchedCred == nil {
		Fail(c, http.StatusUnauthorized, "old password is incorrect")
		return
	}

	// Hash new password and update
	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to hash password")
		return
	}

	matchedCred.SecretHash = string(newHash)
	if err := s.Store.UpdateCredential(ctx, matchedCred); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to update password")
		return
	}

	OK(c, http.StatusOK, "password changed")
}

// HandleCreateUser creates a new user entity with a password credential. Admin only.
func (s *Server) HandleCreateUser(c *gin.Context) {
	if auth.GetEntityType(c) != model.EntityUser {
		Fail(c, http.StatusForbidden, "only users can create users")
		return
	}

	var req struct {
		Username    string `json:"username" binding:"required"`
		Password    string `json:"password" binding:"required"`
		DisplayName string `json:"display_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "username and password required")
		return
	}

	if len(req.Password) < 6 {
		Fail(c, http.StatusBadRequest, "password must be at least 6 characters")
		return
	}

	ctx := c.Request.Context()

	displayName := req.DisplayName
	if displayName == "" {
		displayName = req.Username
	}

	entity := &model.Entity{
		EntityType:  model.EntityUser,
		Name:        req.Username,
		DisplayName: displayName,
		Status:      "active",
	}

	if err := s.Store.CreateEntity(ctx, entity); err != nil {
		Fail(c, http.StatusConflict, "username already exists or creation failed")
		return
	}

	// Create password credential
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to hash password")
		return
	}

	cred := &model.Credential{
		EntityID:   entity.ID,
		CredType:   model.CredPassword,
		SecretHash: string(hash),
	}

	if err := s.Store.CreateCredential(ctx, cred); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to create credential")
		return
	}

	OK(c, http.StatusCreated, entity)
}
