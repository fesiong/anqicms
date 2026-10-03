package intent

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestIsErrorSetOnEndpointFailure 端点业务失败时必须置 MCP 协议的 isError。
//
// 背景（2026-10-03 实测）：api_invoke 把端点失败（5xx / ok=false）表达成
// **成功返回的 JSON**，Compose 因此不返回 Go error，cerr 为 nil。
// handler 原本只在 cerr != nil 时设 IsError，于是：
//
//	system_multilang action=cache_delete → status=500、body 里 ok:false，
//	但 MCP 协议层 isError 为空 —— 严格依赖 isError 的客户端会当成功处理。
//
// isError 在 SDK 里是 `json:"isError,omitempty"`，不显式置 true 等于没告知。
// 写操作失败却不报错，会让 AI 以为已经改好了。
func TestIsErrorSetOnEndpointFailure(t *testing.T) {
	cases := []struct {
		name    string
		text    string
		wantErr bool
	}{
		{
			name: "端点 5xx",
			text:    `{"ok":false,"status":500,"msg":"","data":null,"code":0}`,
			wantErr: true,
		},
		{
			name: "端点 ok=false",
			text:    `{"ok":false,"status":200,"msg":"端点返回失败但未给出原因","data":null,"code":0}`,
			wantErr: true,
		},
		{
			name: "控制器层 code 非 0",
			text:    `{"ok":true,"status":200,"code":0,"msg":"","data":{"code":-1,"msg":"record not found","data":null}}`,
			wantErr: true,
		},
		{
			name: "正常成功不应误报",
			text:    `{"ok":true,"status":200,"code":0,"msg":"","data":{"list":[],"total":0,"page":1,"page_size":20,"count":0}}`,
			wantErr: false,
		},
	}
	for _, c := range cases {
		// 直接验证 endpointFailure 对这些响应的判定，
		// handler 层的接线由下面 TestHandlerMarksEndpointFailureViaMCP 端到端覆盖。
		got := endpointFailure(c.text) != ""
		if got != c.wantErr {
			t.Errorf("%s: endpointFailure 判定 = %v，期望 %v（msg=%q）",
				c.name, got, c.wantErr, endpointFailure(c.text))
		}
	}
}

