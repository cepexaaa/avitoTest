package handler

import (
	"avito-test-task/internal/api"
	"avito-test-task/internal/domain"
	"log"
)

func buildError(code api.ErrorResponseErrorCode, message string) struct {
	Code    api.ErrorResponseErrorCode `json:"code"`
	Message string                     `json:"message"`
} {
	return struct {
		Code    api.ErrorResponseErrorCode `json:"code"`
		Message string                     `json:"message"`
	}{
		Code:    code,
		Message: message,
	}
}

func (h *ServerHandler) handleTeamError(err error) (api.PostTeamAddResponseObject, error) {
	switch err {
	case domain.ErrTeamExists:
		return api.PostTeamAdd400JSONResponse{
			Error: buildError(api.TEAMEXISTS, "Team creation failed"),
		}, nil
	default:
		log.Printf("Internal team error: %v", err)
		return api.PostTeamAdd400JSONResponse{
			Error: buildError(api.ErrorResponseErrorCode(err.Error()), "team_name already exists"),
		}, nil
	}
}

func (h *ServerHandler) handlePRError(err error) (api.PostPullRequestCreateResponseObject, error) {
	switch err {
	case domain.ErrUserNotFound:
		return api.PostPullRequestCreate404JSONResponse{
			Error: buildError(api.NOTFOUND, "Author not found"),
		}, nil
	case domain.ErrPRExists:
		return api.PostPullRequestCreate409JSONResponse{
			Error: buildError(api.PREXISTS, "PR id already exists"),
		}, nil
	case domain.ErrNoCandidates:
		return api.PostPullRequestCreate409JSONResponse{
			Error: buildError(api.NOCANDIDATE, "No candidates to PR"),
		}, nil
	default:
		log.Printf("Internal PR creation error: %v", err)
		return api.PostPullRequestCreate404JSONResponse{
			Error: buildError(api.ErrorResponseErrorCode(err.Error()), "Author/team not found"),
		}, nil
	}
}

func (h *ServerHandler) handlePRReassignError(err error) (api.PostPullRequestReassignResponseObject, error) {
	switch err {
	case domain.ErrPRNotFound:
		return api.PostPullRequestReassign404JSONResponse{
			Error: buildError(api.NOTFOUND, "PR not found"),
		}, nil
	case domain.ErrPRMerged:
		return api.PostPullRequestReassign409JSONResponse{
			Error: buildError(api.PRMERGED, "cannot reassign on merged PR"),
		}, nil
	case domain.ErrReviewerNotAssigned:
		return api.PostPullRequestReassign409JSONResponse{
			Error: buildError(api.NOTASSIGNED, "Reviewer is not assigned to this PR"),
		}, nil
	case domain.ErrNoCandidates:
		return api.PostPullRequestReassign409JSONResponse{
			Error: buildError(api.NOCANDIDATE, "No active replacement candidate in team"),
		}, nil
	default:
		log.Printf("Internal PR reassign error: %v", err)
		return api.PostPullRequestReassign404JSONResponse{
			Error: buildError(api.ErrorResponseErrorCode(err.Error()), "PR not found"),
		}, nil
	}
}
