package domain

import "errors"

var (
	ErrUserNotFound        = errors.New("user not found")
	ErrTeamNotFound        = errors.New("team not found")
	ErrTeamExists          = errors.New("team already exists")
	ErrPRNotFound          = errors.New("pull request not found")
	ErrPRExists            = errors.New("pull request already exists")
	ErrPRMerged            = errors.New("pull request is merged")
	ErrReviewerNotAssigned = errors.New("reviewer not assigned to this PR")
	ErrNoCandidates        = errors.New("no active candidates available")
)