// TestIsErrorSetOnTextChannelFailure 纯文本通道的失败同样必须置 isError。
//
// 背景（2026-10-04 实测）：fs_read / fs_write / fs_edit / fs_replace / fs_glob
// 这批文件工具失败时**不返回 JSON**，而是 `return "错误：…", nil` 的一段人话文本。
// endpointFailure 原来只做 json.Unmarshal，解析失败就返回 ""，于是：
//
//	fs_read path=../../../../etc/passwd
//	→ {"result":{"content":[{"type":"text","text":"错误：禁止访问系统敏感路径"}]}}
//	isError 字段整个缺失，AI 客户端会把「拒绝读系统文件」当成读取成功。
//
// 这条通道是模板修改的主力（改主题模板就走 fs_write/fs_edit），漏判影响面很大。
func TestIsErrorSetOnTextChannelFailure(t *testing.T) {
	cases := []struct {
		name    string
		text    string
		wantErr bool
	}{
		// 实测原文（2026-10-04，fs_read 传路径穿越）
		{"路径穿越被拒", "错误：禁止访问系统敏感路径", true},
		{"写入敏感路径被拒", "错误：禁止写入系统敏感路径", true},
		{"文件不存在", "错误：文件不存在: template/default/不存在.html", true},
		// fs_edit 文本模式匹配失败：注意它是多行文本，不是单行「错误：」开头
		{"edit 精确匹配失败", "精确匹配失败。在第 2 行附近找到相似内容，请检查搜索文本是否与文件内容精确一致（包括缩进）：\n\n{% extends 'base.html' %}", true},
		// fs_write 覆盖风险：不是拒绝，而是要求 confirm 二次确认，同样是「没写成」
		{"write 覆盖需确认", "⚠ 警告：文件 template/default/errors/404.html 将缩小超过 50%（从 282 字节到 12 字节），是否确认？请检查内容是否完整。", true},
		{"replace 无命中", "未找到匹配的文件", true},
		// 以下四条的原文案**没有**「错误：」前缀，2026-10-04 已给源头补上
		// （provider/aiBuiltinTools.go）。它们都是「请求无法执行」而非正常回执：
		// 分页/行号越界、路径不存在、glob 命中过多。
		{"replace 匹配过多需缩范围", "错误：匹配文件过多 (6445)，请缩小 glob 范围", true},
		{"replace 分页越界", "错误：匹配文件共 12 个，起始序号 99 超出范围", true},
		{"search 分页越界", "错误：共匹配 3 处，起始序号 99 超出范围", true},
		{"search 路径不存在", "错误：路径不存在或无法访问: template/default/nope.html", true},
		// fs_read 行号越界：原文案以「文件: …」开头像成功回执，实为「读不到」
		{"read 行号越界", "错误：文件: template/default/errors/404.html (12 行)\n\n起始行号 999 超出文件总行数 12", true},
		// 以下是真实成功文案（取自实测输出），必须判为成功，否则误报会淹没真故障
		{"成功：文件已更新", "文件 template/default/errors/404.html 已更新，共替换 1 处", false},
		{"成功：行模式已替换", "文件 template/default/errors/404.html 已更新（行 8-8 已替换）", false},
		{"成功：写入成功", "文件写入成功: template/default/errors/404.html (282 字节)", false},
		// fs_search 无命中是**搜索成功的正常回执**，不是失败：搜到了 0 个匹配 ≠ 工具出错。
		// 误标成失败会让 AI 以为 grep 坏了，反复重搜同一个不存在的关键词。
		{"成功：search 无命中", "未找到匹配的内容", false},
		{"成功：search 无命中带跳过说明", "未找到匹配的内容\n\n[本次扫描跳过 2 个超大文件（>1MB 或 >5000 行）：a.go、b.go，匹配可能正在其中。要搜它们请传 path=<单个文件路径>，或用 read_file 分段读取]", false},
		{"成功：read 正常读", "文件: template/default/errors/404.html (12 行, 286 字节)", false},
		{"成功：重载信号", `{"code":0,"data":{"message":"模板重载信号已发送","reload_in":"1秒","template":"default"},"msg":"","ok":true,"status":200}`, false},
	}
	for _, c := range cases {
		got := endpointFailure(c.text) != ""
		if got != c.wantErr {
			t.Errorf("%s: endpointFailure 判定 = %v，期望 %v（msg=%q）",
				c.name, got, c.wantErr, endpointFailure(c.text))
		}
	}
}

// TestTextFailurePrefixNoConflictWithSuccess 锁住「失败前缀」与「成功文案」不重叠。
//
// 判定用前缀匹配，一旦有人给某个文件工具的成功文案加上「错误：」之类的前缀
// （比如「错误：0 个文件被修改」），成功就会开始误报 isError，AI 会拒收正常结果。
// 这里把真实成功文案钉住，让这种冲突在测试里立刻暴露。
func TestTextFailurePrefixNoConflictWithSuccess(t *testing.T) {
	successTexts := []string{
		"文件 xxx.html 已更新，共替换 1 处",
		"文件 xxx.html 已更新（行 8-8 已替换）",
		"文件写入成功: xxx.html (282 字节)",
		"文件: template/default/errors/404.html (12 行, 286 字节)",
		"已替换 3 个文件",
		"未找到匹配的内容", // fs_search 无命中 = 搜索成功，只是结果为空
	}
	for _, s := range successTexts {
		if msg := textFailure(s); msg != "" {
			t.Errorf("成功文案被误判为失败：%q → %q", s, msg)
		}
	}
}

