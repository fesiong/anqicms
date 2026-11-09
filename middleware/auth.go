package middleware

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/provider"
)

// ParseAdminToken 解析token
func ParseAdminToken(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	tokenString := ctx.GetHeader("admin")
	// 当 header 中没有 token 时（如浏览器原生下载 window.open），
	// fallback 到 query string 的 token 参数。
	if tokenString == "" {
		tokenString = ctx.URLParam("token")
	}
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

func ValidateToken(currentSite *provider.Website, tokenString string) (string, error) {
	if currentSite == nil || len(tokenString) == 0 {
		return "", fmt.Errorf("invalid token")
	}
	token, tokenErr := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			// can not parse the token
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(currentSite.TokenSecret + "-admin-token"), nil
	})

	if tokenErr == nil {
		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			userID, ok := claims["adminId"].(string)
			timeStamp, ok2 := claims["t"].(string)
			if !ok || !ok2 {
				return "", fmt.Errorf("invalid token")
			}
			sec, _ := strconv.ParseInt(timeStamp, 10, 64)
			if sec < time.Now().Unix() {
				return "", fmt.Errorf("invalid token")
			}

			return userID, nil
		} else {
			return "", fmt.Errorf("invalid token")
		}
	}

	return "", fmt.Errorf("invalid token")
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

// AdminMenuPathForURI 把后台 URI 解析成前端菜单路径。
//
// 规则与历史实现完全一致：按 config.MenuGroups() 的顺序遍历，取第一条 Backend
// 是 uri 前缀的菜单，返回它的 Path。命中的菜单没有 Path 时按「无法映射」处理。
//
// 菜单清单是可注入参数（而不是直接读配置）的唯一目的，是让调用方与测试能构造
// 「清单里没有这条 Backend」的场景——那正是严格版权限检查最需要被证明的分支。
func AdminMenuPathForURI(uri string, groups []*config.MenuGroup) (front string, mapped bool) {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return "", false
	}
	for _, group := range groups {
		if group == nil {
			continue
		}
		for _, m := range group.Menus {
			if m == nil || m.Backend == "" {
				continue
			}
			if strings.HasPrefix(uri, m.Backend) {
				if m.Path == "" {
					return "", false
				}
				return m.Path, true
			}
		}
	}
	return "", false
}

// adminPermissionDenied 是后台权限不足的统一回执。
func adminPermissionDenied(ctx iris.Context) {
	ctx.JSON(iris.Map{
		"code": config.StatusNoAccess,
		"msg":  "权限不足。Permission denied.",
	})
}

// adminGroupAllows 校验管理员分组是否拥有 front 菜单的权限前缀。
// 超管（Id==1 / GroupId==1）由调用方先放行，这里只处理分组前缀匹配。
func adminGroupAllows(currentSite *provider.Website, adminId uint, front string) bool {
	admin, err := currentSite.GetAdminInfoById(adminId)
	if err != nil || admin == nil {
		return false
	}
	if admin.GroupId == 1 {
		return true
	}
	if admin.Group == nil {
		return false
	}
	for _, permission := range admin.Group.Setting.Permissions {
		if strings.HasPrefix(front, permission) {
			return true
		}
	}
	return false
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
	front, mapped := AdminMenuPathForURI(uri, config.MenuGroups())

	// 如果一个链接不在menu里，则不用拦截（历史行为，保持不变）。
	// 需要「未知菜单一律拒绝」的端点在 party 上改用 AdminPermissionRequired。
	if !mapped {
		ctx.Next()
		return
	}

	if adminGroupAllows(currentSite, adminId, front) {
		ctx.Next()
		return
	}

	adminPermissionDenied(ctx)
}

// AdminPermissionRequired 是严格版后台权限检查：URI 无法映射到任何菜单 Backend 时一律拒绝。
//
// 与 AdminPermission 的唯一差别就是「未映射」分支：后台历史行为对不在菜单里的路径是放行的，
// 这对日常页面无害，但对渠道配置这类会签发绑定 token、能改平台凭据的端点，等于把授权
// 寄托在「恰好没人注册这条路由」上。规格 §10 要求渠道端点显式授权，所以这里默认拒绝。
func AdminPermissionRequired(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	adminId := ctx.Values().GetUintDefault("adminId", 0)
	if adminId == 1 {
		ctx.Next()
		return
	}
	uri := strings.TrimPrefix(ctx.RequestPath(false), "/system/api")

	front, mapped := AdminMenuPathForURI(uri, config.MenuGroups())
	if !mapped {
		adminPermissionDenied(ctx)
		return
	}

	if adminGroupAllows(currentSite, adminId, front) {
		ctx.Next()
		return
	}

	adminPermissionDenied(ctx)
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
