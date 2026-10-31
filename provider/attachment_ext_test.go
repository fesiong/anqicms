package provider

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"
)

// dangerousAttachmentExt 是可被服务端解析/落地的后缀集合，供多个测试共用。
var dangerousAttachmentExt = []string{
	// php 家族：多版本 php-fpm / Apache 都可能把下列后缀交给 PHP 解析
	".php", ".php3", ".php4", ".php5", ".php7", ".php8", ".phps",
	".phtml", ".phtm", ".pht", ".phar", ".inc",
	// 其他服务端脚本
	".jsp", ".jspx", ".jspf", ".asp", ".aspx", ".asa", ".ascx", ".cer", ".cdx",
	".shtml", ".shtm", ".stm",
	// 可直接执行或落地的脚本/中间件
	".cgi", ".pl", ".py", ".rb", ".sh", ".bash", ".zsh", ".bat", ".cmd",
	".exe", ".dll", ".so",
	// 服务器配置：部分环境会按目录读取
	".htaccess", ".htpasswd", ".user.ini",
}

var dangerousAttachmentExtSet = func() map[string]bool {
	m := make(map[string]bool, len(dangerousAttachmentExt))
	for _, e := range dangerousAttachmentExt {
		m[e] = true
	}
	return m
}()

// 本文件是 2026-10-31 上传目录木马事件的护栏。
//
// 事故：public/uploads/202610/31/ 出现 shell.php5 与 s2.phtml。
// 根因：AttachmentUpload 的扩展名判断是黑名单，只列了 .php 与 .jsp，
// 其余可被 php-fpm 解析的后缀（.php5/.phtml/.pht/.phar 等）全部放行。
//
// 三条断言分别守住：
//  1. 危险后缀不再被放行（且函数不是恒 false —— 反向对照）；
//  2. 大小写混写不能绕过（旧代码只在部分分支做了 ToLower）；
//  3. 业务常用后缀不被误伤（修复不能把正常上传打死）。

func TestIsAllowedAttachmentExtRejectsExecutableExtensions(t *testing.T) {
	for _, ext := range dangerousAttachmentExt {
		if IsAllowedAttachmentExt(ext) {
			t.Errorf("危险后缀 %q 被放行，可落地成可执行文件", ext)
		}
	}

	// 反向对照：若被测函数恒返回 false，上面所有断言都会空转（假绿）。
	// 必须证明它在正常输入上会返回 true，本测试才具备区分能力。
	if !IsAllowedAttachmentExt(".jpg") {
		t.Fatal("反向对照失败：.jpg 也被拒绝，说明本测试无法区分放行与拒绝")
	}
}

func TestIsAllowedAttachmentExtIsCaseInsensitive(t *testing.T) {
	// 攻击者会尝试 .PHP5 / .Phtml 这类混写；文件系统大小写语义因平台而异，
	// 但判定必须在归一化之后进行，不能依赖「nginx 只匹配小写」这种部署假设。
	for _, ext := range []string{".PHP", ".Php5", ".PHTML", ".ASPX", ".PhAr"} {
		if IsAllowedAttachmentExt(ext) {
			t.Errorf("大写混写的危险后缀 %q 绕过了白名单", ext)
		}
	}
	for _, ext := range []string{".JPG", ".PNG", ".PdF"} {
		if !IsAllowedAttachmentExt(ext) {
			t.Errorf("大写混写的安全后缀 %q 被误拒", ext)
		}
	}
	// 不带前导点也必须按同一规则判定，避免调用方漏传 "." 时静默放行。
	if IsAllowedAttachmentExt("php5") {
		t.Error(`不带前导点的 "php5" 被放行`)
	}
	if !IsAllowedAttachmentExt("jpg") {
		t.Error(`不带前导点的 "jpg" 被拒绝`)
	}
}

