package tools

import (
	"testing"

	"kandaoni.com/anqicms/pkg/mcp/tools"
)

// MockAttachmentProvider implements tools.AttachmentProvider for testing
type MockAttachmentProvider struct {
	attachments map[uint]*tools.AttachmentRecord
	nextID      uint
	err         error
}

func newMockAttachmentProvider() *MockAttachmentProvider {
	return &MockAttachmentProvider{
		attachments: make(map[uint]*tools.AttachmentRecord),
		nextID:      1,
	}
}

func (m *MockAttachmentProvider) GetAttachment(id uint) (*tools.AttachmentRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	att, ok := m.attachments[id]
	if !ok {
		return nil, nil
	}
	return att, nil
}

func (m *MockAttachmentProvider) ListAttachments(req tools.AttachmentListRequest) (*tools.AttachmentListResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	var items []tools.AttachmentRecord
	for _, a := range m.attachments {
		if req.CategoryId > 0 && a.CategoryId != req.CategoryId {
			continue
		}
		if req.UserId > 0 && a.UserId != req.UserId {
			continue
		}
		if req.Status != nil && a.Status != *req.Status {
			continue
		}
		items = append(items, *a)
	}
	if len(items) == 0 {
		return &tools.AttachmentListResult{Total: 0, Items: []tools.AttachmentRecord{}}, nil
	}
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start >= len(items) {
		return &tools.AttachmentListResult{Total: len(items), Items: []tools.AttachmentRecord{}}, nil
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return &tools.AttachmentListResult{
		Total:    len(items),
		Page:     page,
		PageSize: pageSize,
		Items:    items[start:end],
	}, nil
}

func (m *MockAttachmentProvider) DeleteAttachment(id uint) error {
	if m.err != nil {
		return m.err
	}
	delete(m.attachments, id)
	return nil
}

func (m *MockAttachmentProvider) GetAttachmentURL(id uint) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	if att, ok := m.attachments[id]; ok {
		return "https://example.com/" + att.FileLocation, nil
	}
	return "", assertAnError("not found")
}

func TestAttachmentTools_GetAll(t *testing.T) {
	mock := newMockAttachmentProvider()
	tools := NewAttachmentTools(mock)
	defs := tools.GetAll()
	if len(defs) != 3 {
		t.Errorf("expected 3 tools, got %d", len(defs))
	}
	expected := []string{"attachment_list", "attachment_delete", "attachment_url"}
	for i, def := range defs {
		if def.Tool.Name != expected[i] {
			t.Errorf("tool %d: expected %s, got %s", i, expected[i], def.Tool.Name)
		}
	}
}

func TestValidateAttachmentName(t *testing.T) {
	tests := []struct {
		name    string
		fileName string
		wantErr bool
	}{
		{
			name:    "valid name",
			fileName: "image.png",
			wantErr: false,
		},
		{
			name:    "empty name",
			fileName: "",
			wantErr: true,
		},
		{
			name:    "long name",
			fileName: string(make([]byte, 251)),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAttachmentName(tt.fileName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAttachmentName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
