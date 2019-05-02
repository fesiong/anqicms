package middleware

import (
	"errors"
	"fmt"

	"goblog/config"
	"goblog/controller/common"
	"goblog/model"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
)

func getUser(c *gin.Context) (model.User, error) {
	var user model.User
	tokenString := c.GetHeader("token")

	token, tokenErr := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(config.ServerConfig.TokenSecret), nil
	})

	if tokenErr != nil {
		return user, errors.New("未登录")
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		userID := int(claims["id"].(float64))
		err := model.DB.First(&user, userID).Error
		if err != nil {
			return user, errors.New("未登录")
		}
		return user, nil
	}
	return user, errors.New("未登录")
}

// SetContextUser 给 context 设置 user
func SetContextUser(c *gin.Context) {
	var user model.User
	var err error
	if user, err = getUser(c); err != nil {
		c.Set("user", nil)
		c.Next()
		return
	}
	c.Set("user", user)
	c.Next()
}

// SigninRequired 必须是登录用户
func SigninRequired(c *gin.Context) {
	SendErrJSON := common.SendErrJSON
	var user model.User
	var err error
	if user, err = getUser(c); err != nil {
		SendErrJSON("未登录", model.ErrorCode.LoginTimeout, c)
		return
	}
	c.Set("user", user)
	c.Next()
}

// AdminRequired 必须是管理员
func AdminRequired(c *gin.Context) {
	SendErrJSON := common.SendErrJSON
	var user model.User
	var err error
	if user, err = getUser(c); err != nil {
		SendErrJSON("未登录", model.ErrorCode.LoginTimeout, c)
		return
	}
	if user.IsAdmin == 1 {
		c.Set("user", user)
		c.Next()
	} else {
		SendErrJSON("没有权限", c)
	}
}

func CORSMiddleware(c *gin.Context) {
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
	c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, Api, Token")
	c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, UPDATE")

	if c.Request.Method == "OPTIONS" {
		c.AbortWithStatus(204)
		return
	}

	c.Next()
}
