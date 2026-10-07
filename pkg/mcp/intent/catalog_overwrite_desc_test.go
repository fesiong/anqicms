package intent

import (
	"strings"
	"testing"
)

// TestThirdpartySaveDescWarnsFullOverwrite *_save 是全量覆盖语义，
// desc 必须写明，否则 AI 只想改一个字段却把其余凭证清空
// （实测：只传 app_id 会把 app_secret/token 清成空串）。
//
// 这类「参数语义」写在 desc 里比写在代码注释里有用——AI 只读 desc。
func TestThirdpartySaveDescWarnsFullOverwrite(t *testing.T) {
	spec := mustSpec(t, "channel_thirdparty")
	if !strings.Contains(spec.Desc, "全量覆盖") {
		t.Errorf("desc 应说明全量覆盖语义，实际=%q", spec.Desc)
	}
	// 应指引「先 get 再整体写回」
	if !strings.Contains(spec.Desc, "get") || !strings.Contains(spec.Desc, "全量") {
		t.Errorf("desc 应指引先 *_get 取全量再写回，实际=%q", spec.Desc)
	}
	// 明文返回凭证是用户明确决定保留的既有行为，desc 里要点明避免后人误加掩码
	if !strings.Contains(spec.Desc, "明文") {
		t.Errorf("desc 应说明凭证明文返回（用户 2026-10-03 明确保留），实际=%q", spec.Desc)
	}
	// values 的 desc 也要带警告（AI 常只看参数说明）
	v := spec.Params["values"]
	if v.Desc == "" {
		t.Fatal("values 未声明")
	}
	if !strings.Contains(v.Desc, "全量") {
		t.Errorf("values.desc 应说明必须传全量，实际=%q", v.Desc)
	}
	if !strings.Contains(v.Desc, "回读") {
		t.Errorf("values.desc 应提示 save 响应 data 常为 null、要用 get 回读校验，实际=%q", v.Desc)
	}
}

// TestSendmailSettingSaveDescWarnsOverwrite 邮件配置同样是全量覆盖，
// desc 里的警告不能因为改别的意图而丢掉。
func TestSendmailSettingSaveDescWarnsOverwrite(t *testing.T) {
	spec := mustSpec(t, "channel_sendmail")
	if !strings.Contains(spec.Desc, "全量覆盖") {
		t.Errorf("channel_sendmail desc 应说明 setting_save 是全量覆盖，实际=%q", spec.Desc)
	}
	if !strings.Contains(spec.Desc, "setting_get") {
		t.Errorf("desc 应指引先 setting_get 取全量，实际=%q", spec.Desc)
	}
	// 密码掩码语义也要写明（掩码原样回传 = 未改动）
	if !strings.Contains(spec.Desc, "掩码") {
		t.Errorf("desc 应说明 password 掩码语义，实际=%q", spec.Desc)
	}
}

// mustHaveAll 断言 desc 同时包含若干关键片段，任一缺失即失败。
func mustHaveAll(t *testing.T, label, desc string, fragments ...string) {
	t.Helper()
	for _, f := range fragments {
		if !strings.Contains(desc, f) {
			t.Errorf("%s 缺少关键说明 %q\n实际 desc=%q", label, f, desc)
		}
	}
}

// TestWechatValuesDescListsFields channel_wechat 的 values desc 必须列出各 action 的字段名。
//
// 为什么要锁：2026-10-02 实测因 desc 只写「配置或内容参数」，AI 传 is_default
// 为布尔值被端点拒绝（json: cannot unmarshal），且传 title/subject 之类错字段名
// 时端点照样返回「操作成功」却什么都没写。同一天在 channel_subscriber（subject
// vs title）和 skill（tags 该用数组）上重演了同一类问题 —— 不是个案。
func TestWechatValuesDescListsFields(t *testing.T) {
	spec := mustSpec(t, "channel_wechat")
	mustHaveAll(t, "channel_wechat.values",
		spec.Params["values"].Desc,
		"menu_save", "name", "type", "value", "sort", "parent_id", // 菜单字段
		"rule_save", "keyword", "content", "is_default", // 规则字段
		"message_reply", "reply", // 消息字段
		"is_default(整数 0 或 1，不是布尔值", // 踩过的类型坑
		"全量覆盖",
	)
	// 返回形状差异也是实测踩过的坑：menu_list 空表 null vs message_list 空表 []
	mustHaveAll(t, "channel_wechat.Desc", spec.Desc,
		"children", "data 为 null", "回读", "menu_sync",
	)
	// id 参数要说明「传了即更新、不传即新建」
	mustHaveAll(t, "channel_wechat.id", spec.Params["id"].Desc, "更新", "新建")
}

// TestSeoPushSaveDescListsFields seo.push_save 是全量覆盖，字段名必须写明。
func TestSeoPushSaveDescListsFields(t *testing.T) {
	spec := mustSpec(t, "seo")
	mustHaveAll(t, "seo.values", spec.Params["values"].Desc,
		"baidu_api", "bing_api", "google_json", "js_codes", "全量覆盖",
	)
}

