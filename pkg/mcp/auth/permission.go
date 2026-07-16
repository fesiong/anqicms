package auth

import (
	"context"
	"fmt"
	"sync"
)

// Role represents a user role in the system
type Role string

const (
	RoleAdmin  Role = "admin"
	RoleEditor Role = "editor"
	RoleViewer Role = "viewer"
)

// Permission represents an action a role can perform
type Permission struct {
	Role   Role
	Action string // "create", "read", "update", "delete", "publish"
}

// PermissionChecker checks if a user has permission to perform an action
type PermissionChecker interface {
	CheckPermission(role Role, action string) bool
	GetUserRole(ctx context.Context) (Role, error)
}

// InMemoryPermissionChecker is a simple implementation for testing
type InMemoryPermissionChecker struct {
	mu          sync.RWMutex
	roles       map[string]Role
	permissions map[Permission]bool
	context     context.Context
	currentUser string
}

// NewInMemoryPermissionChecker creates a new in-memory permission checker
func NewInMemoryPermissionChecker(ctx context.Context) *InMemoryPermissionChecker {
	perms := map[Permission]bool{
		{RoleAdmin, "create"}:   true,
		{RoleAdmin, "read"}:     true,
		{RoleAdmin, "update"}:   true,
		{RoleAdmin, "delete"}:   true,
		{RoleAdmin, "publish"}:  true,
		{RoleEditor, "create"}:  true,
		{RoleEditor, "read"}:    true,
		{RoleEditor, "update"}:  true,
		{RoleEditor, "publish"}: true,
		{RoleViewer, "read"}:    true,
	}

	return &InMemoryPermissionChecker{
		roles:       make(map[string]Role),
		permissions: perms,
		context:     ctx,
	}
}

// SetUserRole sets the role for a user
func (p *InMemoryPermissionChecker) SetUserRole(userID string, role Role) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.roles[userID] = role
}

// SetPermission sets a permission
func (p *InMemoryPermissionChecker) SetPermission(role Role, action string, allowed bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.permissions[Permission{Role: role, Action: action}] = allowed
}

// CheckPermission checks if a role has permission for an action
func (p *InMemoryPermissionChecker) CheckPermission(role Role, action string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.permissions[Permission{Role: role, Action: action}]
}

// GetUserRole gets the role for the current user
func (p *InMemoryPermissionChecker) GetUserRole(ctx context.Context) (Role, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	role, exists := p.roles[p.currentUser]
	if !exists {
		return "", fmt.Errorf("user %s not found", p.currentUser)
	}
	return role, nil
}

// SetCurrentUser sets the current user ID for permission checking
func (p *InMemoryPermissionChecker) SetCurrentUser(userID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.currentUser = userID
}

// Context key type for passing user info
type contextKey string

const (
	ContextKeyUserID     contextKey = "userId"
	ContextKeyUserRole   contextKey = "userRole"
	ContextKeyPermission contextKey = "permission"
)

// UserContext provides user information in context
type UserContext struct {
	UserID string
	Role   Role
}

// NewUserContext creates a new user context
func NewUserContext(userID string, role Role) *UserContext {
	return &UserContext{
		UserID: userID,
		Role:   role,
	}
}

// ToContext adds user context to the given context
func (uc *UserContext) ToContext(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, ContextKeyUserID, uc.UserID)
	ctx = context.WithValue(ctx, ContextKeyUserRole, uc.Role)
	return ctx
}

// FromContext extracts user context from the given context
func FromContext(ctx context.Context) (*UserContext, error) {
	userID, ok := ctx.Value(ContextKeyUserID).(string)
	if !ok || userID == "" {
		return nil, fmt.Errorf("user ID not found in context")
	}

	role, ok := ctx.Value(ContextKeyUserRole).(Role)
	if !ok {
		role = RoleViewer // Default to viewer
	}

	return &UserContext{
		UserID: userID,
		Role:   role,
	}, nil
}

// RequirePermission is a middleware that checks permissions
func RequirePermission(checker PermissionChecker, action string) func(context.Context) error {
	return func(ctx context.Context) error {
		userCtx, err := FromContext(ctx)
		if err != nil {
			return fmt.Errorf("no user context: %w", err)
		}

		if checker == nil {
			return fmt.Errorf("permission checker not set")
		}

		if !checker.CheckPermission(userCtx.Role, action) {
			return fmt.Errorf("permission denied: role %s cannot perform %s", userCtx.Role, action)
		}

		return nil
	}
}

// MustBeAdmin checks if the current user is an admin
func MustBeAdmin(ctx context.Context) error {
	userCtx, err := FromContext(ctx)
	if err != nil {
		return err
	}

	if userCtx.Role != RoleAdmin {
		return fmt.Errorf("admin role required, got %s", userCtx.Role)
	}
	return nil
}

// ValidateRole validates a role string
func ValidateRole(role string) (Role, error) {
	switch Role(role) {
	case RoleAdmin, RoleEditor, RoleViewer:
		return Role(role), nil
	default:
		return "", fmt.Errorf("invalid role: %s", role)
	}
}
