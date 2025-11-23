package handler

import (
	"avito-test-task/internal/api"
	"avito-test-task/internal/domain"
)

func (h *ServerHandler) convertAPITeamToDomain(apiTeam api.Team) *domain.Team {
	team := &domain.Team{
		Name: apiTeam.TeamName,
	}

	for _, member := range apiTeam.Members {
		team.Members = append(team.Members, domain.TeamMember{
			UserID:   member.UserId,
			Username: member.Username,
			IsActive: member.IsActive,
		})
	}

	return team
}

func (h *ServerHandler) convertDomainTeamToAPI(team *domain.Team) *api.Team {
	var members []api.TeamMember
	for _, user := range team.Members {
		members = append(members, api.TeamMember{
			UserId:   user.UserID,
			Username: user.Username,
			IsActive: user.IsActive,
		})
	}

	return &api.Team{
		TeamName: team.Name,
		Members:  members,
	}
}

func (h *ServerHandler) convertDomainPRToAPI(pr *domain.PullRequest) *api.PullRequest {
	return &api.PullRequest{
		PullRequestId:     pr.ID,
		PullRequestName:   pr.Title,
		AuthorId:          pr.AuthorID,
		Status:            api.PullRequestStatus(pr.Status),
		AssignedReviewers: pr.AssignedReviewers,
		CreatedAt:         pr.CreatedAt,
		MergedAt:          pr.MergedAt,
	}
}

func (h *ServerHandler) convertDomainUserToAPI(user *domain.User) *api.User {
	return &api.User{
		UserId:   user.ID,
		Username: user.Username,
		TeamName: user.TeamName,
		IsActive: user.IsActive,
	}

}
