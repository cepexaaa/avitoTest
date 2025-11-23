package usecase

import (
	"avito-test-task/internal/domain"
	"context"
	"testing"
	"time"
)

func TestPRUseCase_CreatePR(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		prID           string
		title          string
		authorID       string
		setupData      func()
		expectedError  error
		expectedStatus domain.PRStatus
		description    string
	}{
		{
			name:     "successfully create PR with auto-assigned reviewers",
			prID:     "pr_new_1",
			title:    "New feature implementation",
			authorID: "user_1",
			setupData: func() {

			},
			expectedError:  nil,
			expectedStatus: domain.PRStatusOpen,
			description:    "should create PR and auto-assign reviewers from same team",
		},
		{
			name:     "create PR with non-existent author",
			prID:     "pr_invalid_1",
			title:    "Invalid PR",
			authorID: "non_existent_user",
			setupData: func() {

			},
			expectedError:  domain.ErrUserNotFound,
			expectedStatus: "",
			description:    "should fail when author does not exist",
		},
		{
			name:     "create PR when no reviewers available",
			prID:     "pr_no_reviewers",
			title:    "PR without reviewers",
			authorID: "user_2",
			setupData: func() {

				testDB.Exec("UPDATE users SET is_active = false WHERE id != 'user_2'")
			},
			expectedError:  domain.ErrNoCandidates,
			expectedStatus: "",
			description:    "should fail when no active reviewers available in team",
		},
		{
			name:     "create PR with duplicate ID",
			prID:     "pr_duplicate",
			title:    "Duplicate PR",
			authorID: "user_1",
			setupData: func() {

				testDB.Exec(`
					INSERT INTO pull_requests (id, title, author_id, status) 
					VALUES ('pr_duplicate', 'First PR', 'user_1', 'OPEN')
				`)
			},
			expectedError:  nil,
			expectedStatus: domain.PRStatusOpen,
			description:    "should update existing PR when ID duplicates",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupTestData(t)
			tt.setupData()

			result, err := prUseCase.CreatePR(ctx, tt.prID, tt.title, tt.authorID)

			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("Expected error %v, got nil", tt.expectedError)
				} else if err != tt.expectedError {
					t.Errorf("Expected error %v, got %v", tt.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			if tt.expectedError == nil {
				if result == nil {
					t.Error("Expected PR object, got nil")
					return
				}

				if result.ID != tt.prID {
					t.Errorf("PR ID mismatch: got %s, want %s", result.ID, tt.prID)
				}
				if result.Title != tt.title {
					t.Errorf("PR title mismatch: got %s, want %s", result.Title, tt.title)
				}
				if result.AuthorID != tt.authorID {
					t.Errorf("PR author ID mismatch: got %s, want %s", result.AuthorID, tt.authorID)
				}
				if result.Status != tt.expectedStatus {
					t.Errorf("PR status mismatch: got %s, want %s", result.Status, tt.expectedStatus)
				}

				dbPR, err := prRepo.FindByID(ctx, tt.prID)
				if err != nil {
					t.Errorf("Failed to verify PR in DB: %v", err)
					return
				}

				if dbPR.Title != tt.title {
					t.Errorf("PR in DB has title %s, expected %s", dbPR.Title, tt.title)
				}

				if tt.expectedError == nil && tt.authorID != "user_2" {
					if len(result.AssignedReviewers) == 0 {
						t.Error("PR should have assigned reviewers")
					}

					if len(dbPR.AssignedReviewers) != len(result.AssignedReviewers) {
						t.Errorf("DB has %d reviewers, expected %d", len(dbPR.AssignedReviewers), len(result.AssignedReviewers))
					}

					author, err := userRepo.FindByID(ctx, tt.authorID)
					if err != nil {
						t.Errorf("Failed to get author: %v", err)
						return
					}

					for _, reviewerID := range result.AssignedReviewers {
						reviewer, err := userRepo.FindByID(ctx, reviewerID)
						if err != nil {
							t.Errorf("Failed to get reviewer %s: %v", reviewerID, err)
							continue
						}
						if reviewer.TeamID != author.TeamID {
							t.Errorf("Reviewer %s is from different team: %d vs author team %d",
								reviewerID, reviewer.TeamID, author.TeamID)
						}
						if !reviewer.IsActive {
							t.Errorf("Reviewer %s should be active", reviewerID)
						}
						if reviewer.ID == tt.authorID {
							t.Error("Author should not be assigned as reviewer")
						}
					}
				}
			} else {

				if result != nil {
					t.Error("Expected nil result when error occurs")
				}
			}
		})
	}
}