func TestIsAllowedAttachmentExtAcceptsCommonBusinessTypes(t *testing.T) {
	// 覆盖现有业务的真实上传类型：图片/视频/音频/文档/压缩包/字体。
	// 这张表是「修复不能打死功能」的上界，少一项都会让线上正常上传开始报错。
	allowed := []string{
		".jpg", ".jpeg", ".png", ".gif", ".webp", ".avif", ".tiff", ".tif",
		".bmp", ".ico", ".svg", ".heic",
		".mp4", ".webm", ".mov", ".avi", ".mkv", ".flv", ".wmv", ".m4v",
		".mp3", ".wav", ".m4a", ".aac", ".ogg", ".flac",
		".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
		".txt", ".csv", ".md", ".json", ".xml",
		".zip", ".rar", ".7z", ".gz", ".tar", ".bz2",
		".ttf", ".otf", ".woff", ".woff2", ".eot",
	}
	for _, ext := range allowed {
		if !IsAllowedAttachmentExt(ext) {
			t.Errorf("业务常用后缀 %q 被误拒，会打断正常上传", ext)
		}
	}
}

func TestIsAllowedAttachmentExtRejectsEmpty(t *testing.T) {
	// 空扩展名意味着白名单被绕过。旧代码把 "." 归一成 "" 后直接放行，
	// 等于给「文件名不带点」开了后门，这里必须一并堵住。
	for _, ext := range []string{"", ".", " ", ".."} {
		if IsAllowedAttachmentExt(ext) {
			t.Errorf("空/无意义扩展名 %q 被放行", ext)
		}
	}
}

// TestAttachmentUploadUsesWhitelist 断言白名单真的接在了上传主路径上。
//
// 为什么需要：把 IsAllowedAttachmentExt 写对、但忘了在 AttachmentUpload 里调用，
// 上面那些测试依然全绿，线上却照样能被上传 .php5 —— 这是「函数存在但没人调」
// 的典型假绿，本仓踩过同类坑。故用 go/ast 直接扫源码结构。
func TestAttachmentUploadUsesWhitelist(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "attachment.go", nil, 0)
	if err != nil {
		t.Fatalf("解析 attachment.go 失败: %v", err)
	}

	var fn *ast.FuncDecl
	for _, decl := range file.Decls {
		d, ok := decl.(*ast.FuncDecl)
		if ok && d.Name.Name == "AttachmentUpload" && d.Recv != nil {
			fn = d
			break
		}
	}
	if fn == nil {
		t.Fatal("未找到 Website.AttachmentUpload，测试锚点失效（函数被改名或搬走，需同步本护栏）")
	}

	callsWhitelist := false
	hasDangerousLiteralCmp := false
	recordCmp := func(expr ast.Expr) {
		lit, ok := expr.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return
		}
		v, err := strconv.Unquote(lit.Value)
		if err != nil {
			return
		}
		// 只对危险后缀报警：fileExt == ".jpeg" / ".avif" 这类正常分支必须允许存在，
		// 否则本断言会把整段图片处理逻辑误判成"黑名单回来了"。
		if dangerousAttachmentExtSet[strings.ToLower(v)] {
			hasDangerousLiteralCmp = true
		}
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			if id, ok := node.Fun.(*ast.Ident); ok && id.Name == "IsAllowedAttachmentExt" {
				callsWhitelist = true
			}
		case *ast.BinaryExpr:
			if node.Op != token.EQL && node.Op != token.NEQ {
				return true
			}
			if id, ok := node.X.(*ast.Ident); ok && id.Name == "fileExt" {
				recordCmp(node.Y)
			}
			if id, ok := node.Y.(*ast.Ident); ok && id.Name == "fileExt" {
				recordCmp(node.X)
			}
		}
		return true
	})

	if !callsWhitelist {
		t.Error("AttachmentUpload 未调用 IsAllowedAttachmentExt，白名单没有真正生效")
	}
	if hasDangerousLiteralCmp {
		t.Error("AttachmentUpload 里仍有 fileExt 与危险后缀字面量的比较，疑似退回黑名单写法")
	}
}
