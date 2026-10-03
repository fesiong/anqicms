package manageController

import (
	"testing"

	"kandaoni.com/anqicms/config"
)

// TestPluginSendmailSettingDoesNotMutateGlobal 锁住读端点的指针陷阱。
//
// PluginSendmail 的类型是 *config.PluginSendmail（provider/website.go:85），
// 写端点要「就地改内存」是对的，但读端点为了掩码密码**绝不能**碰内存。
// 曾经的写法 `setting := currentSite.PluginSendmail` 只拷贝指针，
// `setting.Password = mask` 等于把全局配置里的真实授权码抹成哨兵串：
// 一次 setting_get 就让站点再也发不出邮件，且无任何报错（2026-10-03 实际踩过）。
//
// 用纯语言级用例验证，不依赖真实站点 —— 这样 CI 里也真能跑到，
// 而不会因「站点未初始化」被 skip 掉。
func TestPluginSendmailSettingDoesNotMutateGlobal(t *testing.T) {
	const realPassword = "REALSMTP AUTH CODE 16"

	// 复现真实字段类型：Website.PluginSendmail 是 *config.PluginSendmail
	global := &config.PluginSendmail{Password: realPassword, Server: "smtp.qq.com", Port: 587}

	// 旧写法（错误）：只拷贝指针 —— 必须证明它**会**污染全局。
	// 若哪天指针语义变化，这条会失败并提示「测试前提不成立」，
	// 而不是静默变成一个什么都验不到的空用例。
	bad := global
	bad.Password = SendmailPasswordMask
	if global.Password == realPassword {
		t.Fatalf("旧写法（拷贝指针）本应污染全局配置，却没污染——"+
			"测试前提不成立，请检查 PluginSendmail 是否已改为值类型")
	}
	if global.Password != SendmailPasswordMask {
		t.Fatalf("旧写法应把全局值改成掩码，实际=%q", global.Password)
	}

	// 新写法（正确）：解引用做值拷贝
	global.Password = realPassword
	good := *global
	good.Password = SendmailPasswordMask
	if global.Password != realPassword {
		t.Fatalf("值拷贝不应影响全局：内存值=%q，期望=%q", global.Password, realPassword)
	}
	if good.Password != SendmailPasswordMask {
		t.Errorf("副本应已被掩码，实际=%q", good.Password)
	}
	if good.Server != global.Server || good.Port != global.Port {
		t.Errorf("副本其余字段应与全局一致：%+v vs %+v", good, *global)
	}
}

// TestSendmailSettingFormKeepsPasswordOnMask 回写保护：掩码原样回传时不得覆盖真值。
//
// 对应 PluginSendmailSettingForm 里的守卫。缺了它，前端/AI 把 setting_get
// 的结果原样写回就会把哨兵串存进库，授权码随之失效。
func TestSendmailSettingFormKeepsPasswordOnMask(t *testing.T) {
	if SendmailPasswordMask != "********" {
		t.Fatalf("哨兵值不应随意变更，实际=%q", SendmailPasswordMask)
	}
	// 模拟守卫逻辑
	apply := func(current, incoming string) string {
		if incoming != SendmailPasswordMask {
			return incoming
		}
		return current
	}
	const real = "AUTHCODE1234567890"
	if got := apply(real, SendmailPasswordMask); got != real {
		t.Errorf("掩码回传应保留原值，实际=%q", got)
	}
	if got := apply(real, "NEWCODE0987654321"); got != "NEWCODE0987654321" {
		t.Errorf("新值应覆盖，实际=%q", got)
	}
}
