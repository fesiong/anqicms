package tools

import (
	"testing"

	"kandaoni.com/anqicms/pkg/mcp/tools"
)

// MockTagProvider implements tools.TagProvider for testing
type MockTagProvider struct {
	tags   map[uint]*tools.TagRecord
	nextID uint
	err    error
}

func newMockTagProvider() *MockTagProvider {
	return &MockTagProvider{
		tags: make(map[uint]*tools.TagRecord),
		nextID: 1,
	}
}

func (m *MockTagProvider) GetTag(id uint) (*tools.TagRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	tag, ok := m.tags[id]
	if !ok {
		return nil, nil
	}
	return tag, nil
}

func (m *MockTagProvider) ListTags(req tools.TagListRequest) ([]tools.TagRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	var result []tools.TagRecord
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

func (m *MockTagProvider) CreateTag(req tools.TagCreateRequest) (uint, error) {
	if m.err != nil {
		return 0, m.err
	}
	tag := &tools.TagRecord{
		Id:       m.nextID,
		Title:    req.Title,
		Status:   req.Status,
		CategoryId: req.CategoryId,
	}
	m.tags[m.nextID] = tag
	id := m.nextID
	m.nextID++
	return id, nil
}

func (m *MockTagProvider) UpdateTag(id uint, req tools.TagUpdateRequest) error {
	if m.err != nil {
		return m.err
	}
	if tag, ok := m.tags[id]; ok {
		if req.Title != nil {
			tag.Title = *req.Title
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
		req     tools.TagCreateRequest
		wantErr bool
	}{
		{
			name:    "valid request",
			req:     tools.TagCreateRequest{Title: "Golang"},
			wantErr: false,
		},
		{
			name:    "empty title",
			req:     tools.TagCreateRequest{},
			wantErr: true,
		},
		{
			name:    "long title",
			req:     tools.TagCreateRequest{Title: string(make([]byte, 256))},
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
		req     tools.TagUpdateRequest
		wantErr bool
	}{
		{
			name:    "valid empty update",
			req:     tools.TagUpdateRequest{},
			wantErr: false,
		},
		{
			name:    "valid title update",
			req:     tools.TagUpdateRequest{Title: strPtr("New Tag")},
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