// TestInteractionValuesDescListsFields 留言设置的字段与 push_way 枚举要写明，
// status 的 0/1/2 语义也要写（整数不是布尔值）。
func TestInteractionValuesDescListsFields(t *testing.T) {
	spec := mustSpec(t, "interaction")
	mustHaveAll(t, "interaction.values", spec.Params["values"].Desc,
		"return_message", "push_way", "site_id", "api_url", "fields", "全量覆盖",
	)
	mustHaveAll(t, "interaction.status", spec.Params["status"].Desc,
		"整数不是布尔值", "0=待审", "2=垃圾",
	)
	// 批量操作入口：端点有 ids 字段，意图层必须暴露，否则 AI 只能逐条循环
	if _, ok := spec.Params["ids"]; !ok {
		t.Error("interaction 未暴露 ids（guestbook delete/status 支持批量，前端菜单里也有批量操作）")
	}
}

// TestOrderIdIsBusinessOrderNo commerce_order 的 id 是订单业务号而非数据库自增 id。
//
// 为什么要锁：端点侧主键字段是 order_id（string，业务单号如 wc2021101838889109642），
// 意图层对外统一叫 id 并做了 rename 换算，所以功能上两种传法都能打通 ——
// 但类型必须声明为 string。若声明成 integer，AI 会照 list 返回里的数据库 id 去传，
// 结果是「record not found」，且看不出是自己传错了字段。
func TestOrderIdIsBusinessOrderNo(t *testing.T) {
	spec := mustSpec(t, "commerce_order")
	id := spec.Params["id"]
	if id.Type != "string" {
		t.Errorf("commerce_order.id 类型应为 string（订单业务单号），实际=%q", id.Type)
	}
	mustHaveAll(t, "commerce_order.id", id.Desc,
		"订单号", "不是数据库自增 id", "order_id",
	)
}

// TestCommerceUserUpdateIsPartialUpdate user_update 现在是**部分更新**：
// 意图层会先回查旧值再补齐未传字段，所以 desc 不该再要求调用方手工 get 全量。
//
// 这条测试的来历值得留着：早先它锁的是「desc 必须警告全量覆盖」——
// 当时的解法是**只在 desc 里警告，把风险推给调用方**。实测证明 AI 不会可靠照做：
// 2026-10-03 会员 id=1564 只改 user_name，email/phone/invite_code 被清空、
// status 从 1 变 0（账号被禁用）。于是改为在 invokeRoutes 层兜底
// （preserveOnInvokeRoute），desc 随之更正。
//
// 锁的是「不再声称需要手工 get 全量」，避免有人日后把兜底删掉却忘了改 desc。
func TestCommerceUserUpdateIsPartialUpdate(t *testing.T) {
	spec := mustSpec(t, "commerce")
	mustHaveAll(t, "commerce.Desc", spec.Desc, "user_update", "部分更新", "沿用原值")
	mustHaveAll(t, "commerce.values", spec.Params["values"].Desc, "user_update", "部分更新")
	// 反向断言：旧文案不该再出现，否则等于没修
	for _, bad := range []string{"先 user_get 取全量", "一起整体写回"} {
		if strings.Contains(spec.Desc, bad) || strings.Contains(spec.Params["values"].Desc, bad) {
			t.Errorf("desc 仍含旧的风险转嫁文案 %q，行为已由意图层兜底，不该再要求调用方手工处理", bad)
		}
	}
}

// TestFsGlobDescAdvertisesSupportedSyntax fs_glob 现在真正支持目录前缀与 **，
// desc 必须把两种 pattern 写法都讲清楚，且不能再出现「不支持」的误导说法。
//
// 背景：2026-10-03 发现实现缺陷（只按 basename 匹配 + ** 走 HasPrefix/HasSuffix 不做通配），
// 当时只把 desc 改成「实测行为」掩盖了过去。修复 provider 侧实现后（matchGlobPattern），
// desc 要同步回正确的完整语法，否则又会与实现脱节。
func TestFsGlobDescAdvertisesSupportedSyntax(t *testing.T) {
	spec := mustSpec(t, "fs_glob")
	mustHaveAll(t, "fs_glob.Desc", spec.Desc,
		"文件名模式",   // 不含 / 的写法
		"路径模式",     // 含 / 的写法
		"跨任意层级",   // ** 的语义
	)
	// 不能再说「不支持 / 或 **」——那是修复前的临时描述
	for _, bad := range []string{"既不认目录前缀", "不要用 **", "不要带目录前缀"} {
		if strings.Contains(spec.Desc, bad) || strings.Contains(spec.Params["pattern"].Desc, bad) {
			t.Errorf("desc 不应再宣称不支持目录前缀或 **（实现已支持），实际 Desc=%q pattern=%q",
				spec.Desc, spec.Params["pattern"].Desc)
		}
	}
	// pattern 的示例要覆盖两类写法
	mustHaveAll(t, "fs_glob.pattern", spec.Params["pattern"].Desc, "**/*.go", "template/**")
}
