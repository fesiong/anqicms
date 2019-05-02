package middleware

import (
	"fmt"

	"goblog/config"

	"github.com/gin-gonic/gin"
)

// RefreshTokenCookie 刷新过期时间
func RefreshTokenCookie(c *gin.Context) {
	tokenString, err := c.Cookie("token")
	fmt.Println(err)
	if tokenString != "" && err == nil {
		c.SetCookie("token", tokenString, config.ServerConfig.TokenMaxAge, "/", "", true, true)
	}
	c.Next()
}
