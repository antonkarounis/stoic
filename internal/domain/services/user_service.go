package services

import (
	"context"
	"time"

	"github.com/antonkarounis/stoic/internal/domain/models"
	"github.com/antonkarounis/stoic/internal/domain/ports"
)

type userService struct {
	users ports.UserRepository
}

func NewUserService(users ports.UserRepository) ports.UserService {
	return &userService{users: users}
}

func (s *userService) Register(ctx context.Context, input ports.RegisterInput) (models.User, error) {
	u := models.User{
		ID:        models.UserID(newID()),
		Name:      input.Name,
		Email:     input.Email,
		Role:      models.RoleMember,
		CreatedAt: time.Now(),
	}
	if err := s.users.Save(ctx, u); err != nil {
		return models.User{}, err
	}
	return u, nil
}

func (s *userService) GetProfile(ctx context.Context, userID models.UserID) (models.User, error) {
	return s.users.FindByID(ctx, userID)
}

func (s *userService) UpdateProfile(ctx context.Context, input ports.UpdateProfileInput) (models.User, error) {
	u, err := s.users.FindByID(ctx, input.UserID)
	if err != nil {
		return models.User{}, err
	}
	u.Name = input.Name
	if err := s.users.Save(ctx, u); err != nil {
		return models.User{}, err
	}
	return u, nil
}
