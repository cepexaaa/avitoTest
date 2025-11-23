package usecase

import (
	"context"

	"avito-test-task/internal/domain"
	"avito-test-task/internal/repository/team"
	"avito-test-task/internal/repository/user"
)

type TeamUseCase struct {
	teamRepo team.TeamRepository
	userRepo user.UserRepository
}

func NewTeamUseCase(teamRepo team.TeamRepository, userRepo user.UserRepository) *TeamUseCase {
	return &TeamUseCase{
		teamRepo: teamRepo,
		userRepo: userRepo,
	}
}

func (uc *TeamUseCase) CreateTeam(ctx context.Context, team *domain.Team) (*domain.Team, error) {
	if err := uc.teamRepo.SaveTeam(ctx, team); err != nil {
		return nil, err
	}

	for _, member := range team.Members {
		user := uc.member2user(&member, team.ID, team.Name)
		if err := uc.userRepo.SaveUser(ctx, &user); err != nil {
			return nil, err
		}
		team.Members = append(team.Members, uc.user2member(&user))
	}

	return team, nil
}

func (uc *TeamUseCase) GetTeam(ctx context.Context, teamName string) (*domain.Team, error) {
	team, err := uc.teamRepo.FindByName(ctx, teamName)
	if err != nil {
		return nil, err
	}

	users, err := uc.userRepo.FindByTeamID(ctx, team.ID)
	if err != nil {
		return nil, err
	}

	team.Members = Map(users, uc.user2member)
	return team, nil
}

func (uc *TeamUseCase) user2member(u *domain.User) domain.TeamMember {
	return domain.TeamMember{
		UserID:   u.ID,
		Username: u.Username,
		IsActive: u.IsActive,
	}
}

func (uc *TeamUseCase) member2user(m *domain.TeamMember, teamID int, teamName string) domain.User {
	return domain.User{
		ID:       m.UserID,
		Username: m.Username,
		TeamID:   teamID,
		TeamName: teamName,
		IsActive: m.IsActive,
	}

}

func Map[T any, R any](items []T, f func(T) R) []R {
	result := make([]R, len(items))
	for i, v := range items {
		result[i] = f(v)
	}
	return result
}
