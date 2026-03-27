package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wzfukui/agent-native-im/internal/auth"
	"github.com/wzfukui/agent-native-im/internal/model"
	"golang.org/x/crypto/bcrypt"
)

// validatePassword checks if a password meets security requirements
func validatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	if len(password) > 128 {
		return fmt.Errorf("password must be less than 128 characters")
	}

	var hasUpper, hasLower, hasNumber bool
	for _, char := range password {
		switch {
		case 'A' <= char && char <= 'Z':
			hasUpper = true
		case 'a' <= char && char <= 'z':
			hasLower = true
		case '0' <= char && char <= '9':
			hasNumber = true
		}
	}

	if !hasUpper || !hasLower || !hasNumber {
		return fmt.Errorf("password must contain uppercase, lowercase, and numbers")
	}

	// Check for common weak passwords
	lowerPass := strings.ToLower(password)
	weakPasswords := []string{"password", "12345678", "qwerty", "admin", "letmein", "welcome", "123456"}
	for _, weak := range weakPasswords {
		if strings.Contains(lowerPass, weak) {
			return fmt.Errorf("password is too common or weak")
		}
	}

	return nil
}

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

	ctx := c.Request.Context()

	// Smart login: if input contains '@', try email first, then username
	var entity *model.Entity
	var err error
	if strings.Contains(req.Username, "@") {
		entity, err = s.Store.GetEntityByEmail(ctx, req.Username)
		if err != nil {
			// Fallback to username lookup
			entity, err = s.Store.GetEntityByName(ctx, req.Username, model.EntityUser)
		}
	} else {
		entity, err = s.Store.GetEntityByName(ctx, req.Username, model.EntityUser)
	}
	if err != nil {
		FailWithCode(c, http.StatusUnauthorized, ErrCodeAuthInvalid, "invalid credentials")
		return
	}
	if entity.Status == "disabled" {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "account is disabled")
		return
	}

	// Look up password credential for this entity
	creds, err := s.Store.GetCredentialsByEntity(c.Request.Context(), entity.ID, model.CredPassword)
	if err != nil || len(creds) == 0 {
		FailWithCode(c, http.StatusUnauthorized, ErrCodeAuthInvalid, "invalid credentials")
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
		FailWithCode(c, http.StatusUnauthorized, ErrCodeAuthInvalid, "invalid credentials")
		return
	}

	token, err := s.Auth.GenerateToken(entity.ID, entity.EntityType)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to generate token")
		return
	}

	auth.SetAuthCookie(c, token)
	s.attachEntityPublicID(ctx, entity)

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
		FailWithCode(c, http.StatusNotFound, ErrCodeEntityNotFound, "entity not found")
		return
	}
	s.attachEntityPublicID(c.Request.Context(), entity)
	OK(c, http.StatusOK, entity)
}

// HandleRefreshToken issues a new JWT token for the authenticated entity.
func (s *Server) HandleRefreshToken(c *gin.Context) {
	entityID := auth.GetEntityID(c)
	entityType := auth.GetEntityType(c)
	if entityType != model.EntityUser {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "only users can refresh token")
		return
	}
	entity, err := s.Store.GetEntityByID(c.Request.Context(), entityID)
	if err != nil {
		FailWithCode(c, http.StatusUnauthorized, ErrCodeAuthInvalid, "invalid token")
		return
	}
	if entity.Status == "disabled" {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "account is disabled")
		return
	}

	token, err := s.Auth.GenerateToken(entityID, entityType)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to generate token")
		return
	}

	auth.SetAuthCookie(c, token)

	OK(c, http.StatusOK, gin.H{"token": token})
}

// HandleLogout clears the auth cookie and ends the session.
func (s *Server) HandleLogout(c *gin.Context) {
	auth.ClearAuthCookie(c)
	OK(c, http.StatusOK, "logged out")
}

