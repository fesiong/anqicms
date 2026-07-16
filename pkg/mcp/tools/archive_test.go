package tools

import (
	"errors"
	"testing"

	"kandaoni.com/anqicms/pkg/mcp/tools"
)

// MockArchiveProvider implements tools.ArchiveProvider for testing
type MockArchiveProvider struct {
	archives map[uint]*tools.ArchiveRecord
	nextID   uint
	err      error
}

func newMockArchiveProvider() *MockArchiveProvider {
	return &MockArchiveProvider{
		archives: make(map[uint]*tools.ArchiveRecord),
		nextID:   1,
	}
}

func (m *MockArchiveProvider) GetArchive(id uint) (*tools.ArchiveRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	archive, ok := m.archives[id]
	if !ok {
		return nil, nil
	}
	return archive, nil
}

func (m *MockArchiveProvider) ListArchives(req tools.ArchiveListRequest) (*tools.ArchiveListResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	var items []tools.ArchiveRecord
	for _, a := range m.archives {
		if req.CategoryId > 0 && a.CategoryId != req.CategoryId {
			continue
		}
		if req.Status != nil && a.Status != *req.Status {
			continue
		}
		items = append(items, *a)
	}
	if len(items) == 0 {
		return &tools.ArchiveListResult{Total: 0, Items: []tools.ArchiveRecord{}}, nil
	}
	// Simple pagination
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	start := (page - 1) * pageSize
	if start >= len(items) {
		return &tools.ArchiveListResult{Total: len(items), Items: []tools.ArchiveRecord{}}, nil
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return &tools.ArchiveListResult{
		Total:    len(items),
		Page:     page,
		PageSize: pageSize,
		Items:    items[start:end],
	}, nil
}

func (m *MockArchiveProvider) CreateArchive(req tools.ArchiveCreateRequest) (uint, error) {
	if m.err != nil {
		return 0, m.err
	}
	archive := &tools.ArchiveRecord{
		Id:         m.nextID,
		Title:      req.Title,
		Content:    req.Content,
		CategoryId: req.CategoryId,
		Status:     req.Status,
	}
	m.archives[m.nextID] = archive
	id := m.nextID
	m.nextID++
	return id, nil
}

func (m *MockArchiveProvider) UpdateArchive(id uint, req tools.ArchiveUpdateRequest) error {
	if m.err != nil {
		return m.err
	}
	if archive, ok := m.archives[id]; ok {
		if req.Title != nil {
			archive.Title = *req.Title
		}
		if req.Content != nil {
			archive.Content = *req.Content
		}
		if req.CategoryId != nil {
			archive.CategoryId = *req.CategoryId
		}
	}
	return nil
}

func (m *MockArchiveProvider) DeleteArchive(id uint) error {
	if m.err != nil {
		return m.err
	}
	delete(m.archives, id)
	return nil
}

func (m *MockArchiveProvider) PublishArchive(id uint, status uint) error {
	if m.err != nil {
		return m.err
	}
	if archive, ok := m.archives[id]; ok {
		archive.Status = status
	}
	return nil
}

func TestArchiveTools_GetAll(t *testing.T) {
	mock := newMockArchiveProvider()
	tools := NewArchiveTools(mock)
	defs := tools.GetAll()
	if len(defs) != 6 {
		t.Errorf("expected 6 tools, got %d", len(defs))
	}
	expected := []string{"archive_list", "archive_get", "archive_create", "archive_update", "archive_delete", "archive_publish"}
	for i, def := range defs {
		if def.Tool.Name != expected[i] {
			t.Errorf("tool %d: expected %s, got %s", i, expected[i], def.Tool.Name)
		}
		if def.Tool.Description == "" {
			t.Errorf("tool %s has empty description", def.Tool.Name)
		}
	}
}

func TestArchiveTools_CreateArchive(t *testing.T) {
	mock := newMockArchiveProvider()
	archiveTools := NewArchiveTools(mock)
	defs := archiveTools.GetAll()
	createHandler := defs[2].Handler

	req := &MockToolRequest{
		params: map[string]any{
			"title":        "Test Title",
			"content":      "Test Content",
			"category_id":  1,
			"keywords":     "test,key",
			"status":       uint(1),
			"tag_ids":      []uint{1, 2},
		},
	}

	result, err := createHandler(nil, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}

	content := result.GetContent()
	if len(content) == 0 {
		t.Error("no content in result")
	}
}

func TestArchiveTools_ListArchives(t *testing.T) {
	mock := newMockArchiveProvider()
	// Add some archives
	mock.archives[1] = &tools.ArchiveRecord{Id: 1, Title: "Article 1", CategoryId: 1, Status: 1}
	mock.archives[2] = &tools.ArchiveRecord{Id: 2, Title: "Article 2", CategoryId: 1, Status: 1}
	mock.archives[3] = &tools.ArchiveRecord{Id: 3, Title: "Draft Article", CategoryId: 1, Status: 0}

	archiveTools := NewArchiveTools(mock)
	defs := archiveTools.GetAll()
	listHandler := defs[0].Handler

	req := &MockToolRequest{
		params: map[string]any{
			"page":       1,
			"page_size":  10,
			"category_id": 1,
		},
	}

	result, err := listHandler(nil, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}

	content := result.GetContent()
	if len(content) == 0 {
		t.Error("no content in result")
	}
}

func TestArchiveTools_GetArchive(t *testing.T) {
	mock := newMockArchiveProvider()
	mock.archives[1] = &tools.ArchiveRecord{Id: 1, Title: "Test Archive"}

	archiveTools := NewArchiveTools(mock)
	defs := archiveTools.GetAll()
	getHandler := defs[1].Handler

	req := &MockToolRequest{
		params: map[string]any{
			"archive_id": 1,
		},
	}

	result, err := getHandler(nil, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}

	content := result.GetContent()
	if len(content) == 0 {
		t.Error("no content in result")
	}
}

func TestArchiveTools_UpdateArchive(t *testing.T) {
	mock := newMockArchiveProvider()
	mock.archives[1] = &tools.ArchiveRecord{Id: 1, Title: "Old Title"}

	archiveTools := NewArchiveTools(mock)
	defs := archiveTools.GetAll()
	updateHandler := defs[3].Handler

	req := &MockToolRequest{
		params: map[string]any{
			"archive_id": 1,
			"title":      "New Title",
		},
	}

	result, err := updateHandler(nil, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
}

func TestArchiveTools_DeleteArchive(t *testing.T) {
	mock := newMockArchiveProvider()
	mock.archives[1] = &tools.ArchiveRecord{Id: 1, Title: "To Delete"}

	archiveTools := NewArchiveTools(mock)
	defs := archiveTools.GetAll()
	deleteHandler := defs[4].Handler

	req := &MockToolRequest{
		params: map[string]any{
			"archive_id": 1,
		},
	}

	result, err := deleteHandler(nil, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
}

func TestArchiveTools_PublishArchive(t *testing.T) {
	mock := newMockArchiveProvider()
	mock.archives[1] = &tools.ArchiveRecord{Id: 1, Title: "Draft", Status: 0}

	archiveTools := NewArchiveTools(mock)
	defs := archiveTools.GetAll()
	publishHandler := defs[5].Handler

	req := &MockToolRequest{
		params: map[string]any{
			"archive_id": 1,
			"status":     1,
		},
	}

	result, err := publishHandler(nil, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
}

func TestValidateArchiveCreate(t *testing.T) {
	tests := []struct {
		name    string
		req     tools.ArchiveCreateRequest
		wantErr bool
	}{
		{
			name:    "valid request",
			req:     tools.ArchiveCreateRequest{Title: "Test", Content: "Content", CategoryId: 1},
			wantErr: false,
		},
		{
			name:    "empty title",
			req:     tools.ArchiveCreateRequest{Content: "Content", CategoryId: 1},
			wantErr: true,
		},
		{
			name:    "long title",
			req:     tools.ArchiveCreateRequest{Title: string(make([]byte, 256)), Content: "Content", CategoryId: 1},
			wantErr: true,
		},
		{
			name:    "empty content",
			req:     tools.ArchiveCreateRequest{Title: "Test", Content: "", CategoryId: 1},
			wantErr: true,
		},
		{
			name:    "zero category",
			req:     tools.ArchiveCreateRequest{Title: "Test", Content: "Content"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateArchiveCreate(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateArchiveCreate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateArchiveUpdate(t *testing.T) {
	tests := []struct {
		name    string
		req     tools.ArchiveUpdateRequest
		wantErr bool
	}{
		{
			name:    "valid empty update",
			req:     tools.ArchiveUpdateRequest{},
			wantErr: false,
		},
		{
			name:    "valid title update",
			req:     tools.ArchiveUpdateRequest{Title: strPtr("New Title")},
			wantErr: false,
		},
		{
			name:    "long title",
			req:     tools.ArchiveUpdateRequest{Title: strPtr(string(make([]byte, 256)))},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateArchiveUpdate(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateArchiveUpdate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFilterArchives(t *testing.T) {
	archives := []tools.ArchiveRecord{
		{Id: 1, Title: "Go Programming", CategoryId: 1, Status: 1, Content: "Golang is great"},
		{Id: 2, Title: "Python Tutorial", CategoryId: 2, Status: 1, Content: "Python is easy"},
		{Id: 3, Title: "Draft Article", CategoryId: 1, Status: 0, Content: "Work in progress"},
	}

	tests := []struct {
		name       string
		keyword    string
		cid        uint
		status     *uint
		wantCount  int
	}{
		{"all", "", 0, nil, 3},
		{"filter by keyword", "go", 0, nil, 2}, // "Go" and "Golang"
		{"filter by category", "", 1, nil, 2},
		{"filter by status", "", 0, uintPtr(1), 2},
		{"filter by keyword and category", "python", 2, nil, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterArchives(archives, tt.keyword, tt.cid, tt.status)
			if len(result) != tt.wantCount {
				t.Errorf("FilterArchives() = %d, want %d", len(result), tt.wantCount)
			}
		})
	}
}

func TestPaginateArchives(t *testing.T) {
	archives := []tools.ArchiveRecord{
		{Id: 1, Title: "A1"}, {Id: 2, Title: "A2"}, {Id: 3, Title: "A3"},
		{Id: 4, Title: "A4"}, {Id: 5, Title: "A5"},
	}

	tests := []struct {
		name     string
		page     int
		pageSize int
		wantLen  int
	}{
		{"page 1, size 2", 1, 2, 2},
		{"page 2, size 2", 2, 2, 2},
		{"page 3, size 2", 3, 2, 1},
		{"page 4, size 2", 4, 2, 0},
		{"invalid page", 0, 2, 2},
		{"large page size", 1, 100, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PaginateArchives(archives, tt.page, tt.pageSize)
			if len(result) != tt.wantLen {
				t.Errorf("PaginateArchives() = %d, want %d", len(result), tt.wantLen)
			}
		})
	}
}

func TestArchiveTools_ErrorHandling(t *testing.T) {
	mock := newMockArchiveProvider()
	mock.err = assertAnError("provider error")

	archiveTools := NewArchiveTools(mock)
	defs := archiveTools.GetAll()

	// Test error propagation for each handler
	tests := []struct {
		handler mcpToolHandler
		params  map[string]any
		name    string
	}{
		{defs[0].Handler, map[string]any{}, "list"},
		{defs[1].Handler, map[string]any{"archive_id": 1}, "get"},
		{defs[2].Handler, map[string]any{"title": "Test", "content": "Content", "category_id": 1}, "create"},
		{defs[3].Handler, map[string]any{"archive_id": 1}, "update"},
		{defs[4].Handler, map[string]any{"archive_id": 1}, "delete"},
		{defs[5].Handler, map[string]any{"archive_id": 1, "status": 1}, "publish"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.handler(nil, &MockToolRequest{params: tt.params})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("result is nil")
			}
			// Error results should have isError flag
		})
	}
}

// Helper types
type MockToolRequest struct {
	params map[string]any
}

func (m *MockToolRequest) UnmarshalParams(v any) error {
	// Simple conversion from map to struct via JSON
	// For testing purposes, we directly set values
	switch typed := v.(type) {
	case *tools.ArchiveCreateRequest:
		if v, ok := m.params["title"]; ok {
			typed.Title = v.(string)
		}
		if v, ok := m.params["content"]; ok {
			typed.Content = v.(string)
		}
		if v, ok := m.params["category_id"]; ok {
			typed.CategoryId = v.(uint)
		}
		if v, ok := m.params["status"]; ok {
			typed.Status = v.(uint)
		}
	case *tools.ArchiveListRequest:
		if v, ok := m.params["page"]; ok {
			typed.Page = v.(int)
		}
		if v, ok := m.params["page_size"]; ok {
			typed.PageSize = v.(int)
		}
		if v, ok := m.params["category_id"]; ok {
			typed.CategoryId = v.(uint)
		}
	case *struct {
		ArchiveId uint `json:"archive_id"`
	}:
		if v, ok := m.params["archive_id"]; ok {
			typed.ArchiveId = v.(uint)
		}
	case *struct {
		ArchiveId uint                     `json:"archive_id"`
		tools.ArchiveUpdateRequest
	}:
		if v, ok := m.params["archive_id"]; ok {
			typed.ArchiveId = v.(uint)
		}
		if v, ok := m.params["title"]; ok {
			typed.Title = strPtr(v.(string))
		}
	case *struct {
		ArchiveId uint `json:"archive_id"`
		Status    uint `json:"status"`
	}:
		if v, ok := m.params["archive_id"]; ok {
			typed.ArchiveId = v.(uint)
		}
		if v, ok := m.params["status"]; ok {
			typed.Status = v.(uint)
		}
	}
	return nil
}

type mcpToolHandler func(ctx interface{}, req interface{}) (interface{}, error)

func strPtr(s string) *string { return &s }
func uintPtr(u uint) *uint   { return &u }
func assertAnError(s string) error { return errors.New(s) }
