package middleware

import (
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/provider"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ParseAdminToken 解析token
func ParseAdminToken(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	tokenString := ctx.GetHeader("admin")
	token, tokenErr := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			// can not parse the token
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(currentSite.TokenSecret + "-admin-token"), nil
	})

	if tokenErr != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusNoLogin,
			"msg":  ctx.Tr("ThisOperationRequiresLogin"),
		})
		return
	} else {
		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			userID, ok := claims["adminId"].(string)
			timeStamp, ok2 := claims["t"].(string)
			if !ok || !ok2 {
				ctx.JSON(iris.Map{
					"code": config.StatusNoLogin,
					"msg":  ctx.Tr("ThisOperationRequiresLogin"),
				})
				return
			}
			sec, _ := strconv.ParseInt(timeStamp, 10, 64)
			if sec < time.Now().Unix() {
				ctx.JSON(iris.Map{
					"code": config.StatusNoLogin,
					"msg":  ctx.Tr("ThisOperationRequiresLogin"),
				})
				return
			}
			// 如果登录过期时间在1小时内，则进行续签，续签只能延长24小时
			if sec < time.Now().Add(1*time.Hour).Unix() {
				adminId, _ := strconv.Atoi(userID)
				newToken := currentSite.GetAdminAuthToken(uint(adminId), false)
				// 下发新token
				ctx.Header("update-token", newToken)
			}
			ctx.Values().Set("adminId", userID)
		} else {
			ctx.JSON(iris.Map{
				"code": config.StatusNoLogin,
				"msg":  ctx.Tr("ThisOperationRequiresLogin"),
			})
			return
		}
	}

	ctx.Next()
}

func ParseAdminUrl(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	if strings.HasPrefix(currentSite.System.AdminUrl, "http") {
		parsedUrl, err := url.Parse(currentSite.System.AdminUrl)
		// 如果解析失败，则跳过
		if err == nil {
			if parsedUrl.Hostname() != library.GetHost(ctx) {
				ctx.JSON(iris.Map{
					"code": config.StatusNoLogin,
					"msg":  ctx.Tr("PleaseUseTheCorrectEntranceToAccess"),
				})
				return
			}
		}
	}

	ctx.Next()
}

func ParseAdminUrlFile(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	if strings.HasPrefix(currentSite.System.AdminUrl, "http") {
		parsedUrl, err := url.Parse(currentSite.System.AdminUrl)
		// 如果解析失败，则跳过
		if err == nil {
			if parsedUrl.Hostname() != library.GetHost(ctx) {
				ctx.StatusCode(http.StatusForbidden)
				return
			}
		}
	}

	ctx.Next()
}

func AdminPermission(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	adminId := ctx.Values().GetUintDefault("adminId", 0)
	if adminId == 1 {
		ctx.Next()
		return
	}
	uri := strings.TrimPrefix(ctx.RequestPath(false), "/system/api")

	// 检查后台对应的前端
	var front string
	for _, g := range config.DefaultMenuGroups {
		exists := false
		for _, m := range g.Menus {
			if m.Backend != "" && strings.HasPrefix(uri, m.Backend) {
				front = m.Path
				exists = true
				break
			}
		}
		if exists {
			break
		}
	}

	// 如果一个链接不在menu里，则不用拦截
	if front == "" {
		ctx.Next()
		return
	}

	admin, err := currentSite.GetAdminInfoById(adminId)
	if err == nil {
		if admin.GroupId == 1 {
			ctx.Next()
			return
		}
		if admin.Group != nil {
			permissions := admin.Group.Setting.Permissions
			for i := range permissions {
				if strings.HasPrefix(front, permissions[i]) {
					ctx.Next()
					return
				}
			}
		}
	}

	ctx.JSON(iris.Map{
		"code": config.StatusNoAccess,
		"msg":  "权限不足。Permission denied.",
	})
}

func Check301(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	uri := ctx.Request().RequestURI
	// 顶级域名301
	host := library.GetHost(ctx)
	if strings.HasSuffix(currentSite.Host, "."+host) {
		// 301 到主域名
		ctx.Redirect(currentSite.System.BaseUrl+uri, 301)
		return
	}
	val := currentSite.GetRedirectFromCache(uri)
	if val != "" {
		// 验证hosts
		if strings.HasPrefix(val, "http") {
			urlParsed, err := url.Parse(val)
			if err == nil && library.GetHost(ctx) == urlParsed.Hostname() && uri == urlParsed.RequestURI() {
				// 相同，跳过
				val = ""
			}
		} else {
			if val == uri {
				val = ""
			} else {
				val = currentSite.GetUrl(val, nil, 0)
			}
		}
		if val != "" {
			ctx.Redirect(val, 301)
			return
		}
	}

	ctx.Next()

}
