package provider

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/kataras/iris/v12"
)

// TestIrisInProcInvokeFeasibility 锁定「进程内直调 iris 路由」这一技术地基的可行性。
//
// 用途：通用 REST 对接层（G 阶段）计划用 bootstrap.Application.ServeHTTP 在进程内
// 调用后台 REST 端点，从而避免两件事：
//  1. 为 394 个端点手写 cap（F 阶段已证明 caps 是 REST 的平行手写实现，无调用链可循）
//  2. 走真实 HTTP 回环（依赖端口监听、DNS、网络栈）
//
// 本测试验证四个必要条件（任一不成立则整条路线作废）：
//   - party 中间件链会执行        → 意味着 ParseAdminToken / ParseAdminUrl 等鉴权仍然生效
//   - 能注入 admin header         → 意味着可用 w.GetAdminAuthToken() 构造合法管理员身份
//   - 伪造 Host 能生效            → 意味着可用于多站点的 CurrentSite 匹配与 ParseAdminUrl 域名校验
//   - 路由 + handler 正常返回     → 意味着复用现有 REST 业务逻辑，零重复实现
//
// 已确认的两个硬约束（改变 iris 大版本时需重新验证）：
//  1. Router 必须先 Build，否则 router.mainHandler 为 nil，ServeHTTP 直接 panic。
//     生产环境 app.Run() 已完成 Build，故运行时调用安全。
//  2. 多站点场景下必须按目标站点伪造 Host，否则 CurrentSite 会回落到默认站点，
//     并且 ParseAdminUrl 的域名校验会以 未登录 拒绝请求。
func TestIrisInProcInvokeFeasibility(t *testing.T) {
	app := iris.New()

	var mwExecuted, adminHeaderRead, hostSpoofed, handlerExecuted bool

	// 复刻 anqicms 的 party 结构：/system/api/... 并挂中间件，
	// 以验证注入层 middleware 的行为与真实链路一致。
	system := app.Party("/system")
	manage := system.Party("/api", func(ctx iris.Context) {
		mwExecuted = true
		// 模拟 ParseAdminToken：从 "admin" header 取 JWT
		if ctx.GetHeader("admin") == "fake-jwt" {
			adminHeaderRead = true
		}
		// 模拟 ParseAdminUrl / CurrentSite：按 host 识别站点
		hostSpoofed = ctx.Host() == "admin.example.com"
		ctx.Next()
	})
	manage.Get("/ping", func(ctx iris.Context) {
		handlerExecuted = true
		ctx.JSON(iris.Map{"code": 0, "msg": "pong"})
	})

	// 硬约束 1：必须先 Build，否则 ServeHTTP 内 mainHandler 为 nil → panic
	if err := app.Build(); err != nil {
		t.Fatalf("Build 失败: %v", err)
	}

	// 硬约束 2：Host 必须伪装成目标站点，才能让多站点识别与入口域名校验通过
	req := httptest.NewRequest("GET", "/system/api/ping", nil)
	req.Host = "admin.example.com"
	req.Header.Set("admin", "fake-jwt")
	rec := httptest.NewRecorder()

	// Application 内嵌 *router.Router，ServeHTTP 被提升，故可作为 http.Handler 直接使用
	app.ServeHTTP(rec, req)

	fmt.Printf("\n=== 进程内直调可行性验证 ===\n")
	fmt.Printf("HTTP 状态码      : %d\n", rec.Code)
	fmt.Printf("响应体           : %s\n", rec.Body.String())
	fmt.Printf("中间件是否执行   : %v\n", mwExecuted)
	fmt.Printf("读到 admin header: %v\n", adminHeaderRead)
	fmt.Printf("伪造 Host 生效   : %v\n", hostSpoofed)
	fmt.Printf("handler 是否执行 : %v\n", handlerExecuted)

	if rec.Code != 200 {
		t.Fatalf("期望 200，实得 %d / body=%s", rec.Code, rec.Body.String())
	}
	if !mwExecuted {
		t.Fatal("party 中间件链未执行：鉴权将无法生效")
	}
	if !adminHeaderRead {
		t.Fatal("admin header 未透传：无法构造管理员身份")
	}
	if !hostSpoofed {
		t.Fatal("伪造 Host 未生效：多站点场景会命中错误的站点")
	}
	if !handlerExecuted {
		t.Fatal("handler 未执行：无法复用现有 REST 业务逻辑")
	}
}
