package handler

import (
	"context"
	"log"

	"avito-test-task/internal/api"
	"avito-test-task/internal/domain"
	"avito-test-task/internal/usecase"
)

const (
	// There aren't enough any kinds of errors in openapi specification
	// In that cases UnexpectedError was returned
	UnexpectedError = "Unexpected Error"
)

type ServerHandler struct {
	teamUC *usecase.TeamUseCase
	userUC *usecase.UserUseCase
	prUC   *usecase.PRUseCase
}

func NewServerHandler(team *usecase.TeamUseCase, user *usecase.UserUseCase, pr *usecase.PRUseCase) *ServerHandler {
	return &ServerHandler{
		teamUC: team,
		userUC: user,
		prUC:   pr,
	}
}

// Implementation of all interface methods StrictServerInterface
func (h *ServerHandler) PostTeamAdd(ctx context.Context, request api.PostTeamAddRequestObject) (api.PostTeamAddResponseObject, error) {
	domainTeam := h.convertAPITeamToDomain(*request.Body)

	team, err := h.teamUC.CreateTeam(ctx, domainTeam)
	if err != nil {
		return h.handleTeamError(err)
	}

	return api.PostTeamAdd201JSONResponse{
		Team: h.convertDomainTeamToAPI(team),
	}, nil
}

func (h *ServerHandler) GetTeamGet(ctx context.Context, request api.GetTeamGetRequestObject) (api.GetTeamGetResponseObject, error) {
	team, err := h.teamUC.GetTeam(ctx, request.Params.TeamName)
	if err != nil {
		return api.GetTeamGet404JSONResponse{
			Error: buildError(api.NOTFOUND, "Team not found"),
		}, nil
	}

	return api.GetTeamGet200JSONResponse(*h.convertDomainTeamToAPI(team)), nil
}

func (h *ServerHandler) PostUsersSetIsActive(ctx context.Context, request api.PostUsersSetIsActiveRequestObject) (api.PostUsersSetIsActiveResponseObject, error) {
	user, err := h.userUC.SetUserActivity(ctx, request.Body.UserId, request.Body.IsActive)
	if err != nil {
		return api.PostUsersSetIsActive404JSONResponse{
			Error: buildError(api.NOTFOUND, "User not found"),
		}, nil
	}

	return api.PostUsersSetIsActive200JSONResponse{
		User: h.convertDomainUserToAPI(user),
	}, nil
}

func (h *ServerHandler) PostPullRequestCreate(ctx context.Context, request api.PostPullRequestCreateRequestObject) (api.PostPullRequestCreateResponseObject, error) {
	pr, err := h.prUC.CreatePR(ctx, request.Body.PullRequestId, request.Body.PullRequestName, request.Body.AuthorId)
	if err != nil {
		return h.handlePRError(err)
	}

	return api.PostPullRequestCreate201JSONResponse{
		Pr: h.convertDomainPRToAPI(pr),
	}, nil
}

func (h *ServerHandler) PostPullRequestMerge(ctx context.Context, request api.PostPullRequestMergeRequestObject) (api.PostPullRequestMergeResponseObject, error) {
	pr, err := h.prUC.MergePR(ctx, request.Body.PullRequestId)
	if err != nil {
		if err == domain.ErrPRNotFound {
			return api.PostPullRequestMerge404JSONResponse{
				Error: buildError(api.NOTFOUND, "PR not found"),
			}, nil
		}
		log.Printf("Internal error merging PR: %v", err)
		return api.PostPullRequestMerge404JSONResponse{
			Error: buildError(api.NOTFOUND, "Unexpected error in merging"),
		}, err
	}

	return api.PostPullRequestMerge200JSONResponse{
		Pr: h.convertDomainPRToAPI(pr),
	}, nil
}

func (h *ServerHandler) PostPullRequestReassign(ctx context.Context, request api.PostPullRequestReassignRequestObject) (api.PostPullRequestReassignResponseObject, error) {
	newReviewerID, err := h.prUC.ReassignReviewer(
		ctx,
		request.Body.PullRequestId,
		request.Body.OldUserId,
	)

	if err != nil {
		return h.handlePRReassignError(err)
	}

	pr, err := h.prUC.GetPR(ctx, request.Body.PullRequestId)
	if err != nil {
		if err == domain.ErrPRNotFound {
			return api.PostPullRequestReassign404JSONResponse{
				Error: buildError(api.NOTFOUND, "PR not found"),
			}, nil
		}
		log.Printf("Internal error getting PR: %v", err)
		return api.PostPullRequestReassign404JSONResponse{
			Error: buildError(UnexpectedError, "Unexpected error in reassigning"),
		}, err
	}

	return api.PostPullRequestReassign200JSONResponse{
		Pr:         *h.convertDomainPRToAPI(pr),
		ReplacedBy: newReviewerID,
	}, nil
}

func (h *ServerHandler) GetUsersGetReview(ctx context.Context, request api.GetUsersGetReviewRequestObject) (api.GetUsersGetReviewResponseObject, error) {
	prs, err := h.prUC.GetPRsByReviewer(ctx, request.Params.UserId)
	if err != nil {
		if err == domain.ErrUserNotFound {
			return api.GetUsersGetReview200JSONResponse{
				UserId:       request.Params.UserId,
				PullRequests: []api.PullRequestShort{},
			}, nil
		}
		log.Printf("Internal error getting user reviews: %v", err)
		return api.GetUsersGetReview200JSONResponse{
			UserId:       request.Params.UserId,
			PullRequests: []api.PullRequestShort{},
		}, nil
	}

	var apiPRs []api.PullRequestShort
	for _, pr := range prs {
		apiPRs = append(apiPRs, api.PullRequestShort{
			PullRequestId:   pr.ID,
			PullRequestName: pr.Title,
			AuthorId:        pr.AuthorID,
			Status:          api.PullRequestShortStatus(pr.Status),
		})
	}

	return api.GetUsersGetReview200JSONResponse{
		UserId:       request.Params.UserId,
		PullRequests: apiPRs,
	}, nil
}
