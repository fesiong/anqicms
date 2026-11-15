package route

import (
	"embed"
	"testing"

	"github.com/kataras/iris/v12"
)

// routeTestSystemFiles 是测试用的最小「后台静态资源」内嵌目录。
// HandleDir 要求传入的 embed.FS 根下恰好有目录（iris 会 fs.Sub 到第一项），
// 空 FS 会直接 panic，所以必须放一个真实文件。
//
//go:embed testdata/embedroot
var routeTestSystemFiles embed.FS

// 本文件是 IM 渠道后台 API 的「真实路由表」回归锁（T12b R6）。
//
// 为什么不能用 AST 断言：终审曾报「`/channels/` 与 `/channels` 不一致」。
// 实测否证 —— Iris 的 cleanPath 会去掉尾斜杠再注册，`Party("/channels").Get("/")`
// 实际注册成 `/system/api/channels`；请求无斜杠返回 200，带斜杠返回 301 到无斜杠版本。
// 所以唯一可靠的断言对象是 Register() 之后的真实路由表，而不是源码里的字符串拼接。
//
// 反向对照：把期望列表里的路径改成带尾斜杠的写法（或改成 `channels.Get("/")`
// 之外会改变路径的注册方式），本测试立即变红。

// registeredRoutes 返回 Register() 之后的真实路由集合（方法 + 完整路径）。
func registeredRoutes(t *testing.T) map[string]bool {
	t.Helper()
	app := iris.New()
	// Register 现为双参数签名（bootstrap.go 传 main 包内嵌的 SystemFiles），
	// 测试里传入本文件内嵌的最小目录即可——本用例只断言路由表，不请求静态资源。
	Register(app, routeTestSystemFiles)
	app.Build()
	routes := map[string]bool{}
	for _, item := range app.GetRoutes() {
		if item.Method == "" {
			continue
		}
		routes[item.Method+" "+item.Path] = true
	}
	return routes
}

func TestChannelRoutesRegisteredExactPaths(t *testing.T) {
	routes := registeredRoutes(t)

	// 精确匹配，不用 TrimRight 推断：锁的必须是真实注册结果。
	want := []string{
		"GET /system/api/channels",
		"GET /system/api/channels/records",
		"POST /system/api/channels/test",
	}
	for _, route := range want {
		if !routes[route] {
			t.Errorf("路由表里没有 %s（渠道后台 API 的注册路径变了，前端 baseUrl=/system/api 会 404）", route)
		}
	}

	// 尾斜杠版本不得被注册成独立路由：它只是 cleanPath 之后的同一端点。
	// 这条同时证明上面的精确匹配有分辨力（否则「带不带斜杠都算存在」会让锁失效）。
	for _, route := range []string{
		"GET /system/api/channels/",
		"GET /system/api/channels/records/",
		"POST /system/api/channels/test/",
	} {
		if routes[route] {
			t.Errorf("路由表里出现了尾斜杠版本 %s：Iris cleanPath 应把它归一到无斜杠路径", route)
		}
	}
}

// TestSmsRoutesRegisteredExactPaths 锁住短信相关的 7 条真实路由。
//
// 背景：源码级护栏只间接提到 /test 与 /logs，/plugin/sms/setting 的 GET/POST
// 以及三条前台端点此前没有任何 route 层断言——把注册删掉 go test 依然全绿。
// 这里断言 Register() 之后的真实路由表（方法 + 完整路径），而不是源码字符串。
//
// 反向对照：注释掉 `sms.Get("/setting", ...)` → 本用例必红。
func TestSmsRoutesRegisteredExactPaths(t *testing.T) {
	routes := registeredRoutes(t)

	want := []string{
		// 后台配置（AdminPermissionRequired）
		"GET /system/api/plugin/sms/setting",
		"POST /system/api/plugin/sms/setting",
		"POST /system/api/plugin/sms/test",
		"GET /system/api/plugin/sms/logs",
		// 前台发码 / 找回密码 / 换绑
		"POST /api/sms/send",
		"POST /api/sms/password/reset",
		"POST /api/sms/phone/bind",
	}
	for _, route := range want {
		if !routes[route] {
			t.Errorf("路由表里没有 %s（删掉注册也不会变红，前端会 404）", route)
		}
	}

	// 方法必须对得上：写成 GET 的写端点或 POST 的读端点同样会让前端 404/405。
	for _, route := range []string{
		"GET /system/api/plugin/sms/test",
		"POST /system/api/plugin/sms/logs",
		"GET /api/sms/send",
		"GET /api/sms/phone/bind",
	} {
		if routes[route] {
			t.Errorf("路由表里出现了错误方法的 %s", route)
		}
	}
}
