package controller

import (
	"os"
	"regexp"
	"testing"
)

// 2026-10-28 加固：GET /api/verify/email?state=exist 原来回 {id, email}。
//
// id 不是无害的内部编号——它是「邮箱 → 站内 user id」的关联键，而
// GET /api/comment/list?user_id=X 与 GET /api/review/list?user_id=X 都不需要登录，
// 于是「知道邮箱 → 拿到 id → 读出这个人所有公开评论/评价」形成去匿名化链路。
// email 回显更是零信息量（输入就是自己给的）。
func TestVerifyEmailExistDoesNotLeakUserIdentity(t *testing.T) {
	src := readFuncSource(t, "apiUser.go", "ApiVerifyEmail")

	mustNotContain(t, src, `"id":\s*user\.Id`,
		"state=exist 不得回 user.Id：它可以拼出 /api/comment/list?user_id= 的去匿名化链路")
	mustNotContain(t, src, `"email":\s*user\.Email`,
		"state=exist 不得回显 email（输入本来就是自己给的）")
	mustContain(t, src, `"exist":\s*true`,
		"state=exist 应只回一个布尔值，且 code 语义保持不变（模板靠 code 判断已注册）")
}

// 只删 data 字段不算修好：三个探针都能判断「邮箱是否已注册」，
// 必须共用一份按 IP 的预算，否则攻击者在 exist / 发信 / 注册之间切换即可绕过单点限制。
// 这是源码级护栏：中间件写了但没挂到路由上，等于没写（原密码重置漏洞正是这种形态）。
func TestAccountProbeRoutesAreRateLimited(t *testing.T) {
	src, err := os.ReadFile("../route/base.go")
	if err != nil {
		t.Fatalf("读取 ../route/base.go 失败：%v", err)
	}
	route := string(src)

	probes := []struct {
		desc    string
		pattern string
	}{
		{"GET /api/verify/email（state=exist 枚举）", `api\.Get\("/verify/email",\s*middleware\.AccountProbeGuard,`},
		{"POST /api/verify/email（触发发信）", `api\.Post\("/verify/email",\s*middleware\.AccountProbeGuard,`},
		{"POST /api/register（重复邮箱报错）", `api\.Post\("/register",\s*middleware\.AccountProbeGuard,`},
	}
	for _, p := range probes {
		if !regexp.MustCompile(p.pattern).MatchString(route) {
			t.Errorf("%s 未挂 middleware.AccountProbeGuard，该探针可以无限次探测", p.desc)
		}
	}
}
