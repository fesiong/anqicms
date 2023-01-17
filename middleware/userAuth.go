package middleware

import (
	"fmt"
	"github.com/golang-jwt/jwt/v4"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"strconv"
	"time"
)

func ParseUserToken(ctx iris.Context) {
	tokenString := ctx.GetHeader("token")
	if tokenString == "" {
		// read from cookies
		tokenString = ctx.GetCookie("token")
	}

	token, tokenErr := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			// can not parse the token
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(config.Server.Server.TokenSecret), nil
	})

	if tokenErr == nil {
		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			userID, ok := claims["userId"].(string)
			timeStamp, ok2 := claims["t"].(string)
			if ok && ok2 {
				sec, _ := strconv.ParseInt(timeStamp, 10, 64)
				if sec >= time.Now().Unix() {
					// 转换成 int
					id, _ := strconv.Atoi(userID)
					userInfo, err := provider.GetUserInfoById(uint(id))
					if err == nil {
						ctx.Values().Set("userId", userID)
						ctx.Values().Set("userInfo", userInfo)

						userGroup, _ := provider.GetUserGroupInfo(userInfo.GroupId)
						ctx.Values().Set("userGroup", userGroup)
						// set data to view
						ctx.ViewData("userGroup", userGroup)
						ctx.ViewData("userInfo", userInfo)
					}
				}
			}
		}
	}

	ctx.Next()
}

func UserAuth(ctx iris.Context) {
	userId := ctx.Values().GetUintDefault("userId", 0)
	if userId == 0 {
		ctx.JSON(iris.Map{
			"code": config.StatusNoLogin,
			"msg":  config.Lang("该操作需要登录，请登录后重试"),
		})
		return
	}

	ctx.Next()
}
