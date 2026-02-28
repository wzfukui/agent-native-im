package handler

import "github.com/gin-gonic/gin"

func OK(c *gin.Context, code int, data interface{}) {
	c.JSON(code, gin.H{"ok": true, "data": data})
}

func Fail(c *gin.Context, code int, msg string) {
	c.JSON(code, gin.H{"ok": false, "error": msg})
}