func TestPRUseCase_GetPR(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		prID          string
		setupData     func()
		expectedError error
		description   string
	}{
		{
			name: "successfully get existing PR",
			prID: "pr_existing",
			setupData: func() {
				testDB.Exec(`
					INSERT INTO pull_requests (id, title, author_id, status) 
					VALUES ('pr_existing', 'Test PR', 'user_1', 'OPEN')
				`)
				testDB.Exec(`
					INSERT INTO pr_reviewers (pr_id, reviewer_id) 
					VALUES ('pr_existing', 'user_3'), ('pr_existing', 'user_4')
				`)
			},
			expectedError: nil,
			description:   "should return PR with reviewers",
		},
		{
			name: "get non-existent PR",
			prID: "non_existent_pr",
			setupData: func() {

			},
			expectedError: domain.ErrPRNotFound,
			description:   "should return error for non-existent PR",
		},
		{
			name: "get PR without reviewers",
			prID: "pr_no_reviewers",
			setupData: func() {
				testDB.Exec(`
					INSERT INTO pull_requests (id, title, author_id, status) 
					VALUES ('pr_no_reviewers', 'PR without reviewers', 'user_1', 'OPEN')
				`)
			},
			expectedError: nil,
			description:   "should return PR with empty reviewers list",
		},
		{
			name: "get merged PR",
			prID: "pr_merged",
			setupData: func() {
				testDB.Exec(`
					INSERT INTO pull_requests (id, title, author_id, status, merged_at) 
					VALUES ('pr_merged', 'Merged PR', 'user_1', 'MERGED', NOW())
				`)
			},
			expectedError: nil,
			description:   "should return merged PR with merged_at time",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupTestData(t)
			tt.setupData()

			result, err := prUseCase.GetPR(ctx, tt.prID)

			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("Expected error %v, got nil", tt.expectedError)
				} else if err != tt.expectedError {
					t.Errorf("Expected error %v, got %v", tt.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					return
				}
			}

			if tt.expectedError == nil {
				if result == nil {
					t.Error("Expected PR object, got nil")
					return
				}

				if result.ID != tt.prID {
					t.Errorf("PR ID mismatch: got %s, want %s", result.ID, tt.prID)
				}

				if tt.prID == "pr_existing" && len(result.AssignedReviewers) != 2 {
					t.Errorf("Expected 2 reviewers, got %d", len(result.AssignedReviewers))
				}

				if tt.prID == "pr_no_reviewers" && len(result.AssignedReviewers) != 0 {
					t.Errorf("Expected 0 reviewers, got %d", len(result.AssignedReviewers))
				}

				if tt.prID == "pr_merged" && result.Status != domain.PRStatusMerged {
					t.Errorf("Expected status MERGED, got %s", result.Status)
				}
			} else {

				if result != nil {
					t.Error("Expected nil result when error occurs")
				}
			}
		})
	}
}

