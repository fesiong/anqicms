package provider

import (
	"testing"

	"kandaoni.com/anqicms/provider/fulltext"
)

// fakeFulltextService 只用于验证状态组装，不真正建索引。
type fakeFulltextService struct {
	count int64
}

func (f *fakeFulltextService) Index(body interface{}) error { return nil }
func (f *fakeFulltextService) Create(doc fulltext.TinyArchive) error {
	return nil
}
func (f *fakeFulltextService) Update(doc fulltext.TinyArchive) error {
	return nil
}
func (f *fakeFulltextService) Delete(doc fulltext.TinyArchive) error {
	return nil
}
func (f *fakeFulltextService) Bulk(docs []fulltext.TinyArchive) error { return nil }
func (f *fakeFulltextService) Search(keyword string, moduleId uint, page int, pageSize int) ([]fulltext.TinyArchive, int64, error) {
	return nil, 0, nil
}
func (f *fakeFulltextService) Count() (int64, error) { return f.count, nil }
func (f *fakeFulltextService) Close()                {}
func (f *fakeFulltextService) Flush()                {}

// TestGetFullTextStatusIncludesIndexed 保证"索引数量"真的从搜索引擎取数，
// 并且返回的是副本（后台构建协程仍在写 Current/Msg，直接返回原指针会竞态）。
func TestGetFullTextStatusIncludesIndexed(t *testing.T) {
	w := &Website{
		fulltextStatus: &FulltextStatus{Status: 2, Total: 1200, Current: 1000},
		searcher:       &fakeFulltextService{count: 1000},
	}

	status := w.GetFullTextStatus()
	if status == nil {
		t.Fatal("状态不应为 nil")
	}
	if status.Indexed != 1000 {
		t.Fatalf("索引数量应取自搜索引擎，实际 %d", status.Indexed)
	}
	if status.Status != 2 || status.Current != 1000 || status.Total != 1200 {
		t.Fatalf("进度字段被破坏: %+v", status)
	}
	if status == w.fulltextStatus {
		t.Fatal("应返回副本，避免与构建协程并发读写")
	}

	// 反向对照：没有引擎时不得伪造数量
	w2 := &Website{fulltextStatus: &FulltextStatus{Status: 2}}
	if got := w2.GetFullTextStatus(); got == nil || got.Indexed != 0 {
		t.Fatalf("无搜索引擎时应为 0，实际 %+v", got)
	}

	// 未启用过全文索引时保持原语义：返回 nil，前端据此不展示
	if got := (&Website{}).GetFullTextStatus(); got != nil {
		t.Fatalf("未初始化时应返回 nil，实际 %+v", got)
	}
}
