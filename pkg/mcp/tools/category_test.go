package tools

import (
	"testing"

	"kandaoni.com/anqicms/pkg/mcp/tools"
)

// MockCategoryProvider implements tools.CategoryProvider for testing
type MockCategoryProvider struct {
	categories map[uint]*tools.CategoryRecord
	nextID     uint
	err        error
}

func newMockCategoryProvider() *MockCategoryProvider {
	return &MockCategoryProvider{
		categories: make(map[uint]*tools.CategoryRecord),
		nextID:     1,
	}
}

func (m *MockCategoryProvider) GetCategory(id uint) (*tools.CategoryRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	cat, ok := m.categories[id]
	if !ok {
		return nil, nil
	}
	return cat, nil
}

func (m *MockCategoryProvider) ListCategories(req tools.CategoryListRequest) ([]tools.CategoryRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	var result []tools.CategoryRecord
	for _, c := range m.categories {
		if req.Pid > 0 && c.Pid != req.Pid {
			continue
		}
		if req.Status != nil && c.Status != *req.Status {
			continue
		}
		result = append(result, *c)
	}
	return result, nil
}

func (m *MockCategoryProvider) CreateCategory(req tools.CategoryCreateRequest) (uint, error) {
	if m.err != nil {
		return 0, m.err
	}
	cat := &tools.CategoryRecord{
		Id:    m.nextID,
		Title: req.Title,
		Status: req.Status,
		Pid:   req.Pid,
	}
	m.categories[m.nextID] = cat
	id := m.nextID
	m.nextID++
	return id, nil
}

func (m *MockCategoryProvider) UpdateCategory(id uint, req tools.CategoryUpdateRequest) error {
	if m.err != nil {
		return m.err
	}
	if cat, ok := m.categories[id]; ok {
		if req.Title != nil {
			cat.Title = *req.Title
		}
		if req.Pid != nil {
			cat.Pid = *req.Pid
		}
	}
	return nil
}

func (m *MockCategoryProvider) DeleteCategory(id uint) error {
	if m.err != nil {
		return m.err
	}
	delete(m.categories, id)
	return nil
}

func (m *MockCategoryProvider) GetCategoryTree() ([]tools.CategoryRecord, error) {
	return nil, nil
}

func TestCategoryTools_GetAll(t *testing.T) {
	mock := newMockCategoryProvider()
	categoryTools := NewCategoryTools(mock)
	defs := categoryTools.GetAll()
	if len(defs) != 5 {
		t.Errorf("expected 5 tools, got %d", len(defs))
	}
	expected := []string{"category_list", "category_detail", "category_create", "category_update", "category_delete"}
	for i, def := range defs {
		if def.Tool.Name != expected[i] {
			t.Errorf("tool %d: expected %s, got %s", i, expected[i], def.Tool.Name)
		}
	}
}

func TestValidateCategoryCreate(t *testing.T) {
	tests := []struct {
		name    string
		req     tools.CategoryCreateRequest
		wantErr bool
	}{
		{
			name:    "valid request",
			req:     tools.CategoryCreateRequest{Title: "Technology", Pid: 0, Status: 1},
			wantErr: false,
		},
		{
			name:    "empty title",
			req:     tools.CategoryCreateRequest{Pid: 0},
			wantErr: true,
		},
		{
			name:    "long title",
			req:     tools.CategoryCreateRequest{Title: string(make([]byte, 256))},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCategoryCreate(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCategoryCreate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateCategoryUpdate(t *testing.T) {
	tests := []struct {
		name    string
		req     tools.CategoryUpdateRequest
		wantErr bool
	}{
		{
			name:    "valid empty update",
			req:     tools.CategoryUpdateRequest{},
			wantErr: false,
		},
		{
			name:    "valid title update",
			req:     tools.CategoryUpdateRequest{Title: strPtr("New Title")},
			wantErr: false,
		},
		{
			name:    "long title",
			req:     tools.CategoryUpdateRequest{Title: strPtr(string(make([]byte, 256)))},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCategoryUpdate(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCategoryUpdate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildCategoryTree(t *testing.T) {
	cats := []tools.CategoryRecord{
		{Id: 1, Title: "Parent 1", Pid: 0},
		{Id: 2, Title: "Child 1", Pid: 1},
		{Id: 3, Title: "Parent 2", Pid: 0},
		{Id: 4, Title: "Child 2", Pid: 1},
		{Id: 5, Title: "Standalone", Pid: 0},
	}

	tree := BuildCategoryTree(cats)

	if len(tree) != 3 {
		t.Errorf("expected 3 root categories, got %d", len(tree))
	}

	// Check Parent 1 has children
	for _, c := range tree {
		if c.Id == 1 && len(c.Children) != 2 {
			t.Errorf("Parent 1 expected 2 children, got %d", len(c.Children))
		}
		if c.Id == 3 && len(c.Children) != 0 {
			t.Errorf("Parent 2 expected 0 children, got %d", len(c.Children))
		}
	}
}