// TestFileToolRejectionsCarryErrorPrefix 锁住 provider 侧文件工具的拒绝文案都带前缀。
//
// 为什么需要这条跨包约束：textFailure 靠**前缀**判定，而前缀写在
// provider/aiBuiltinTools.go 的文案里。那边的开发者随手写一句
// `return fmt.Sprintf("匹配文件过多…"), nil`（无 error、非前缀），
// 意图层这边完全无感知 —— 工具失败但协议层 isError 为空，AI 当成功，
// 而且**没有任何测试会红**。所以必须从源码里把这类返回捞出来断言。
//
// 判据：`return <非空字符串>, nil` 且文案不以已知成功前缀开头 → 必须以
// textFailureMarkers 之一开头。`return "", fmt.Errorf(...)` 是 Go error，
// 走 rpc error 通道，不在此列。
func TestFileToolRejectionsCarryErrorPrefix(t *testing.T) {
	src, err := os.ReadFile("../../../provider/aiBuiltinTools.go")
	if err != nil {
		t.Fatalf("读取 provider/aiBuiltinTools.go 失败: %v", err)
	}
	// 成功回执的文案特征：含这些词说明是在报「做成了什么」或「搜完了没结果」，不是拒绝。
	//
	// 「未找到匹配的内容」「未找到关于…的搜索结果」是有意排除的：web_search /
	// fs_search 无命中是**搜索成功但结果为空**的正常回执，标成失败会让 AI 以为
	// 搜索能力坏了。它与 fs_replace 的「未找到匹配的文件」语义相反 —— 后者是要改的
	// 文件一个都没匹配到，属于真失败。
	successHints := []string{"已更新", "已替换", "已写入", "写入成功", "命中",
		"读取", "已删除", "成功", "共匹配", "行）", "已完成",
		"未找到匹配的内容", "未找到关于"}

	// 匹配 return <表达式>, nil，表达式为字符串字面量 / fmt.Sprintf / 两者用 + 拼接。
	//
	// ⚠️ 两个已踩过的正则失效点：
	//  1. Sprintf 参数含嵌套括号（len(allFiles)）——用 `[^)]*` 会在第一个右括号处截断；
	//  2. 拒绝文案常写成 `"错误：" + fmt.Sprintf(...)` 前缀拼接 —— 只匹配单个字面量
	//     或单个 Sprintf 会整行漏掉。
	// 故这里匹配「字面量或 Sprintf 重复用 + 连接」的组合。
	re := regexp.MustCompile(`return ((?:"(?:[^"\\]|\\.)*"|fmt\.Sprintf\((?:[^()]|\([^()]*\))*\))(?:(?:\s*\+\s*)(?:"(?:[^"\\]|\\.)*"|fmt\.Sprintf\((?:[^()]|\([^()]*\))*\)|[a-zA-Z][a-zA-Z0-9_.]*))*(?:\s*\+\s*(?:skipNote|relPath|[a-zA-Z][a-zA-Z0-9_.]*))?),\s*nil\b`)
	checked := 0
	for _, m := range re.FindAllStringSubmatch(string(src), -1) {
		lit := m[1]
		isSuccess := false
		for _, h := range successHints {
			if strings.Contains(lit, h) {
				isSuccess = true
				break
			}
		}
		if isSuccess {
			continue
		}
		checked++
		ok := false
		for _, marker := range textFailureMarkers {
			if strings.Contains(lit, marker) || strings.Contains(lit, `"`+marker) {
				ok = true
				break
			}
		}
		// fmt.Sprintf("错误：…") 的字面量在 marker 里，Contains 即可命中。
		if !ok {
			t.Errorf("拒绝文案缺少失败前缀，协议层会漏标 isError：\n\t%s\n\t（修复：在 provider 侧给该文案加 %q 前缀，并在 textFailureMarkers 登记）",
				lit, textFailureMarkers[0])
		}
	}
	if checked == 0 {
		t.Fatal("一条拒绝文案都没匹配到 —— 正则已随源码改动失效，测试空转（等于没测）")
	}
	// 正则空转的防线：点名一条**一定存在于源码**的拒绝文案，匹配不到就说明
	// 整条正则对该行失效，上面的 checked 计数具有欺骗性（2026-10-04 踩过）。
	if !re.MatchString(`return "错误：" + fmt.Sprintf("匹配文件过多 (%d)，请缩小 glob 范围", len(allFiles)), nil`) {
		t.Fatal("正则未能匹配已知的 fs_replace 拒绝行 —— 正则已失效，本次校验结果不可信")
	}
	t.Logf("已校验 %d 条拒绝文案", checked)
}
