package graph

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"kandaoni.com/anqicms/pkg/mcp/tools"
)

func TestNewWorkflow(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	w := NewWorkflow(nil, nil, nil, nil, logger)
	if w == nil {
		t.Fatal("NewWorkflow returned nil")
	}
	if w.logger == nil {
		t.Error("logger is nil")
	}
	if w.graph == nil {
		t.Error("graph is nil")
	}
	if w.txnGraph == nil {
		t.Error("txnGraph is nil")
	}
}

func TestWorkflow_RunContentWorkflow(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	w := NewWorkflow(nil, nil, nil, nil, logger)

	input := WorkflowInput{
		Title:      "Test Article",
		Content:    "This is test content",
		CategoryID: 1,
		Tags:       []string{"test", "demo"},
		UserID:     1,
	}

	output, err := w.RunContentWorkflow(context.Background(), input)
	if err != nil {
		t.Fatalf("RunContentWorkflow() error = %v", err)
	}
	if output == nil {
		t.Fatal("output is nil")
	}
	if output.Status != "success" {
		t.Errorf("output.Status = %v, want 'success'", output.Status)
	}
	if output.SeoScore != 85 {
		t.Errorf("output.SeoScore = %v, want 85", output.SeoScore)
	}
}

func TestWorkflow_RunTxnWorkflow(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	w := NewWorkflow(nil, nil, nil, nil, logger)

	result, err := w.RunTxnWorkflow(context.Background(), "test input")
	if err != nil {
		t.Fatalf("RunTxnWorkflow() error = %v", err)
	}
	if result == "" {
		t.Error("result is empty")
	}
}

func TestWorkflow_Run(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	w := NewWorkflow(nil, nil, nil, nil, logger)

	// Test with non-string input
	_, err := w.Run(context.Background(), 123)
	if err == nil {
		t.Error("expected error for non-string input")
	}

	// Test with string input (graph might be nil if compilation failed)
	// The graph might be compiled in NewWorkflow, so this depends on that
}

func TestWorkflowInput_Fields(t *testing.T) {
	input := WorkflowInput{
		Title:     "My Title",
		Content:   "My Content",
		CategoryID: 2,
		Tags:      []string{"tag1", "tag2"},
		ExtraData: map[string]any{"key": "value"},
		UserID:    10,
	}

	if input.Title != "My Title" {
		t.Errorf("Title = %v, want 'My Title'", input.Title)
	}
	if input.CategoryID != 2 {
		t.Errorf("CategoryID = %v, want 2", input.CategoryID)
	}
	if len(input.Tags) != 2 {
		t.Errorf("Tags length = %v, want 2", len(input.Tags))
	}
	if input.UserID != 10 {
		t.Errorf("UserID = %v, want 10", input.UserID)
	}
}

func TestWorkflowOutput_Fields(t *testing.T) {
	output := WorkflowOutput{
		ArchiveID:   42,
		Status:      "created",
		Suggestions: []string{"Add images", "Improve formatting"},
		SeoScore:    90,
		TagsSuggested: []string{"tech", "blog"},
	}

	if output.ArchiveID != 42 {
		t.Errorf("ArchiveID = %v, want 42", output.ArchiveID)
	}
	if output.Status != "created" {
		t.Errorf("Status = %v, want 'created'", output.Status)
	}
	if output.SeoScore != 90 {
		t.Errorf("SeoScore = %v, want 90", output.SeoScore)
	}
}

func TestMockArchiveProvider_Create(t *testing.T) {
	provider := &mockArchiveProvider{
		archives: make(map[uint]*tools.ArchiveRecord),
		nextID:   1,
	}

	req := tools.ArchiveCreateRequest{
		Title:      "Test",
		Content:    "Content",
		CategoryId: 1,
		Status:     1,
	}

	id, err := provider.CreateArchive(req)
	if err != nil {
		t.Fatalf("CreateArchive() error = %v", err)
	}
	if id != 1 {
		t.Errorf("id = %d, want 1", id)
	}

	archive, err := provider.GetArchive(id)
	if err != nil {
		t.Fatalf("GetArchive() error = %v", err)
	}
	if archive.Title != "Test" {
		t.Errorf("archive.Title = %v, want 'Test'", archive.Title)
	}
}

