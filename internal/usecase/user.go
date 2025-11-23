package usecase

import (
	"context"

	"avito-test-task/internal/domain"
	"avito-test-task/internal/repository/user"
)

type UserUseCase struct {
	userRepo user.UserRepository
}

func NewUserUseCase(userRepo user.UserRepository) *UserUseCase {
	return &UserUseCase{userRepo: userRepo}
}

func (uc *UserUseCase) SetUserActivity(ctx context.Context, userID string, isActive bool) (*domain.User, error) {
	user, err := uc.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if err := uc.userRepo.UpdateActivity(ctx, userID, isActive); err != nil {
		return nil, err
	}

	user.IsActive = isActive
	return user, nil
}