func TestPRUseCase_MergePR(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		prID          string
		setupData     func()
		expectedError error
		description   string
	}{
		{
			name: "successfully merge open PR",
			prID: "pr_to_merge",
			setupData: func() {
				testDB.Exec(`
					INSERT INTO pull_requests (id, title, author_id, status) 
					VALUES ('pr_to_merge', 'PR to merge', 'user_1', 'OPEN')
				`)
			},
			expectedError: nil,
			description:   "should merge open PR and set merged_at time",
		},
		{
			name: "merge already merged PR (idempotence)",
			prID: "pr_already_merged",
			setupData: func() {
				mergedTime := time.Now().Add(-1 * time.Hour)
				testDB.Exec(`
					INSERT INTO pull_requests (id, title, author_id, status, merged_at) 
					VALUES ('pr_already_merged', 'Already merged PR', 'user_1', 'MERGED', $1)
				`, mergedTime)
			},
			expectedError: nil,
			description:   "should return merged PR without error (idempotent)",
		},
		{
			name: "merge non-existent PR",
			prID: "non_existent_pr",
			setupData: func() {

			},
			expectedError: domain.ErrPRNotFound,
			description:   "should fail when PR does not exist",
		},
		{
			name: "merge closed PR",
			prID: "pr_closed",
			setupData: func() {
				testDB.Exec(`
					INSERT INTO pull_requests (id, title, author_id, status) 
					VALUES ('pr_closed', 'Closed PR', 'user_1', 'CLOSED')
				`)
			},
			expectedError: nil,
			description:   "should change status from CLOSED to MERGED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupTestData(t)
			tt.setupData()

			result, err := prUseCase.MergePR(ctx, tt.prID)

			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("Expected error %v, got nil", tt.expectedError)
				} else if err != tt.expectedError {
					t.Errorf("Expected error %v, got %v", tt.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			if tt.expectedError == nil {
				if result == nil {
					t.Error("Expected PR object, got nil")
					return
				}

				if result.Status != domain.PRStatusMerged {
					t.Errorf("PR status should be MERGED, got %s", result.Status)
				}

				if result.MergedAt == nil {
					t.Error("PR merged_at should be set")
				}

				dbPR, err := prRepo.FindByID(ctx, tt.prID)
				if err != nil {
					t.Errorf("Failed to verify PR in DB: %v", err)
					return
				}

				if dbPR.Status != domain.PRStatusMerged {
					t.Errorf("PR in DB has status %s, expected MERGED", dbPR.Status)
				}

				if dbPR.MergedAt == nil {
					t.Error("PR in DB should have merged_at set")
				}

				if tt.prID == "pr_already_merged" {
					originalPR, _ := prRepo.FindByID(ctx, tt.prID)
					if originalPR.MergedAt.Sub(*result.MergedAt).Abs() > time.Second {
						t.Error("Merged time should not change for already merged PR")
					}
				}
			} else {

				if result != nil {
					t.Error("Expected nil result when error occurs")
				}
			}
		})
	}
}

