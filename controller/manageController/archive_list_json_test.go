package manageController

import (
	"testing"

	"kandaoni.com/anqicms/model"
)

// 空结果必须以 [] 返回，不能是 null：
// 前端 ProTable 在 total=0 且 data=null 时会求值 null.length，
// 抛 "Cannot read properties of null (reading 'length')"，total 停留在上一次的值。
func TestEnsureNonNilArchives(t *testing.T) {
	got := ensureNonNilArchives(nil)
	if got == nil {
		t.Fatal("nil 切片应归一为空切片，否则 JSON 会序列化成 null")
	}
	if len(got) != 0 {
		t.Fatalf("空输入应返回空切片，得到长度 %d", len(got))
	}

	src := []*model.ArchiveDraft{{Archive: model.Archive{Id: 1}}}
	got = ensureNonNilArchives(src)
	if len(got) != 1 || got[0] != src[0] {
		t.Fatalf("非空切片不应被改动：%#v", got)
	}
}
