package provider

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// 需求（2026-10-27）：
//   - 没绑定第三方账号的创建（后台/AI 的 user_create，用户名由调用方自选）→ 重名不允许，直接报错；
//   - 绑定第三方账号的创建（Google / 公众号 / 小程序自动注册，用户名取自平台昵称，用户无法自选）
//     → 先查重，重复时追加数字后缀：张三、张三2、张三3……
//
// nextUniqueUserName 是这套后缀规则里唯一可脱库测试的部分，所以单独抽成纯函数：
// 调用方传入"这个名字是否被占用"的回调，便于用假数据覆盖边界。
func TestNextUniqueUserName(t *testing.T) {
	cases := []struct {
		name  string
		base  string
		taken map[string]bool
		want  string
	}{
		{"未占用时原样返回", "张三", nil, "张三"},
		{"占用时追加2", "张三", map[string]bool{"张三": true}, "张三2"},
		{"连续占用递进到3", "张三", map[string]bool{"张三": true, "张三2": true}, "张三3"},
		{"中间空缺优先复用", "张三", map[string]bool{"张三": true, "张三3": true}, "张三2"},
		{"空昵称退化为默认名", "", nil, "用户"},
		{"纯空白等同空昵称", "   ", map[string]bool{"用户": true}, "用户2"},
		{"首尾空白先裁掉再查重", "  张三  ", map[string]bool{"张三": true}, "张三2"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			taken := c.taken
			got := nextUniqueUserName(c.base, func(s string) bool { return taken[s] })
			if got != c.want {
				t.Fatalf("base=%q taken=%v → %q，期望 %q", c.base, taken, got, c.want)
			}
		})
	}

	// model.User.UserName 是 varchar(64)：后缀最多 4 位，所以 base 必须截到 60 个字符以内，
	// 否则 MySQL 严格模式下直接写库失败。
	t.Run("超长昵称压到字段长度内", func(t *testing.T) {
		base := strings.Repeat("长", 80)
		got := nextUniqueUserName(base, func(string) bool { return false })
		if n := len([]rune(got)); n > 64 {
			t.Fatalf("长度 %d 超过 varchar(64)：%q", n, got)
		}
		truncated := strings.Repeat("长", 60)
		got2 := nextUniqueUserName(base, func(s string) bool { return s == truncated })
		if n := len([]rune(got2)); n > 64 {
			t.Fatalf("加后缀后长度 %d 超过 varchar(64)：%q", n, got2)
		}
		if !strings.HasSuffix(got2, "2") {
			t.Fatalf("被占用时应追加后缀，实际 %q", got2)
		}
	})

	t.Run("后缀全被占用也不返回重复值", func(t *testing.T) {
		got := nextUniqueUserName("张三", func(string) bool { return true })
		if got == "张三" {
			t.Fatalf("仍返回了被占用的原值")
		}
		if n := len([]rune(got)); n > 64 {
			t.Fatalf("兜底值长度 %d 超过 varchar(64)：%q", n, got)
		}
	})
}

// 源码级护栏：三条第三方自动注册路径的用户名都直接来自平台昵称，
// 绕过 uniqueUserName 就等于同一昵称的两个微信用户落成同名记录；
// 而 user_name 是「按用户名登录」的凭据（LoginViaPassword 的 user_name 分支），
// 重名会让人登进别人的账号。线上库现有 3 条空用户名记录就是这条路径留下的。
func TestThirdPartyRegistrationDedupesUserName(t *testing.T) {
	raw, err := os.ReadFile("user.go")
	if err != nil {
		t.Fatalf("读取 user.go 失败：%v", err)
	}
	src := string(raw)

	// 直接赋值（未去重）的写法：struct 字面量字段与普通赋值都要抓住，
	// 所以只匹配「字段名 + 等号 + 平台昵称」，不依赖对齐空白。
	rawAssign := regexp.MustCompile(`UserName:\s+(wecahtUserInfo\.Nickname|googleUser\.Name|userWechat\.Nickname)\b`)
	if hit := rawAssign.FindAllString(src, -1); len(hit) > 0 {
		t.Fatalf("第三方昵称被直接写成用户名（未经去重）：%v", hit)
	}
	noDedupe := regexp.MustCompile(`user\.UserName\s*=\s*(wecahtUserInfo\.Nickname|googleUser\.Name|userWechat\.Nickname)\b`)
	if hit := noDedupe.FindAllString(src, -1); len(hit) > 0 {
		t.Fatalf("第三方昵称被直接覆盖用户名（未经去重）：%v", hit)
	}

	// 反证：确认这三处确实存在，否则上面的断言只是空转。
	for _, want := range []string{
		"uniqueUserName(wecahtUserInfo.Nickname",
		"uniqueUserName(googleUser.Name",
		"uniqueUserName(userWechat.Nickname",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("没找到去重调用 %q：护栏空转，请检查第三方注册路径", want)
		}
	}
}
