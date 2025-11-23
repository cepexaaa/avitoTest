package usecase

import (
	"context"
	"log"
	"math/rand"
	"time"

	"avito-test-task/internal/domain"
	pullrequest "avito-test-task/internal/repository/pull_request"
	"avito-test-task/internal/repository/team"
	"avito-test-task/internal/repository/user"
)

type PRUseCase struct {
	prRepo   pullrequest.PRRepository
	userRepo user.UserRepository
	teamRepo team.TeamRepository
}

func NewPRUseCase(prRepo pullrequest.PRRepository, userRepo user.UserRepository, teamRepo team.TeamRepository) *PRUseCase {
	return &PRUseCase{
		prRepo:   prRepo,
		userRepo: userRepo,
		teamRepo: teamRepo,
	}
}

func (uc *PRUseCase) CreatePR(ctx context.Context, prID, title, authorID string) (*domain.PullRequest, error) {
	author, err := uc.userRepo.FindByID(ctx, authorID)
	if err != nil {
		log.Printf("Error searching author: %v", err)
		return nil, domain.ErrUserNotFound
	}

	reviewers, err := uc.autoAssignReviewers(ctx, author.TeamID, authorID)
	if err != nil {
		log.Printf("Error in autoAssignReviewers: %v", err)
		return nil, err
	}

	log.Println(reviewers)

	pr := &domain.PullRequest{
		ID:                prID,
		Title:             title,
		AuthorID:          authorID,
		Status:            domain.PRStatusOpen,
		AssignedReviewers: reviewers,
	}

	if err := uc.prRepo.SavePR(ctx, pr); err != nil {
		return nil, err
	}

	return pr, nil
}

func (uc *PRUseCase) GetPR(ctx context.Context, id string) (*domain.PullRequest, error) {
	return uc.prRepo.FindByID(ctx, id)
}

func (uc *PRUseCase) MergePR(ctx context.Context, prID string) (*domain.PullRequest, error) {
	pr, err := uc.prRepo.FindByID(ctx, prID)
	if err != nil {
		return nil, err
	}

	if pr.Status == domain.PRStatusMerged {
		return pr, nil // idempotence
	}

	now := time.Now()
	pr.Status = domain.PRStatusMerged
	pr.MergedAt = &now

	if err := uc.prRepo.UpdateStatus(ctx, prID, domain.PRStatusMerged, &now); err != nil {
		return nil, err
	}

	return pr, nil
}

func (uc *PRUseCase) ReassignReviewer(ctx context.Context, prID, oldReviewerID string) (string, error) {
	pr, err := uc.prRepo.FindByID(ctx, prID)
	if err != nil {
		return "", err
	}

	if pr.Status == domain.PRStatusMerged {
		return "", domain.ErrPRMerged
	}

	isAssigned := false
	for _, reviewer := range pr.AssignedReviewers {
		if reviewer == oldReviewerID {
			isAssigned = true
			break
		}
	}
	if !isAssigned {
		return "", domain.ErrReviewerNotAssigned
	}

	oldReviewer, err := uc.userRepo.FindByID(ctx, oldReviewerID)
	if err != nil {
		return "", err
	}

	newReviewerID, err := uc.selectRandomReviewer(ctx, pr, oldReviewer.TeamID, oldReviewerID)
	if err != nil {
		return "", err
	}

	if err := uc.prRepo.ReplaceReviewer(ctx, prID, oldReviewerID, newReviewerID); err != nil {
		return "", err
	}

	return newReviewerID, nil
}

func (uc *PRUseCase) GetPRsByReviewer(ctx context.Context, reviewerID string) ([]*domain.PullRequest, error) {
	return uc.prRepo.FindByReviewerID(ctx, reviewerID)
}

func (uc *PRUseCase) autoAssignReviewers(ctx context.Context, teamID int, excludeUserID string) ([]string, error) {
	candidates, err := uc.userRepo.FindActiveByTeamID(ctx, teamID, excludeUserID)
	if err != nil {
		return nil, err
	}

	if len(candidates) == 0 {
		return []string{}, domain.ErrNoCandidates
	}

	rand.Seed(time.Now().UnixNano())
	indexes := make([]int, 1, 2)
	indexes[0] = rand.Intn(len(candidates))
	if len(candidates) > 1 {
		indexes = append(indexes, rand.Intn(len(candidates)))
	}
	for len(indexes) > 1 && indexes[0] == indexes[1] {
		indexes[1] = rand.Intn(len(candidates))
	}

	reviewers := make([]string, len(indexes))
	for i := 0; i < len(indexes); i++ {
		reviewers[i] = candidates[i].ID
	}

	return reviewers, nil
}

func (uc *PRUseCase) selectRandomReviewer(ctx context.Context, pr *domain.PullRequest, teamID int, excludeUserID string) (string, error) {
	candidates, err := uc.userRepo.FindActiveByTeamID(ctx, teamID, excludeUserID)
	if err != nil {
		return "", err
	}

	availableCandidates := make([]*domain.User, 0)
	for _, candidate := range candidates {
		isAlreadyReviewer := false
		for _, reviewer := range pr.AssignedReviewers {
			if reviewer == candidate.ID {
				isAlreadyReviewer = true
				break
			}
		}
		if !isAlreadyReviewer {
			availableCandidates = append(availableCandidates, candidate)
		}
	}

	if len(availableCandidates) == 0 {
		return "", domain.ErrNoCandidates
	}

	rand.Seed(time.Now().UnixNano())
	selected := availableCandidates[rand.Intn(len(availableCandidates))]
	return selected.ID, nil
}