func TestPRUseCase_ReassignReviewer(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name              string
		prID              string
		oldReviewerID     string
		setupData         func()
		expectedError     error
		expectNewReviewer bool
		description       string
	}{
		{
			name:          "successfully reassign reviewer",
			prID:          "pr_reassign",
			oldReviewerID: "user_3",
			setupData: func() {

				testDB.Exec(`
					INSERT INTO pull_requests (id, title, author_id, status) 
					VALUES ('pr_reassign', 'PR for reassign', 'user_1', 'OPEN')
				`)
				testDB.Exec(`
					INSERT INTO pr_reviewers (pr_id, reviewer_id) 
					VALUES ('pr_reassign', 'user_3'), ('pr_reassign', 'user_4')
				`)

				testDB.Exec(`
					INSERT INTO users (id, username, team_id, is_active) 
					VALUES ('extra_user_1', 'extra1', 2, true),
						   ('extra_user_2', 'extra2', 2, true)
				`)
			},
			expectedError:     nil,
			expectNewReviewer: true,
			description:       "should replace reviewer with new one from same team",
		},
		{
			name:          "reassign reviewer from merged PR",
			prID:          "pr_merged_reassign",
			oldReviewerID: "user_3",
			setupData: func() {
				testDB.Exec(`
					INSERT INTO pull_requests (id, title, author_id, status, merged_at) 
					VALUES ('pr_merged_reassign', 'Merged PR', 'user_1', 'MERGED', NOW())
				`)
				testDB.Exec(`
					INSERT INTO pr_reviewers (pr_id, reviewer_id) 
					VALUES ('pr_merged_reassign', 'user_3')
				`)
			},
			expectedError:     domain.ErrPRMerged,
			expectNewReviewer: false,
			description:       "should fail when PR is already merged",
		},
		{
			name:          "reassign non-assigned reviewer",
			prID:          "pr_wrong_reviewer",
			oldReviewerID: "user_1",
			setupData: func() {
				testDB.Exec(`
					INSERT INTO pull_requests (id, title, author_id, status) 
					VALUES ('pr_wrong_reviewer', 'PR with reviewers', 'user_1', 'OPEN')
				`)
				testDB.Exec(`
					INSERT INTO pr_reviewers (pr_id, reviewer_id) 
					VALUES ('pr_wrong_reviewer', 'user_3')
				`)
			},
			expectedError:     domain.ErrReviewerNotAssigned,
			expectNewReviewer: false,
			description:       "should fail when reviewer is not assigned to PR",
		},
		{
			name:          "reassign when no candidates available",
			prID:          "pr_no_candidates",
			oldReviewerID: "user_3",
			setupData: func() {
				testDB.Exec(`
					INSERT INTO pull_requests (id, title, author_id, status) 
					VALUES ('pr_no_candidates', 'PR no candidates', 'user_1', 'OPEN')
				`)
				testDB.Exec(`
					INSERT INTO pr_reviewers (pr_id, reviewer_id) 
					VALUES ('pr_no_candidates', 'user_3'), ('pr_no_candidates', 'user_4')
				`)

				testDB.Exec("UPDATE users SET is_active = false WHERE team_id = 2 AND id NOT IN ('user_3', 'user_4')")
			},
			expectedError:     domain.ErrNoCandidates,
			expectNewReviewer: false,
			description:       "should fail when no active reviewers available for replacement",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupTestData(t)
			tt.setupData()

			newReviewerID, err := prUseCase.ReassignReviewer(ctx, tt.prID, tt.oldReviewerID)

			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("Expected error %v, got nil", tt.expectedError)
				} else if err != tt.expectedError {
					t.Errorf("Expected error %v, got %v", tt.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			if tt.expectedError == nil {
				if tt.expectNewReviewer {
					if newReviewerID == "" {
						t.Error("Expected new reviewer ID, got empty")
					}
					if newReviewerID == tt.oldReviewerID {
						t.Error("New reviewer should be different from old reviewer")
					}

					dbPR, err := prRepo.FindByID(ctx, tt.prID)
					if err != nil {
						t.Errorf("Failed to verify PR in DB: %v", err)
						return
					}

					foundOld := false
					for _, reviewer := range dbPR.AssignedReviewers {
						if reviewer == tt.oldReviewerID {
							foundOld = true
							break
						}
					}
					if foundOld {
						t.Error("Old reviewer should be removed from PR")
					}

					foundNew := false
					for _, reviewer := range dbPR.AssignedReviewers {
						if reviewer == newReviewerID {
							foundNew = true
							break
						}
					}
					if !foundNew {
						t.Error("New reviewer should be assigned to PR")
					}

					oldReviewer, _ := userRepo.FindByID(ctx, tt.oldReviewerID)
					newReviewer, err := userRepo.FindByID(ctx, newReviewerID)
					if err != nil {
						t.Errorf("Failed to get new reviewer: %v", err)
						return
					}
					if newReviewer.TeamID != oldReviewer.TeamID {
						t.Errorf("New reviewer should be from same team: got %d, want %d",
							newReviewer.TeamID, oldReviewer.TeamID)
					}
				}
			} else {

				if newReviewerID != "" {
					t.Error("Expected empty reviewer ID when error occurs")
				}
			}
		})
	}
}

func TestPRUseCase_GetPRsByReviewer(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name            string
		reviewerID      string
		setupData       func()
		expectedPRCount int
		expectedError   error
		description     string
	}{
		{
			name:       "get PRs for reviewer with multiple assignments",
			reviewerID: "user_3",
			setupData: func() {

				testDB.Exec(`
					INSERT INTO pull_requests (id, title, author_id, status) 
					VALUES 
						('pr_reviewer_1', 'PR 1', 'user_1', 'OPEN'),
						('pr_reviewer_2', 'PR 2', 'user_2', 'OPEN'),
						('pr_reviewer_3', 'PR 3', 'user_1', 'MERGED')
				`)
				testDB.Exec(`
					INSERT INTO pr_reviewers (pr_id, reviewer_id) 
					VALUES 
						('pr_reviewer_1', 'user_3'),
						('pr_reviewer_2', 'user_3'),
						('pr_reviewer_3', 'user_3'),
						('pr_reviewer_1', 'user_4') 
				`)
			},
			expectedPRCount: 3,
			expectedError:   nil,
			description:     "should return all PRs assigned to reviewer",
		},
		{
			name:       "get PRs for reviewer without assignments",
			reviewerID: "user_1",
			setupData: func() {

				testDB.Exec(`
					INSERT INTO pull_requests (id, title, author_id, status) 
					VALUES ('pr_no_review', 'PR no review', 'user_2', 'OPEN')
				`)
			},
			expectedPRCount: 0,
			expectedError:   nil,
			description:     "should return empty list for reviewer without assignments",
		},
		{
			name:       "get PRs for non-existent reviewer",
			reviewerID: "non_existent_reviewer",
			setupData: func() {

			},
			expectedPRCount: 0,
			expectedError:   nil,
			description:     "should return empty list for non-existent reviewer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupTestData(t)
			tt.setupData()

			results, err := prUseCase.GetPRsByReviewer(ctx, tt.reviewerID)

			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("Expected error %v, got nil", tt.expectedError)
				} else if err != tt.expectedError {
					t.Errorf("Expected error %v, got %v", tt.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			if len(results) != tt.expectedPRCount {
				t.Errorf("Expected %d PRs, got %d", tt.expectedPRCount, len(results))
			}

			for _, pr := range results {
				found := false
				for _, reviewer := range pr.AssignedReviewers {
					if reviewer == tt.reviewerID {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("PR %s should have reviewer %s", pr.ID, tt.reviewerID)
				}
			}
		})
	}
}

