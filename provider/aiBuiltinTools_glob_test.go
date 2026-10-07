package provider

import "testing"

// TestMatchGlobPattern 穷举 glob 的两类 pattern 语义。
//
// 2026-10-03 修复的缺陷（实测 '**/*.go'、'pkg/mcp/intent/*.go'、'template/**' 全部返回「未找到」）：
//  1. 非 ** 分支只按 basename 匹配 → 带目录前缀的 pattern 永远匹配不到；
//  2. ** 分支拆成 prefix/suffix 后用 HasPrefix/HasSuffix → 那两个函数不做通配，
//     suffix `*.go` 永远匹配不上；且只处理恰好一个 **，`a/**/b/**/c` 直接失效。
func TestMatchGlobPattern(t *testing.T) {
	cases := []struct {
		pattern string
		rel     string
		want    bool
	}{
		// ── ① 不含 '/'：文件名模式，匹配任意层级（保持旧行为，向后兼容）──
		{"*.go", "main.go", true},
		{"*.go", "pkg/mcp/intent/catalog.go", true},
		{"*.go", "a/b/c/d/deep.go", true},
		{"*.go", "main.js", false},
		{"catalog_*.go", "pkg/mcp/intent/catalog_api.go", true},
		{"catalog_*.go", "pkg/mcp/intent/catalog_merged.go", true},
		{"catalog_*.go", "pkg/mcp/intent/api.go", false},
		{"go.mod", "go.mod", true},
		{"go.mod", "pkg/sonic-loader/go.mod", true},
		{"go.mod", "pkg/x/go.sum", false},

		// ── ② 含 '/'：路径模式，逐段匹配 ──
		{"pkg/mcp/intent/*.go", "pkg/mcp/intent/catalog.go", true},
		{"pkg/mcp/intent/*.go", "pkg/mcp/intent/sub/x.go", false},
		{"pkg/mcp/intent/*.go", "pkg/mcp/server/server.go", false},
		{"pkg/*/server.go", "pkg/mcp/server.go", true},
		{"pkg/*/server.go", "pkg/a/b/server.go", false},
		// 段内通配
		{"pkg/mcp/*/*.go", "pkg/mcp/intent/catalog.go", true},
		{"data/*.json", "data/a.json", true},
		{"data/*.json", "data/sub/a.json", false},
		// 字符类与 ? （委托 filepath.Match）
		{"[abc]*.go", "apple.go", true},
		{"[abc]*.go", "pear.go", false},
		{"?.go", "a.go", true},
		{"?.go", "ab.go", false},

		// ── ③ ** 跨任意层级 ──
		{"**/*.go", "main.go", true},                     // 零层
		{"**/*.go", "a/b/c/deep.go", true},               // 多层
		{"**/*.json", "a/b.json", true},
		{"**/*.json", "a/b/c.json", true},
		{"**/*.json", "a/b/c.txt", false},
		{"**/catalog_*.go", "pkg/mcp/intent/catalog_api.go", true},
		{"**/catalog_*.go", "catalog_api.go", true}, // ** 吃掉零层
		{"**/catalog_*.go", "pkg/mcp/intent/catalog.go", false}, // 同理：catalog.go 无下划线
		{"**/catalog_*.go", "catalog.go", false},                  // 同上
		{"pkg/**/*.go", "pkg/a.go", true},                // ** 匹配零层
		{"pkg/**/*.go", "pkg/a/b/c.go", true},            // ** 匹配多层
		{"pkg/**/*.go", "other/a.go", false},             // 前缀不符
		{"**/mcp/**/*.go", "pkg/mcp/intent/catalog.go", true},
		{"**/mcp/**/*.go", "pkg/other/intent/catalog.go", false},
		// 末尾的 ** 吞掉剩余全部段
		{"template/**", "template", true},
		{"template/**", "template/a", true},
		{"template/**", "template/a/b/c.html", true},
		{"template/**", "other/a", false},
		{"docs/**", "docs", true},
		// 多个 ** —— 旧实现直接失效（len(parts) != 2）
		{"a/**/b/**/c", "a/b/c", true},
		{"a/**/b/**/c", "a/x/b/y/c", true},
		{"a/**/b/**/c", "a/b/c", true},   // 两个 ** 各吃零层
		{"a/**/b/**/c", "a/b/x/c", true}, // 第一个 ** 吃零层、第二个吃一层（b 段被复用）→ 标准 glob 语义下成立
		{"a/**/b/**/c", "a/x/c", false},  // 中间的 b 段必须存在
		{"**/**/x", "a/b/x", true},

		// ── ④ 边界 ──
		{"", "main.go", false},
		{"*", "a", true},
		{"**", "a/b/c", true},
		// 刻意行为：不含 '/' 的 pattern 走「文件名模式」，故 * 匹配任意层级的 basename
		{"*", "a/b", true},
		// 但一旦含 '/' 就是路径模式，单个 * 不跨层
		{"a/*", "a/b", true},
		{"a/*", "a/b/c", false},
		{"a/b", "a/b", true},
		{"a/b", "a/b/c", false},
		// 刻意行为：'a//b' 等价 'a/b'（标准 glob 语义），故为 true
		{"a//b", "a/b", true},
	}
	for _, c := range cases {
		if got := matchGlobPattern(c.pattern, c.rel); got != c.want {
			t.Errorf("matchGlobPattern(%q, %q) = %v，期望 %v", c.pattern, c.rel, got, c.want)
		}
	}
}

// TestMatchGlobPatternZeroValue 防御：空 rel（Walk 出错时 rel 可能为空串）不应 panic。
func TestMatchGlobPatternZeroValue(t *testing.T) {
	if matchGlobPattern("*.go", "") {
		t.Error("空路径不应匹配任何 pattern")
	}
	if matchGlobPattern("**", "") {
		t.Error("空路径不应匹配 **")
	}
}

// TestMatchGlobPatternBackslashWindows 传入 Windows 风格分隔符时也要归一化，
// 不然同一份 pattern 在不同平台行为不一致。
func TestMatchGlobPatternBackslashWindows(t *testing.T) {
	if !matchGlobPattern(`pkg\mcp\intent\*.go`, `pkg\mcp\intent\catalog.go`) {
		t.Error("反斜杠形式的路径模式应能匹配（ToSlash 归一化未生效？）")
	}
}
