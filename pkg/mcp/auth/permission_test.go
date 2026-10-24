package auth

import (
	"testing"

	"kandaoni.com/anqicms/pkg/mcp/auth"
)

func TestHasPermission(t *testing.T) {
	tests := []struct {
		name       string
		userId     uint
		roleId     uint
		action     string
		resourceId uint
		perms      map[uint]*auth.RolePermission
		want       bool
		wantErr    bool
	}{
		{
			name:       "admin has all permissions",
			userId:     1,
			roleId:     1,
			action:     auth.ActionCreate,
			resourceId: 0,
			perms: map[uint]*auth.RolePermission{
				1: {RoleId: 1, Resource: "archive", Action: auth.ActionAll},
			},
			want:    true,
			wantErr: false,
		},
		{
			name:       "editor has create permission",
			userId:     2,
			roleId:     2,
			action:     auth.ActionCreate,
			resourceId: 0,
			perms: map[uint]*auth.RolePermission{
				2: {RoleId: 2, Resource: "archive", Action: auth.ActionCreate},
			},
			want:    true,
			wantErr: false,
		},
		{
			name:       "viewer lacks create permission",
			userId:     3,
			roleId:     3,
			action:     auth.ActionCreate,
			resourceId: 0,
			perms: map[uint]*auth.RolePermission{
				3: {RoleId: 3, Resource: "archive", Action: auth.ActionRead},
			},
			want:    false,
			wantErr: false,
		},
		{
			name:       "viewer can read",
			userId:     3,
			roleId:     3,
			action:     auth.ActionRead,
			resourceId: 0,
			perms: map[uint]*auth.RolePermission{
				3: {RoleId: 3, Resource: "archive", Action: auth.ActionRead},
			},
			want:    true,
			wantErr: false,
		},
		{
			name:       "editor has all permission on resource",
			userId:     4,
			roleId:     4,
			action:     auth.ActionUpdate,
			resourceId: 10,
			perms: map[uint]*auth.RolePermission{
				4: {RoleId: 4, Resource: "archive", Action: auth.ActionAll},
			},
			want:    true,
			wantErr: false,
		},
		{
			name:       "user with no role",
			userId:     99,
			roleId:     0,
			action:     auth.ActionCreate,
			resourceId: 0,
			perms:      map[uint]*auth.RolePermission{},
			want:       false,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			perm := NewPermissionManager(tt.perms)
			result, err := perm.HasPermission(tt.userId, tt.roleId, tt.action, tt.resourceId)
			if (err != nil) != tt.wantErr {
				t.Errorf("HasPermission() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != tt.want {
				t.Errorf("HasPermission() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestPermissionManager_CheckPermission(t *testing.T) {
	perms := map[uint]*auth.RolePermission{
		1: {RoleId: 1, Resource: "archive", Action: auth.ActionCreate},
		2: {RoleId: 1, Resource: "category", Action: auth.ActionAll},
	}
	perm := NewPermissionManager(perms)

	tests := []struct {
		name     string
		userId   uint
		roleId   uint
		resource string
		action   string
		want     bool
		wantErr  bool
	}{
		{
			name:     "admin create archive",
			userId:   1, roleId: 1, resource: "archive", action: auth.ActionCreate,
			want: true, wantErr: false,
		},
		{
			name:     "admin all on category",
			userId:   1, roleId: 1, resource: "category", action: auth.ActionDelete,
			want: true, wantErr: false,
		},
		{
			name:     "unknown role",
			userId:   99, roleId: 99, resource: "archive", action: auth.ActionCreate,
			want: false, wantErr: false,
		},
		{
			name:     "action not allowed",
			userId:   1, roleId: 1, resource: "category", action: auth.ActionDelete,
			want: false, wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := perm.CheckPermission(tt.userId, tt.roleId, tt.resource, tt.action)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckPermission() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != tt.want {
				t.Errorf("CheckPermission() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestPermissionManager_IsAdmin(t *testing.T) {
	tests := []struct {
		name     string
		roleId   uint
		perms    map[uint]*auth.RolePermission
		want     bool
	}{
		{"admin role 1", 1, map[uint]*auth.RolePermission{1: {RoleId: 1, Resource: "*", Action: "*"}}, true},
		{"admin role", 1, map[uint]*auth.RolePermission{}, true},
		{"non-admin role 2", 2, map[uint]*auth.RolePermission{}, false},
		{"zero role", 0, map[uint]*auth.RolePermission{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			perm := NewPermissionManager(tt.perms)
			if got := perm.IsAdmin(tt.roleId); got != tt.want {
				t.Errorf("IsAdmin(%d) = %v, want %v", tt.roleId, got, tt.want)
			}
		})
	}
}

func TestPermissionManager_CreateUserPermission(t *testing.T) {
	perm := NewPermissionManager(nil)

	tests := []struct {
		name       string
		userId     uint
		roleId     uint
		resource   string
		action     string
		wantErr    bool
	}{
		{"valid", 1, 2, "archive", auth.ActionCreate, false},
		{"invalid resource (too long)", 1, 2, string(make([]byte, 129)), auth.ActionCreate, true},
		{"invalid action (too long)", 1, 2, "archive", string(make([]byte, 65)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := perm.CreateUserPermission(tt.userId, tt.roleId, tt.resource, tt.action)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateUserPermission() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewPermissionManager(t *testing.T) {
	tests := []struct {
		name  string
		perms map[uint]*auth.RolePermission
	}{
		{"nil perms", nil},
		{"empty perms", map[uint]*auth.RolePermission{}},
		{"with perms", map[uint]*auth.RolePermission{1: {RoleId: 1, Resource: "archive", Action: auth.ActionCreate}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			perm := NewPermissionManager(tt.perms)
			if perm == nil {
				t.Error("NewPermissionManager returned nil")
			}
		})
	}
}
