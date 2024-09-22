package middleware

import (
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"strconv"
	"time"
)

func ParseUserToken(ctx iris.Context) {
	// 允许 API 重新设置siteID
	tmpSiteId := ctx.URLParamIntDefault("site_id", 0)
	if tmpSiteId > 0 {
		tmpSite := provider.GetWebsite(uint(tmpSiteId))
		if tmpSite != nil {
			ctx.Values().Set("siteId", tmpSite.Id)
		}
	}

	currentSite := provider.CurrentSite(ctx)
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
		return []byte(currentSite.TokenSecret + "-user-token"), nil
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
					userInfo, err := currentSite.GetUserInfoById(uint(id))
					if err == nil {
						ctx.Values().Set("userId", userID)
						ctx.Values().Set("userInfo", userInfo)

						userGroup, _ := currentSite.GetUserGroupInfo(userInfo.GroupId)
						ctx.Values().Set("userGroup", userGroup)
						// set data to view
						ctx.ViewData("userGroup", userGroup)
						ctx.ViewData("userInfo", userInfo)
					}
					// 如果登录过期时间在1小时内，则进行续签，续签只能延长24小时
					if sec < time.Now().Add(1*time.Hour).Unix() {
						newToken := currentSite.GetUserAuthToken(uint(id), false)
						// 下发新token
						ctx.Header("update-token", newToken)
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
			"msg":  ctx.Tr("ThisOperationRequiresLogin"),
		})
		return
	}

	ctx.Next()
}