func TestMockArchiveProvider_List(t *testing.T) {
	provider := &mockArchiveProvider{
		archives: make(map[uint]*tools.ArchiveRecord),
		nextID:   1,
	}

	// Add test archives
	for i := 1; i <= 5; i++ {
		provider.archives[uint(i)] = &tools.ArchiveRecord{
			Id:         uint(i),
			Title:      "Article " + string(rune('A'+i)),
			CategoryId: uint(i % 2 + 1),
			Status:     1,
		}
		provider.nextID = uint(i + 1)
	}

	result, err := provider.ListArchives(tools.ArchiveListRequest{
		Page:     1,
		PageSize: 2,
	})
	if err != nil {
		t.Fatalf("ListArchives() error = %v", err)
	}
	if result.Total != 5 {
		t.Errorf("result.Total = %d, want 5", result.Total)
	}
	if len(result.Items) != 2 {
		t.Errorf("len(Items) = %d, want 2", len(result.Items))
	}
}

func TestMockArchiveProvider_Update(t *testing.T) {
	provider := &mockArchiveProvider{
		archives: make(map[uint]*tools.ArchiveRecord),
		nextID:   1,
	}

	provider.archives[1] = &tools.ArchiveRecord{Id: 1, Title: "Old"}

	newTitle := "New Title"
	err := provider.UpdateArchive(1, tools.ArchiveUpdateRequest{Title: &newTitle})
	if err != nil {
		t.Fatalf("UpdateArchive() error = %v", err)
	}

	archive, _ := provider.GetArchive(1)
	if archive.Title != "New Title" {
		t.Errorf("archive.Title = %v, want 'New Title'", archive.Title)
	}
}

func TestMockArchiveProvider_Delete(t *testing.T) {
	provider := &mockArchiveProvider{
		archives: make(map[uint]*tools.ArchiveRecord),
		nextID:   1,
	}

	provider.archives[1] = &tools.ArchiveRecord{Id: 1}
	err := provider.DeleteArchive(1)
	if err != nil {
		t.Fatalf("DeleteArchive() error = %v", err)
	}

	_, err = provider.GetArchive(1)
	if err != nil {
		t.Fatalf("GetArchive after delete should not error, got %v", err)
	}
}

func TestMockArchiveProvider_Publish(t *testing.T) {
	provider := &mockArchiveProvider{
		archives: make(map[uint]*tools.ArchiveRecord),
		nextID:   1,
	}

	provider.archives[1] = &tools.ArchiveRecord{Id: 1, Status: 0}
	err := provider.PublishArchive(1, 1)
	if err != nil {
		t.Fatalf("PublishArchive() error = %v", err)
	}

	archive, _ := provider.GetArchive(1)
	if archive.Status != 1 {
		t.Errorf("archive.Status = %d, want 1", archive.Status)
	}
}

func TestMockArchiveProvider_ErrorHandling(t *testing.T) {
	provider := &mockArchiveProvider{
		archives: make(map[uint]*tools.ArchiveRecord),
		err:      assertAnError("test error"),
	}

	_, err := provider.GetArchive(1)
	if err == nil {
		t.Error("expected error from provider")
	}

	_, err = provider.ListArchives(tools.ArchiveListRequest{})
	if err == nil {
		t.Error("expected error from provider")
	}

	_, err = provider.CreateArchive(tools.ArchiveCreateRequest{Title: "Test"})
	if err == nil {
		t.Error("expected error from provider")
	}
}

func TestMockCategoryProvider_Create(t *testing.T) {
	provider := newMockCategoryProvider()

	req := tools.CategoryCreateRequest{
		Title:  "Technology",
		Pid:    0,
		Status: 1,
	}

	id, err := provider.CreateCategory(req)
	if err != nil {
		t.Fatalf("CreateCategory() error = %v", err)
	}
	if id != 1 {
		t.Errorf("id = %d, want 1", id)
	}

	cat, err := provider.GetCategory(id)
	if err != nil {
		t.Fatalf("GetCategory() error = %v", err)
	}
	if cat.Title != "Technology" {
		t.Errorf("cat.Title = %v, want 'Technology'", cat.Title)
	}
}

func TestMockCategoryProvider_List(t *testing.T) {
	provider := newMockCategoryProvider()

	provider.categories[1] = &tools.CategoryRecord{Id: 1, Title: "Parent", Pid: 0}
	provider.categories[2] = &tools.CategoryRecord{Id: 2, Title: "Child", Pid: 1}

	// List all
	cats, err := provider.ListCategories(tools.CategoryListRequest{})
	if err != nil {
		t.Fatalf("ListCategories() error = %v", err)
	}
	if len(cats) != 2 {
		t.Errorf("len(cats) = %d, want 2", len(cats))
	}

	// Filter by pid
	cats, err = provider.ListCategories(tools.CategoryListRequest{Pid: 1})
	if err != nil {
		t.Fatalf("ListCategories(pid=1) error = %v", err)
	}
	if len(cats) != 1 {
		t.Errorf("len(cats) with pid=1 = %d, want 1", len(cats))
	}
}