func TestPRUseCase_Integration_CreateMergeAndReassign(t *testing.T) {
	ctx := context.Background()

	t.Run("complete PR lifecycle: create, merge, and reassign", func(t *testing.T) {
		setupTestData(t)

		prID := "pr_lifecycle_1"
		title := "Complete lifecycle PR"
		authorID := "user_1"

		createdPR, err := prUseCase.CreatePR(ctx, prID, title, authorID)
		if err != nil {
			t.Fatalf("Failed to create PR: %v", err)
		}

		if createdPR.Status != domain.PRStatusOpen {
			t.Errorf("New PR should be OPEN, got %s", createdPR.Status)
		}
		if len(createdPR.AssignedReviewers) == 0 {
			t.Error("New PR should have assigned reviewers")
		}

		retrievedPR, err := prUseCase.GetPR(ctx, prID)
		if err != nil {
			t.Fatalf("Failed to get PR: %v", err)
		}

		if retrievedPR.Title != title {
			t.Errorf("Retrieved PR title mismatch: got %s, want %s", retrievedPR.Title, title)
		}

		mergedPR, err := prUseCase.MergePR(ctx, prID)
		if err != nil {
			t.Fatalf("Failed to merge PR: %v", err)
		}

		if mergedPR.Status != domain.PRStatusMerged {
			t.Errorf("Merged PR should be MERGED, got %s", mergedPR.Status)
		}
		if mergedPR.MergedAt == nil {
			t.Error("Merged PR should have merged_at set")
		}

		if len(createdPR.AssignedReviewers) > 0 {
			oldReviewerID := createdPR.AssignedReviewers[0]
			_, err := prUseCase.ReassignReviewer(ctx, prID, oldReviewerID)
			if err != domain.ErrPRMerged {
				t.Errorf("Expected ErrPRMerged when reassigning on merged PR, got: %v", err)
			}
		}

		if len(createdPR.AssignedReviewers) > 0 {
			reviewerID := createdPR.AssignedReviewers[0]
			reviewerPRs, err := prUseCase.GetPRsByReviewer(ctx, reviewerID)
			if err != nil {
				t.Fatalf("Failed to get PRs by reviewer: %v", err)
			}

			found := false
			for _, pr := range reviewerPRs {
				if pr.ID == prID {
					found = true
					break
				}
			}
			if !found {
				t.Error("PR should be found by reviewer")
			}
		}
	})
}

func TestPRUseCase_EdgeCases(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		operation   func() error
		expectErr   bool
		description string
	}{
		{
			name: "create PR with empty ID",
			operation: func() error {
				_, err := prUseCase.CreatePR(ctx, "", "Test PR", "user_1")
				return err
			},
			expectErr:   true,
			description: "should handle empty PR ID",
		},
		{
			name: "create PR with empty title",
			operation: func() error {
				_, err := prUseCase.CreatePR(ctx, "pr_empty_title", "", "user_1")
				return err
			},
			expectErr:   true,
			description: "should handle empty PR title",
		},
		{
			name: "get PR with empty ID",
			operation: func() error {
				_, err := prUseCase.GetPR(ctx, "")
				return err
			},
			expectErr:   true,
			description: "should handle empty PR ID in get",
		},
		{
			name: "merge PR with empty ID",
			operation: func() error {
				_, err := prUseCase.MergePR(ctx, "")
				return err
			},
			expectErr:   true,
			description: "should handle empty PR ID in merge",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupTestData(t)

			err := tt.operation()

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error for %s, got nil", tt.description)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for %s: %v", tt.description, err)
				}
			}
		})
	}
}
