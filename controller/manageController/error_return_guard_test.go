package manageController

import (
	"regexp"
	"strings"
	"testing"
)

// 端点测试里发现的另一类缺陷：错误分支写完 TODO 回执后没有 return，
// 紧接着又写了一次成功回执，响应体变成两段 JSON 拼接（客户端解析失败，
// 真实错误也被成功回执盖掉）。CommunicationOrderClose、CommunicationMessageDelete
// 都踩过这个坑。
//
// 这条护栏扫全包的 handler：凡是 `if ... err != nil {` 块里写了 ctx.JSON，
// 块内必须有 return。
func TestErrorBranchesReturn(t *testing.T) {
	src := loadControllerSources(t)
	openIf := regexp.MustCompile(`(?m)^\s*if .*err != nil \{\s*$`)

	for name, body := range src {
		lines := strings.Split(body, "\n")
		for i, line := range lines {
			if !openIf.MatchString(line) {
				continue
			}
			depth, end := 0, -1
			for j := i; j < len(lines); j++ {
				depth += strings.Count(lines[j], "{") - strings.Count(lines[j], "}")
				if depth == 0 && j > i {
					end = j
					break
				}
			}
			if end < 0 {
				continue
			}
			block := strings.Join(lines[i:end+1], "\n")
			if strings.Contains(block, "ctx.JSON") && !strings.Contains(block, "return") {
				t.Errorf("%s 的错误分支写了回执却没有 return，会再写一次成功回执：\n%s", name, block)
			}
		}
	}
}
