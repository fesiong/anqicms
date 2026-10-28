package controller

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"testing"
)

// 2026-10-28 修复的未授权密码重置漏洞（POST /api/password/reset）：
// 邮件验证码校验是空分支，唯一凭据是 Md5(email+password)；空密码账号
// （Google / 公众号 / 邮箱验证码注册）凭据退化为 Md5(email)，知道邮箱即可改密。
// 下面的断言是源码级护栏：只测 ResetPasswordToken 本身锁不住「handler 忘了用」，
// 而那正是原缺陷的形态（函数是对的，调用点没接上）。
func TestResetPasswordRejectsUnverifiableToken(t *testing.T) {
	src := readFuncSource(t, "apiUser.go", "ApiResetUserPassword")

	mustNotContain(t, src, `library\.Md5\(`,
		"重置凭据不得再由 handler 自行拼算（密码哈希参与拼算 = 空密码时等于 Md5(email)）")
	mustContain(t, src, `ResetPasswordToken\(\s*currentSite\.TokenSecret`,
		"校验凭据必须调用 provider.ResetPasswordToken 并混入站点 TokenSecret")
	mustContain(t, src, `CodeCache\.Verify\(\s*req\.Email\s*,\s*req\.Code`,
		"必须真正校验邮件验证码，且缓存 key 用 req.Email（CodeCache 以 email 为 key，不是 token）")
}

// 发信端与校验端必须是同一个算法，否则正常用户点邮件链接后永远重置失败。
func TestSendVerifyEmailUsesSameTokenFunc(t *testing.T) {
	src := readFuncSource(t, "../provider/sendmail.go", "SendVerifyEmail")

	mustNotContain(t, src, `library\.Md5\(\s*user\.Email\s*\+\s*user\.Password`,
		"发信端仍在用 Md5(email+password) 拼凭据")
	mustContain(t, src, `ResetPasswordToken\(\s*w\.TokenSecret`,
		"发信端必须调用 provider.ResetPasswordToken，与校验端保持一致")
}

// ApiVerifyEmail 的 state=reset 只是「跳到重置页」，验证码要留给随后
// POST /api/password/reset 消费。这里若提前 clear，P1 的校验必然失败。
func TestVerifyEmailKeepsResetCodeForSubmit(t *testing.T) {
	src := readFuncSource(t, "apiUser.go", "ApiVerifyEmail")

	mustNotContain(t, src, `CodeCache\.Get\(\s*email\s*,\s*true\s*\)`,
		"Get(email, true) 会在跳转前消费掉验证码，导致重置提交永远校验失败；应按 state 决定是否 clear")
	mustContain(t, src, `CodeCache\.Get\(\s*email\s*,\s*state\s*!=\s*"reset"\s*\)`,
		"state=reset 必须保留验证码（其余 state 仍保持一次性消费）")
}

// 凭据与验证码都是一次性的：改密成功必须消费，否则同一封邮件可以重复改密。
func TestResetPasswordConsumesCodeAfterSuccess(t *testing.T) {
	src := readFuncSource(t, "apiUser.go", "ApiResetUserPassword")

	mustContain(t, src, `CodeCache\.Delete\(\s*req\.Email\s*\)`,
		"改密成功后必须消费验证码（key 是 email）")
}

func readFuncSource(t *testing.T, file, funcName string) string {
	t.Helper()

	src, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("读取 %s 失败：%v", file, err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, parser.ParseComments)
	if err != nil {
		t.Fatalf("解析 %s 失败：%v", file, err)
	}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != funcName {
			continue
		}
		return string(src[fset.Position(fn.Pos()).Offset:fset.Position(fn.End()).Offset])
	}
	t.Fatalf("%s 中找不到函数 %s", file, funcName)

	return ""
}

func mustContain(t *testing.T, src, pattern, why string) {
	t.Helper()
	if !regexp.MustCompile(pattern).MatchString(src) {
		t.Errorf("%s（期望命中 /%s/）", why, pattern)
	}
}

func mustNotContain(t *testing.T, src, pattern, why string) {
	t.Helper()
	if regexp.MustCompile(pattern).MatchString(src) {
		t.Errorf("%s（不应命中 /%s/）", why, pattern)
	}
}
