package tools

import (
	"testing"
)

// MockTagProvider implements TagProvider for testing
type MockTagProvider struct {
	tags   map[uint]*TagRecord
	nextID uint
	err    error
}

func newMockTagProvider() *MockTagProvider {
	return &MockTagProvider{
		tags:   make(map[uint]*TagRecord),
		nextID: 1,
	}
}

func (m *MockTagProvider) GetTag(id uint) (*TagRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	tag, ok := m.tags[id]
	if !ok {
		return nil, nil
	}
	return tag, nil
}

func (m *MockTagProvider) ListTags(req TagListRequest) ([]TagRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	var result []TagRecord
	for _, t := range m.tags {
		if req.CategoryId > 0 && t.CategoryId != req.CategoryId {
			continue
		}
		if req.Status != nil && t.Status != *req.Status {
			continue
		}
		result = append(result, *t)
	}
	return result, nil
}

func (m *MockTagProvider) CreateTag(req TagCreateRequest) (uint, error) {
	if m.err != nil {
		return 0, m.err
	}
	tag := &TagRecord{
		Id:         m.nextID,
		Title:      req.Title,
		Status:     req.Status,
		CategoryId: req.CategoryId,
	}
	m.tags[m.nextID] = tag
	id := m.nextID
	m.nextID++
	return id, nil
}

func (m *MockTagProvider) UpdateTag(id uint, req TagUpdateRequest) error {
	if m.err != nil {
		return m.err
	}
	if tag, ok := m.tags[id]; ok {
		if req.Title != "" {
			tag.Title = req.Title
		}
	}
	return nil
}

func (m *MockTagProvider) DeleteTag(id uint) error {
	if m.err != nil {
		return m.err
	}
	delete(m.tags, id)
	return nil
}

func TestTagTools_GetAll(t *testing.T) {
	mock := newMockTagProvider()
	tagTools := NewTagTools(mock)
	defs := tagTools.GetAll()
	if len(defs) != 5 {
		t.Errorf("expected 5 tools, got %d", len(defs))
	}
	expected := []string{"tag_list", "tag_detail", "tag_create", "tag_update", "tag_delete"}
	for i, def := range defs {
		if def.Tool.Name != expected[i] {
			t.Errorf("tool %d: expected %s, got %s", i, expected[i], def.Tool.Name)
		}
	}
}

func TestValidateTagCreate(t *testing.T) {
	tests := []struct {
		name    string
		req     TagCreateRequest
		wantErr bool
	}{
		{
			name:    "valid request",
			req:     TagCreateRequest{Title: "Golang"},
			wantErr: false,
		},
		{
			name:    "empty title",
			req:     TagCreateRequest{},
			wantErr: true,
		},
		{
			name:    "long title",
			req:     TagCreateRequest{Title: string(make([]byte, 256))},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTagCreate(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTagCreate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateTagUpdate(t *testing.T) {
	tests := []struct {
		name    string
		req     TagUpdateRequest
		wantErr bool
	}{
		{
			name:    "valid empty update",
			req:     TagUpdateRequest{},
			wantErr: false,
		},
		{
			name:    "valid title update",
			req:     TagUpdateRequest{Title: "New Tag"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTagUpdate(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTagUpdate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
