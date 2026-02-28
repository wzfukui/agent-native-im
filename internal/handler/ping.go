package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func HandlePing(c *gin.Context) {
	OK(c, http.StatusOK, "pong")
}