// HandleUpdateProfile updates the authenticated entity's display name and/or avatar.
func (s *Server) HandleUpdateProfile(c *gin.Context) {
	var req struct {
		DisplayName *string `json:"display_name"`
		AvatarURL   *string `json:"avatar_url"`
		Email       *string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	if req.DisplayName == nil && req.AvatarURL == nil && req.Email == nil {
		Fail(c, http.StatusBadRequest, "nothing to update")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	entity, err := s.Store.GetEntityByID(ctx, entityID)
	if err != nil {
		FailWithCode(c, http.StatusNotFound, ErrCodeEntityNotFound, "entity not found")
		return
	}

	if req.DisplayName != nil {
		entity.DisplayName = *req.DisplayName
	}
	if req.AvatarURL != nil {
		entity.AvatarURL = *req.AvatarURL
	}
	if req.Email != nil {
		email := strings.TrimSpace(*req.Email)
		if email != "" {
			existing, _ := s.Store.GetEntityByEmail(ctx, email)
			if existing != nil && existing.ID != entityID {
				FailWithCode(c, http.StatusConflict, ErrCodeDuplicateUser, "email already in use")
				return
			}
		}
		entity.Email = email
	}

	if err := s.Store.UpdateEntity(ctx, entity); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to update profile")
		return
	}
	s.attachEntityPublicID(ctx, entity)

	OK(c, http.StatusOK, entity)
}

// HandleChangePassword changes the authenticated user's password.
func (s *Server) HandleChangePassword(c *gin.Context) {
	if auth.GetEntityType(c) != model.EntityUser {
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "only users can change passwords")
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

	// Validate new password complexity
	if err := validatePassword(req.NewPassword); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
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
		FailWithCode(c, http.StatusForbidden, ErrCodePermDenied, "only users can create users")
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

	// Validate password complexity
	if err := validatePassword(req.Password); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
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
		Metadata:    mustJSONMetadata(map[string]any{entityPublicIDKey: uuid.NewString()}),
	}

	if err := s.Store.CreateEntity(ctx, entity); err != nil {
		FailWithCode(c, http.StatusConflict, ErrCodeDuplicateUser, "username already exists or creation failed")
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
	s.attachEntityPublicID(ctx, entity)

	OK(c, http.StatusCreated, entity)
}

// HandleRegister creates a new user account (public registration).
func (s *Server) HandleRegister(c *gin.Context) {
	var req struct {
		Username    string `json:"username" binding:"required"`
		Password    string `json:"password" binding:"required"`
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "username and password required")
		return
	}

	// Validate password complexity
	if err := validatePassword(req.Password); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	// Check if username already exists
	existing, err := s.Store.GetEntityByName(c.Request.Context(), req.Username, model.EntityUser)
	if err == nil && existing != nil {
		FailWithCode(c, http.StatusConflict, ErrCodeDuplicateUser, "username already exists")
		return
	}

	// Check email uniqueness if provided
	email := strings.TrimSpace(req.Email)
	if email != "" {
		existingByEmail, _ := s.Store.GetEntityByEmail(c.Request.Context(), email)
		if existingByEmail != nil {
			FailWithCode(c, http.StatusConflict, ErrCodeDuplicateUser, "email already registered")
			return
		}
	}

	ctx := c.Request.Context()

	displayName := req.DisplayName
	if displayName == "" {
		displayName = req.Username
	}

	entity := &model.Entity{
		EntityType:  model.EntityUser,
		Name:        req.Username,
		Email:       email,
		DisplayName: displayName,
		Status:      "active",
		Metadata:    mustJSONMetadata(map[string]any{entityPublicIDKey: uuid.NewString()}),
	}

	if err := s.Store.CreateEntity(ctx, entity); err != nil {
		FailWithCode(c, http.StatusConflict, ErrCodeDuplicateUser, "username already exists or creation failed")
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

	// Generate token for auto-login
	token, err := s.Auth.GenerateToken(entity.ID, entity.EntityType)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to generate token")
		return
	}

	auth.SetAuthCookie(c, token)
	s.attachEntityPublicID(ctx, entity)

	OK(c, http.StatusCreated, gin.H{
		"token":  token,
		"entity": entity,
	})
}

func mustJSONMetadata(meta map[string]any) []byte {
	encoded, err := json.Marshal(meta)
	if err != nil {
		return nil
	}
	return encoded
}