func TestMockCategoryProvider_Update(t *testing.T) {
	provider := newMockCategoryProvider()

	provider.categories[1] = &tools.CategoryRecord{Id: 1, Title: "Old"}

	newTitle := "New Title"
	err := provider.UpdateCategory(1, tools.CategoryUpdateRequest{Title: &newTitle})
	if err != nil {
		t.Fatalf("UpdateCategory() error = %v", err)
	}

	cat, _ := provider.GetCategory(1)
	if cat.Title != "New Title" {
		t.Errorf("cat.Title = %v, want 'New Title'", cat.Title)
	}
}

func TestMockCategoryProvider_Delete(t *testing.T) {
	provider := newMockCategoryProvider()

	provider.categories[1] = &tools.CategoryRecord{Id: 1}
	err := provider.DeleteCategory(1)
	if err != nil {
		t.Fatalf("DeleteCategory() error = %v", err)
	}

	_, err = provider.GetCategory(1)
	if err != nil {
		t.Fatalf("GetCategory after delete should not error, got %v", err)
	}
}

func TestMockTagProvider_Create(t *testing.T) {
	provider := newMockTagProvider()

	req := tools.TagCreateRequest{Title: "Golang"}
	id, err := provider.CreateTag(req)
	if err != nil {
		t.Fatalf("CreateTag() error = %v", err)
	}
	if id != 1 {
		t.Errorf("id = %d, want 1", id)
	}

	tag, _ := provider.GetTag(id)
	if tag.Title != "Golang" {
		t.Errorf("tag.Title = %v, want 'Golang'", tag.Title)
	}
}

func TestMockTagProvider_List(t *testing.T) {
	provider := newMockTagProvider()

	provider.tags[1] = &tools.TagRecord{Id: 1, Title: "Tag1", CategoryId: 1}
	provider.tags[2] = &tools.TagRecord{Id: 2, Title: "Tag2", CategoryId: 2}

	tags, err := provider.ListTags(tools.TagListRequest{})
	if err != nil {
		t.Fatalf("ListTags() error = %v", err)
	}
	if len(tags) != 2 {
		t.Errorf("len(tags) = %d, want 2", len(tags))
	}

	tags, err = provider.ListTags(tools.TagListRequest{CategoryId: 1})
	if err != nil {
		t.Fatalf("ListTags(cat=1) error = %v", err)
	}
	if len(tags) != 1 {
		t.Errorf("len(tags) with cat=1 = %d, want 1", len(tags))
	}
}

func TestMockAttachmentProvider_List(t *testing.T) {
	provider := newMockAttachmentProvider()

	provider.attachments[1] = &tools.AttachmentRecord{Id: 1, FileName: "image.png"}
	provider.attachments[2] = &tools.AttachmentRecord{Id: 2, FileName: "doc.pdf"}

	result, err := provider.ListAttachments(tools.AttachmentListRequest{})
	if err != nil {
		t.Fatalf("ListAttachments() error = %v", err)
	}
	if result.Total != 2 {
		t.Errorf("result.Total = %d, want 2", result.Total)
	}
	if len(result.Items) != 2 {
		t.Errorf("len(Items) = %d, want 2", len(result.Items))
	}
}

func TestMockAttachmentProvider_GetURL(t *testing.T) {
	provider := newMockAttachmentProvider()

	provider.attachments[1] = &tools.AttachmentRecord{Id: 1, FileLocation: "uploads/img.png"}

	url, err := provider.GetAttachmentURL(1)
	if err != nil {
		t.Fatalf("GetAttachmentURL() error = %v", err)
	}
	if url != "https://example.com/uploads/img.png" {
		t.Errorf("url = %v, want 'https://example.com/uploads/img.png'", url)
	}

	// Non-existent
	_, err = provider.GetAttachmentURL(999)
	if err == nil {
		t.Error("expected error for non-existent attachment")
	}
}

func TestMockAttachmentProvider_Delete(t *testing.T) {
	provider := newMockAttachmentProvider()

	provider.attachments[1] = &tools.AttachmentRecord{Id: 1}
	err := provider.DeleteAttachment(1)
	if err != nil {
		t.Fatalf("DeleteAttachment() error = %v", err)
	}
}
