package ports

import (
	"context"

	"github.com/antonkarounis/stoic/internal/domain/models"
)

type RegisterInput struct {
	Name  string
	Email string
}

type LinkUserInput struct {
	UserID      models.UserID
	Role        models.Role
}

type UnlinkUserInput struct {
	UserID models.UserID
}

type UpdateProfileInput struct {
	UserID models.UserID
	Name   string
}

type UserService interface {
	Register(ctx context.Context, input RegisterInput) (models.User, error)
	GetProfile(ctx context.Context, userID models.UserID) (models.User, error)
	UpdateProfile(ctx context.Context, input UpdateProfileInput) (models.User, error)
}
