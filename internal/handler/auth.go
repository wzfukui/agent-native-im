package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
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

	user, err := s.Store.GetUserByUsername(c.Request.Context(), req.Username)
	if err != nil {
		Fail(c, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		Fail(c, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := s.Auth.GenerateToken(user.ID, user.Username)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to generate token")
		return
	}

	OK(c, http.StatusOK, gin.H{
		"token": token,
		"user":  user,
	})
}
