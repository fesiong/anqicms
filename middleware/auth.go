package middleware

import (
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/provider"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ParseAdminToken 解析token
func ParseAdminToken(ctx iris.Context) {
	tokenString := ctx.GetHeader("admin")
	token, tokenErr := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			// can not parse the token
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(config.JsonData.Server.TokenSecret), nil
	})

	if tokenErr != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusNoLogin,
			"msg":  "该操作需要登录，请登录后重试",
		})
		return
	} else {
		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			userID, ok := claims["adminId"].(string)
			timeStamp, ok2 := claims["t"].(string)
			if !ok || !ok2 {
				ctx.JSON(iris.Map{
					"code": config.StatusNoLogin,
					"msg":  "该操作需要登录，请登录后重试",
				})
				return
			}
			sec, _ := strconv.ParseInt(timeStamp, 10, 64)
			if sec < time.Now().Unix() {
				ctx.JSON(iris.Map{
					"code": config.StatusNoLogin,
					"msg":  "该操作需要登录，请登录后重试",
				})
				return
			}
			ctx.Values().Set("adminId", userID)
		} else {
			ctx.JSON(iris.Map{
				"code": config.StatusNoLogin,
				"msg":  "该操作需要登录，请登录后重试",
			})
			return
		}
	}

	ctx.Next()
}

func ParseAdminUrl(ctx iris.Context) {
	if strings.HasPrefix(config.JsonData.System.AdminUrl, "http") {
		parsedUrl, err := url.Parse(config.JsonData.System.AdminUrl)
		// 如果解析失败，则跳过
		if err == nil {
			if parsedUrl.Host != ctx.Host() {
				ctx.JSON(iris.Map{
					"code": config.StatusNoLogin,
					"msg":  "请使用正确的入口访问。 Please use the correct entry to visit.",
				})
				return
			}
		}
	}

	ctx.Next()
}

func AdminPermission(ctx iris.Context) {
	adminId := uint(ctx.Values().GetIntDefault("adminId", 0))
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

	admin, err := provider.GetAdminInfoById(adminId)
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

func FrontendCheck(ctx iris.Context) {
	uri := ctx.Request().RequestURI

	// 如果有后台域名，则后台后台将链接跳转到后台
	if strings.HasPrefix(config.JsonData.System.AdminUrl, "http") {
		parsedUrl, err := url.Parse(config.JsonData.System.AdminUrl)
		// 如果解析失败，则跳过
		if err == nil {
			if parsedUrl.Host == ctx.Host() && !strings.HasPrefix(uri, "/system") {
				// 来自后端的域名，但访问的不是后端的业务，则强制跳转到后端。
				ctx.Redirect(strings.TrimRight(config.JsonData.System.AdminUrl, "/") + "/system")
				return
			}
		}
	}

	if dao.DB == nil && !strings.HasPrefix(uri, "/static") && !strings.HasPrefix(uri, "/install") {
		ctx.Redirect("/install")
		return
	}

	ctx.Next()
}

func Check301(ctx iris.Context) {
	uri := ctx.Request().RequestURI
	val := provider.GetRedirectFromCache(uri)
	if val != "" {
		// 验证hosts
		if strings.HasPrefix(val, "http") {
			urlParsed, err := url.Parse(val)
			if err == nil && ctx.Host() == urlParsed.Host && uri == urlParsed.RequestURI() {
				// 相同，跳过
				val = ""
			}
		} else {
			if val == uri {
				val = ""
			} else {
				val = provider.GetUrl(val, nil, 0)
			}
		}
		if val != "" {
			ctx.Redirect(val, 301)
			return
		}
	}

	ctx.Next()

}
