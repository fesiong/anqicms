package route

import (
	"testing"

	"github.com/kataras/iris/v12"
)

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
	Register(app)
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
