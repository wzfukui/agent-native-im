package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wzfukui/agent-native-im/internal/model"
)

type createBotRequest struct {
	Name string `json:"name" binding:"required"`
}

func (s *Server) HandleCreateBot(c *gin.Context) {
	var req createBotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "name is required")
		return
	}

	userID := c.GetInt64("userID")
	token := uuid.New().String()

	bot := &model.Bot{
		OwnerID: userID,
		Name:    req.Name,
		Token:   token,
		Status:  "active",
	}

	if err := s.Store.CreateBot(c.Request.Context(), bot); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to create bot")
		return
	}

	OK(c, http.StatusCreated, gin.H{
		"id":    bot.ID,
		"name":  bot.Name,
		"token": token, // shown only once
	})
}

func (s *Server) HandleListBots(c *gin.Context) {
	userID := c.GetInt64("userID")

	bots, err := s.Store.ListBotsByOwner(c.Request.Context(), userID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to list bots")
		return
	}

	OK(c, http.StatusOK, bots)
}

func (s *Server) HandleDeleteBot(c *gin.Context) {
	userID := c.GetInt64("userID")
	botID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid bot id")
		return
	}

	if err := s.Store.DeleteBot(c.Request.Context(), botID, userID); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to delete bot")
		return
	}

	OK(c, http.StatusOK, "bot deleted")
}

func (s *Server) HandleBotMe(c *gin.Context) {
	botID := c.GetInt64("botID")

	bot, err := s.Store.GetBotByID(c.Request.Context(), botID)
	if err != nil {
		Fail(c, http.StatusNotFound, "bot not found")
		return
	}

	OK(c, http.StatusOK, bot)
}
