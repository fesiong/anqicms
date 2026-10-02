package provider

import (
	"encoding/json"
	"testing"
)

// TestIsWithinScope 验证 worker 子 agent 的 scope 校验:
//   - 解析路径字段做目录边界感知的 glob 匹配
//   - 杜绝子串误中 (如 /a/b/ 命中 /a/bc/x)
//   - 未声明 scope 时 fail-closed
func TestIsWithinScope(t *testing.T) {
	cases := []struct {
		name  string
		args  map[string]any
		scope []string
		want  bool
	}{
		{
			name:  "绝对路径目录 scope 命中子文件",
			args:  map[string]any{"file_path": "/a/b/c.go"},
			scope: []string{"/a/b/**"},
			want:  true,
		},
		{
			name:  "子串越界被拒 (/a/b/ 不应命中 /a/bc/x)",
			args:  map[string]any{"file_path": "/a/bc/x.go"},
			scope: []string{"/a/b/**"},
			want:  false,
		},
		{
			name:  "**/*.go 后缀过滤",
			args:  map[string]any{"file_path": "/a/b/c.go"},
			scope: []string{"/a/b/**/*.go"},
			want:  true,
		},
		{
			name:  "**/*.go 拒绝 .py",
			args:  map[string]any{"file_path": "/a/b/sub/x.py"},
			scope: []string{"/a/b/**/*.go"},
			want:  false,
		},
		{
			name:  "相对目录 scope 命中相对文件",
			args:  map[string]any{"file_path": "src/main.go"},
			scope: []string{"src/**"},
			want:  true,
		},
		{
			name:  "基准名通配 *.go 跨目录命中",
			args:  map[string]any{"file_path": "/x/y/main.go"},
			scope: []string{"*.go"},
			want:  true,
		},
		{
			name:  "空 scope 一律拒绝 (fail-closed)",
			args:  map[string]any{"file_path": "/a/b/x.go"},
			scope: []string{},
			want:  false,
		},
		{
			name:  "无路径字段时回退子串匹配 (旧行为兜底)",
			args:  map[string]any{"query": "please write /a/b/x.go now"},
			scope: []string{"/a/b/"},
			want:  true,
		},
		{
			name:  "无路径字段且不匹配则拒",
			args:  map[string]any{"query": "hello world"},
			scope: []string{"/a/b/"},
			want:  false,
		},
		{
			name:  "绝对 scope 拒绝上层越界写入",
			args:  map[string]any{"file_path": "/a/etc/passwd"},
			scope: []string{"/a/b/**"},
			want:  false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			raw, err := json.Marshal(c.args)
			if err != nil {
				t.Fatalf("marshal args: %v", err)
			}
			got := isWithinScope(string(raw), c.scope)
			if got != c.want {
				t.Errorf("isWithinScope(%s, %v) = %v, want %v", raw, c.scope, got, c.want)
			}
		})
	}
}
