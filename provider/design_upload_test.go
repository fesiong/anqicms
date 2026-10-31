package provider

import (
	"path/filepath"
	"strings"
	"testing"
)

// 本文件守护模板包 / 静态资源的上传落地。
//
// 与附件上传同源的问题：public/static/<pkg>/ 是被浏览器直接访问的目录，
// 只用「跳过 .php」的黑名单挡不住 .phtml/.pht/.phar/.shtml。
// 另外整包导入那条路径（design.go 的 static/ 前缀分支）此前没有做路径越界校验，
// zip 条目名里的 ../ 可以写到 public 之外 —— 即 Zip Slip。

func TestIsAllowedDesignStaticExtRejectsScripts(t *testing.T) {
	dangerous := []string{
		".php", ".php3", ".php5", ".phtml", ".phtm", ".pht", ".phar", ".inc",
		".jsp", ".jspx", ".asp", ".aspx", ".asa", ".cer",
		".shtml", ".shtm", ".cgi", ".pl", ".py", ".rb", ".sh", ".bat", ".cmd",
		".exe", ".dll", ".so", ".htaccess", ".user.ini",
	}
	for _, ext := range dangerous {
		if isAllowedDesignStaticExt(ext) {
			t.Errorf("静态目录允许了可执行后缀 %q", ext)
		}
	}
	// 反向对照：函数恒 false 时上面全部空转
	if !isAllowedDesignStaticExt(".js") {
		t.Fatal("反向对照失败：.js 也被拒绝，本测试无法区分放行与拒绝")
	}
}

func TestIsAllowedDesignStaticExtAcceptsWebAssets(t *testing.T) {
	// 模板 static 目录的真实内容类型。少一项就会让正常模板包导入失败。
	allowed := []string{
		".html", ".htm", ".css", ".js", ".mjs", ".map", ".json", ".xml", ".txt", ".md",
		".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".ico", ".avif", ".bmp", ".tiff",
		".woff", ".woff2", ".ttf", ".otf", ".eot",
		".mp4", ".webm", ".mp3", ".wav", ".ogg",
		".pdf", ".zip",
	}
	for _, ext := range allowed {
		if !isAllowedDesignStaticExt(ext) {
			t.Errorf("静态资源后缀 %q 被误拒，会影响模板包导入", ext)
		}
	}
	// 大小写不敏感 + 无前导点
	if !isAllowedDesignStaticExt("JS") {
		t.Error(`"JS" 未被识别为 .js`)
	}
	if isAllowedDesignStaticExt("PHP5") {
		t.Error(`"PHP5" 未被识别为危险后缀`)
	}
}

func TestJoinArchivePathBlocksTraversal(t *testing.T) {
	root := filepath.Join("public", "static", "pkg")
	attacks := []string{
		"../../shell.phtml",
		"../../../etc/passwd",
		"/etc/passwd",
		"a/../../../x.phtml",
		`..\..\win.phtml`,
	}
	for _, rel := range attacks {
		got, ok := joinArchivePath(root, rel)
		if !ok {
			continue
		}
		relPath, err := filepath.Rel(root, got)
		if err != nil {
			t.Fatalf("Rel(%q, %q) 失败: %v", root, got, err)
		}
		if strings.HasPrefix(relPath, "..") {
			t.Errorf("joinArchivePath(%q) = %q，越界到 %q", rel, got, relPath)
		}
	}

	// 反向对照：正常条目必须落在 root 下，否则上面"没有越界"是因为什么都没拼出来。
	got, ok := joinArchivePath(root, "css/app.css")
	if !ok {
		t.Fatal("正常路径 css/app.css 被拒绝，本测试无区分能力")
	}
	if want := filepath.Join(root, "css", "app.css"); got != want {
		t.Errorf("joinArchivePath = %q, 期望 %q", got, want)
	}
}
