package models

import "time"

type UserID string

type Role string

const (
	RoleOwner  Role = "owner"
	RoleMember Role = "member"
)

type User struct {
	ID          UserID
	Name        string
	Email       string
	Role        Role
	CreatedAt   time.Time
}

// Identity represents a linked OIDC account. It holds only the OIDC-specific
// fields needed to map an auth provider subject to a domain User.
type Identity struct {
	ID      int64
	AuthSub string
	UserID  *UserID // nil if not yet linked to a domain user
}
