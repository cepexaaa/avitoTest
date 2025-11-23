package pullrequest

import (
	"avito-test-task/internal/domain"
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:15-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_DB":       "test_review_service",
			"POSTGRES_USER":     "test_user",
			"POSTGRES_PASSWORD": "test_password",
		},
		WaitingFor: wait.ForAll(
			wait.ForLog("database system is ready to accept connections"),
			wait.ForListeningPort("5432/tcp"),
		).WithStartupTimeout(30 * time.Second),
	}

	postgresContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		log.Fatalf("Failed to start container: %s", err)
	}
	defer postgresContainer.Terminate(ctx)

	host, err := postgresContainer.Host(ctx)
	if err != nil {
		log.Fatalf("Failed to get host: %s", err)
	}

	port, err := postgresContainer.MappedPort(ctx, "5432")
	if err != nil {
		log.Fatalf("Failed to get port: %s", err)
	}

	connStr := fmt.Sprintf("host=%s port=%s user=test_user password=test_password dbname=test_review_service sslmode=disable",
		host, port.Port())

	var db *sql.DB
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		db, err = sql.Open("postgres", connStr)
		if err != nil {
			log.Printf("Failed to open database (attempt %d): %s", i+1, err)
			time.Sleep(2 * time.Second)
			continue
		}

		err = db.Ping()
		if err != nil {
			log.Printf("Failed to ping database (attempt %d): %s", i+1, err)
			db.Close()
			time.Sleep(2 * time.Second)
			continue
		}
		break
	}

	if err != nil {
		log.Fatalf("Failed to connect to database after %d attempts: %s", maxRetries, err)
	}

	testDB = db

	if err := setupTestDB(testDB); err != nil {
		log.Fatalf("Failed to setup test database: %s", err)
	}

	code := m.Run()

	testDB.Close()
	os.Exit(code)
}

func setupTestDB(db *sql.DB) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS teams (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) UNIQUE NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			id VARCHAR(255) PRIMARY KEY,
			username VARCHAR(255) NOT NULL,
			team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
			is_active BOOLEAN DEFAULT TRUE
		)`,
		`CREATE TABLE IF NOT EXISTS pull_requests (
			id VARCHAR(255) PRIMARY KEY,
			title VARCHAR(500) NOT NULL,
			author_id VARCHAR(255) NOT NULL REFERENCES users(id),
			status VARCHAR(50) NOT NULL DEFAULT 'OPEN',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			merged_at TIMESTAMP WITH TIME ZONE NULL
		)`,
		`CREATE TABLE IF NOT EXISTS pr_reviewers (
			pr_id VARCHAR(255) NOT NULL REFERENCES pull_requests(id) ON DELETE CASCADE,
			reviewer_id VARCHAR(255) NOT NULL REFERENCES users(id),
			PRIMARY KEY(pr_id, reviewer_id)
		)`,

		`INSERT INTO teams (name) VALUES 
			('backend-team'),
			('frontend-team')
		ON CONFLICT (name) DO NOTHING`,
		`INSERT INTO users (id, username, team_id, is_active) VALUES 
			('user_1', 'alice', 1, true),
			('user_2', 'bob', 1, true),
			('user_3', 'charlie', 2, true),
			('user_4', 'dave', 2, true)
		ON CONFLICT (id) DO NOTHING`,
		`INSERT INTO pull_requests (id, title, author_id, status, created_at, merged_at) VALUES 
			('pr_1', 'Add authentication', 'user_1', 'OPEN', '2024-01-01 10:00:00', NULL),
			('pr_2', 'Fix login bug', 'user_2', 'MERGED', '2024-01-02 11:00:00', '2024-01-03 12:00:00'),
			('pr_3', 'Update UI', 'user_3', 'OPEN', '2024-01-03 13:00:00', NULL),
			('pr_4', 'Refactor API', 'user_1', 'MERGED', '2024-01-04 14:00:00', NULL)
		ON CONFLICT (id) DO NOTHING`,
		`INSERT INTO pr_reviewers (pr_id, reviewer_id) VALUES 
			('pr_1', 'user_2'),
			('pr_1', 'user_3'),
			('pr_2', 'user_1'),
			('pr_3', 'user_1'),
			('pr_3', 'user_4')
		ON CONFLICT (pr_id, reviewer_id) DO NOTHING`,
	}

	for _, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}
	return nil
}

func cleanupTestDB(db *sql.DB) error {
	_, err := db.Exec(`
		TRUNCATE TABLE 
			pr_reviewers,
			pull_requests,
			users,
			teams 
		RESTART IDENTITY CASCADE
	`)
	return err
}

func cleanAndSetup(t *testing.T) {
	t.Helper()
	if err := cleanupTestDB(testDB); err != nil {
		t.Fatalf("Failed to cleanup DB: %v", err)
	}
	if err := setupTestDB(testDB); err != nil {
		t.Fatalf("Failed to setup test data: %v", err)
	}
}

func TestPRRepository_SavePR(t *testing.T) {
	repo := NewPRRepository(testDB)
	ctx := context.Background()

	someDate := time.Date(2024, 1, 5, 10, 0, 0, 0, time.UTC)
	now := time.Now()
	tests := []struct {
		name        string
		pr          *domain.PullRequest
		wantErr     bool
		description string
	}{
		{
			name: "successful save new PR",
			pr: &domain.PullRequest{
				ID:                "pr_new_1",
				Title:             "New feature implementation",
				AuthorID:          "user_1",
				Status:            domain.PRStatusOpen,
				CreatedAt:         &someDate,
				AssignedReviewers: []string{"user_2", "user_3"},
			},
			wantErr:     false,
			description: "should save new PR with reviewers successfully",
		},
		{
			name: "successful update existing PR",
			pr: &domain.PullRequest{
				ID:                "pr_1",
				Title:             "Updated authentication feature",
				AuthorID:          "user_1",
				Status:            domain.PRStatusMerged,
				CreatedAt:         &someDate,
				MergedAt:          &[]time.Time{time.Date(2024, 1, 6, 12, 0, 0, 0, time.UTC)}[0],
				AssignedReviewers: []string{"user_4"},
			},
			wantErr:     false,
			description: "should update existing PR and replace reviewers",
		},
		{
			name: "save PR with non-existent author",
			pr: &domain.PullRequest{
				ID:        "pr_invalid_1",
				Title:     "Invalid PR",
				AuthorID:  "non_existent_user",
				Status:    domain.PRStatusOpen,
				CreatedAt: &now,
			},
			wantErr:     true,
			description: "should fail with foreign key violation for author",
		},
		{
			name: "save PR with non-existent reviewers",
			pr: &domain.PullRequest{
				ID:                "pr_invalid_2",
				Title:             "Invalid reviewers PR",
				AuthorID:          "user_1",
				Status:            domain.PRStatusOpen,
				CreatedAt:         &now,
				AssignedReviewers: []string{"non_existent_reviewer"},
			},
			wantErr:     true,
			description: "should fail with foreign key violation for reviewers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanAndSetup(t)

			err := repo.SavePR(ctx, tt.pr)

			if (err != nil) != tt.wantErr {
				t.Errorf("SavePR() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {

				savedPR, err := repo.FindByID(ctx, tt.pr.ID)
				if err != nil {
					t.Errorf("Failed to verify PR save: %v", err)
					return
				}

				if savedPR.Title != tt.pr.Title {
					t.Errorf("PR title mismatch: got %s, want %s", savedPR.Title, tt.pr.Title)
				}
				if savedPR.Status != tt.pr.Status {
					t.Errorf("PR status mismatch: got %s, want %s", savedPR.Status, tt.pr.Status)
				}
				if savedPR.AuthorID != tt.pr.AuthorID {
					t.Errorf("PR author ID mismatch: got %s, want %s", savedPR.AuthorID, tt.pr.AuthorID)
				}

				if len(savedPR.AssignedReviewers) < len(tt.pr.AssignedReviewers) {
					t.Errorf("Reviewers count mismatch: got %d, want %d",
						len(savedPR.AssignedReviewers), len(tt.pr.AssignedReviewers))
				}

				reviewerMap := make(map[string]bool)
				for _, reviewer := range savedPR.AssignedReviewers {
					reviewerMap[reviewer] = true
				}
				for _, expectedReviewer := range tt.pr.AssignedReviewers {
					if !reviewerMap[expectedReviewer] {
						t.Errorf("Missing reviewer: %s", expectedReviewer)
					}
				}
			}
		})
	}
}

func TestPRRepository_FindByID(t *testing.T) {
	repo := NewPRRepository(testDB)
	ctx := context.Background()

	tests := []struct {
		name    string
		prID    string
		wantPR  *domain.PullRequest
		wantErr bool
	}{
		{
			name: "successful find PR with multiple reviewers",
			prID: "pr_1",
			wantPR: &domain.PullRequest{
				ID:                "pr_1",
				Title:             "Add authentication",
				AuthorID:          "user_1",
				Status:            domain.PRStatusOpen,
				AssignedReviewers: []string{"user_2", "user_3"},
			},
			wantErr: false,
		},
		{
			name: "successful find merged PR",
			prID: "pr_2",
			wantPR: &domain.PullRequest{
				ID:                "pr_2",
				Title:             "Fix login bug",
				AuthorID:          "user_2",
				Status:            domain.PRStatusMerged,
				AssignedReviewers: []string{"user_1"},
			},
			wantErr: false,
		},
		{
			name:    "PR not found",
			prID:    "non_existent_pr",
			wantPR:  nil,
			wantErr: true,
		},
		{
			name: "find PR without reviewers",
			prID: "pr_4",
			wantPR: &domain.PullRequest{
				ID:                "pr_4",
				Title:             "Refactor API",
				AuthorID:          "user_1",
				Status:            domain.PRStatusMerged,
				AssignedReviewers: []string{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanAndSetup(t)

			gotPR, err := repo.FindByID(ctx, tt.prID)

			if (err != nil) != tt.wantErr {
				t.Errorf("FindByID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if gotPR != nil {
					t.Errorf("FindByID() should return nil PR on error")
				}
				return
			}

			if gotPR == nil {
				t.Error("FindByID() returned nil PR without error")
				return
			}

			if gotPR.ID != tt.wantPR.ID {
				t.Errorf("FindByID() ID = %s, want %s", gotPR.ID, tt.wantPR.ID)
			}
			if gotPR.Title != tt.wantPR.Title {
				t.Errorf("FindByID() Title = %s, want %s", gotPR.Title, tt.wantPR.Title)
			}
			if gotPR.AuthorID != tt.wantPR.AuthorID {
				t.Errorf("FindByID() AuthorID = %s, want %s", gotPR.AuthorID, tt.wantPR.AuthorID)
			}
			if gotPR.Status != tt.wantPR.Status {
				t.Errorf("FindByID() Status = %s, want %s", gotPR.Status, tt.wantPR.Status)
			}

			if len(gotPR.AssignedReviewers) != len(tt.wantPR.AssignedReviewers) {
				t.Errorf("FindByID() reviewers count = %d, want %d",
					len(gotPR.AssignedReviewers), len(tt.wantPR.AssignedReviewers))
				return
			}

			reviewerMap := make(map[string]bool)
			for _, reviewer := range gotPR.AssignedReviewers {
				reviewerMap[reviewer] = true
			}
			for _, wantReviewer := range tt.wantPR.AssignedReviewers {
				if !reviewerMap[wantReviewer] {
					t.Errorf("FindByID() missing reviewer: %s", wantReviewer)
				}
			}
		})
	}
}

func TestPRRepository_UpdateStatus(t *testing.T) {
	repo := NewPRRepository(testDB)
	ctx := context.Background()

	tests := []struct {
		name     string
		prID     string
		status   domain.PRStatus
		mergedAt *time.Time
		wantErr  bool
	}{
		{
			name:     "successful update status to merged",
			prID:     "pr_1",
			status:   domain.PRStatusMerged,
			mergedAt: &[]time.Time{time.Date(2024, 1, 6, 14, 0, 0, 0, time.UTC)}[0],
			wantErr:  false,
		},
		{
			name:     "successful update status to closed",
			prID:     "pr_3",
			status:   domain.PRStatusMerged,
			mergedAt: nil,
			wantErr:  false,
		},
		{
			name:     "update status with nil mergedAt for merged PR",
			prID:     "pr_1",
			status:   domain.PRStatusMerged,
			mergedAt: nil,
			wantErr:  false,
		},
		{
			name:     "update non-existent PR",
			prID:     "non_existent_pr",
			status:   domain.PRStatusMerged,
			mergedAt: &[]time.Time{time.Now()}[0],
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanAndSetup(t)

			err := repo.UpdateStatus(ctx, tt.prID, tt.status, tt.mergedAt)

			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {

				updatedPR, err := repo.FindByID(ctx, tt.prID)
				if err != nil {
					t.Errorf("Failed to verify status update: %v", err)
					return
				}

				if updatedPR.Status != tt.status {
					t.Errorf("UpdateStatus() status = %s, want %s", updatedPR.Status, tt.status)
				}

				if tt.mergedAt != nil {
					if updatedPR.MergedAt == nil || !updatedPR.MergedAt.Equal(*tt.mergedAt) {
						t.Errorf("UpdateStatus() merged_at mismatch")
					}
				}
			}
		})
	}
}

func TestPRRepository_ReplaceReviewer(t *testing.T) {
	repo := NewPRRepository(testDB)
	ctx := context.Background()

	tests := []struct {
		name          string
		prID          string
		oldReviewerID string
		newReviewerID string
		wantErr       bool
	}{
		{
			name:          "successful replace reviewer",
			prID:          "pr_1",
			oldReviewerID: "user_2",
			newReviewerID: "user_4",
			wantErr:       false,
		},
		{
			name:          "replace non-assigned reviewer",
			prID:          "pr_1",
			oldReviewerID: "user_1",
			newReviewerID: "user_4",
			wantErr:       true,
		},
		{
			name:          "replace on non-existent PR",
			prID:          "non_existent_pr",
			oldReviewerID: "user_1",
			newReviewerID: "user_2",
			wantErr:       true,
		},
		{
			name:          "replace with non-existent new reviewer",
			prID:          "pr_1",
			oldReviewerID: "user_2",
			newReviewerID: "non_existent_user",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanAndSetup(t)

			originalPR, err := repo.FindByID(ctx, tt.prID)
			if err != nil && !tt.wantErr {
				t.Fatalf("Failed to get original PR: %v", err)
			}

			err = repo.ReplaceReviewer(ctx, tt.prID, tt.oldReviewerID, tt.newReviewerID)

			if (err != nil) != tt.wantErr {
				t.Errorf("ReplaceReviewer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {

				updatedPR, err := repo.FindByID(ctx, tt.prID)
				if err != nil {
					t.Errorf("Failed to get updated PR: %v", err)
					return
				}

				foundOld := false
				for _, reviewer := range updatedPR.AssignedReviewers {
					if reviewer == tt.oldReviewerID {
						foundOld = true
						break
					}
				}
				if foundOld {
					t.Error("Old reviewer should be removed after replacement")
				}

				foundNew := false
				for _, reviewer := range updatedPR.AssignedReviewers {
					if reviewer == tt.newReviewerID {
						foundNew = true
						break
					}
				}
				if !foundNew {
					t.Error("New reviewer should be assigned after replacement")
				}

				if len(updatedPR.AssignedReviewers) != len(originalPR.AssignedReviewers) {
					t.Errorf("Reviewers count changed: got %d, want %d",
						len(updatedPR.AssignedReviewers), len(originalPR.AssignedReviewers))
				}
			}
		})
	}
}

func TestPRRepository_FindByReviewerID(t *testing.T) {
	repo := NewPRRepository(testDB)
	ctx := context.Background()

	tests := []struct {
		name        string
		reviewerID  string
		wantPRCount int
		wantPRIDs   []string
		wantErr     bool
	}{
		{
			name:        "find PRs for reviewer with multiple assignments",
			reviewerID:  "user_1",
			wantPRCount: 2,
			wantPRIDs:   []string{"pr_2", "pr_3"},
			wantErr:     false,
		},
		{
			name:        "find PRs for reviewer with one assignment",
			reviewerID:  "user_2",
			wantPRCount: 1,
			wantPRIDs:   []string{"pr_1"},
			wantErr:     false,
		},
		{
			name:        "find PRs for reviewer without assignments",
			reviewerID:  "user_4",
			wantPRCount: 1,
			wantPRIDs:   []string{"pr_3"},
			wantErr:     false,
		},
		{
			name:        "find PRs for non-existent reviewer",
			reviewerID:  "non_existent_user",
			wantPRCount: 0,
			wantPRIDs:   []string{},
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanAndSetup(t)

			prs, err := repo.FindByReviewerID(ctx, tt.reviewerID)

			if (err != nil) != tt.wantErr {
				t.Errorf("FindByReviewerID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			if len(prs) != tt.wantPRCount {
				t.Errorf("FindByReviewerID() count = %d, want %d", len(prs), tt.wantPRCount)
				return
			}

			for _, pr := range prs {
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

			if len(tt.wantPRIDs) > 0 {
				prIDMap := make(map[string]bool)
				for _, pr := range prs {
					prIDMap[pr.ID] = true
				}
				for _, wantPRID := range tt.wantPRIDs {
					if !prIDMap[wantPRID] {
						t.Errorf("Missing expected PR: %s", wantPRID)
					}
				}
			}
		})
	}
}

func TestPRRepository_Integration_CompleteWorkflow(t *testing.T) {
	repo := NewPRRepository(testDB)
	ctx := context.Background()
	now := time.Now()

	t.Run("complete PR lifecycle", func(t *testing.T) {
		cleanAndSetup(t)

		newPR := &domain.PullRequest{
			ID:                "pr_workflow_1",
			Title:             "New workflow feature",
			AuthorID:          "user_1",
			Status:            domain.PRStatusOpen,
			CreatedAt:         &now,
			AssignedReviewers: []string{"user_2", "user_3"},
		}

		err := repo.SavePR(ctx, newPR)
		if err != nil {
			t.Fatalf("Failed to create PR: %v", err)
		}

		createdPR, err := repo.FindByID(ctx, "pr_workflow_1")
		if err != nil {
			t.Fatalf("Failed to find created PR: %v", err)
		}
		if createdPR.Status != domain.PRStatusOpen {
			t.Errorf("New PR should have OPEN status, got: %s", createdPR.Status)
		}
		if len(createdPR.AssignedReviewers) != 2 {
			t.Errorf("New PR should have 2 reviewers, got: %d", len(createdPR.AssignedReviewers))
		}

		err = repo.ReplaceReviewer(ctx, "pr_workflow_1", "user_2", "user_4")
		if err != nil {
			t.Fatalf("Failed to replace reviewer: %v", err)
		}

		updatedPR, err := repo.FindByID(ctx, "pr_workflow_1")
		if err != nil {
			t.Fatalf("Failed to find updated PR: %v", err)
		}
		if len(updatedPR.AssignedReviewers) != 2 {
			t.Errorf("PR should still have 2 reviewers after replacement, got: %d", len(updatedPR.AssignedReviewers))
		}

		hasUser3 := false
		hasUser4 := false
		for _, reviewer := range updatedPR.AssignedReviewers {
			if reviewer == "user_3" {
				hasUser3 = true
			}
			if reviewer == "user_4" {
				hasUser4 = true
			}
			if reviewer == "user_2" {
				t.Error("Old reviewer user_2 should be removed")
			}
		}
		if !hasUser3 || !hasUser4 {
			t.Error("PR should have user_3 and user_4 as reviewers")
		}

		mergedAt := time.Now().UTC()
		err = repo.UpdateStatus(ctx, "pr_workflow_1", domain.PRStatusMerged, &mergedAt)
		if err != nil {
			t.Fatalf("Failed to update PR status: %v", err)
		}

		finalPR, err := repo.FindByID(ctx, "pr_workflow_1")
		if err != nil {
			t.Fatalf("Failed to find final PR: %v", err)
		}
		if finalPR.Status != domain.PRStatusMerged {
			t.Errorf("Final PR status should be MERGED, got: %s", finalPR.Status)
		}

		user4PRs, err := repo.FindByReviewerID(ctx, "user_4")
		if err != nil {
			t.Fatalf("Failed to find PRs by reviewer: %v", err)
		}
		found := false
		for _, pr := range user4PRs {
			if pr.ID == "pr_workflow_1" {
				found = true
				break
			}
		}
		if !found {
			t.Error("PR should be found by reviewer user_4")
		}
	})
}

func TestPRRepository_EdgeCases(t *testing.T) {
	repo := NewPRRepository(testDB)
	ctx := context.Background()
	now := time.Now()

	t.Run("test edge cases", func(t *testing.T) {
		cleanAndSetup(t)

		emptyPR := &domain.PullRequest{
			ID:        "",
			Title:     "Test",
			AuthorID:  "user_1",
			Status:    domain.PRStatusOpen,
			CreatedAt: &now,
		}
		err := repo.SavePR(ctx, emptyPR)
		if err == nil {
			t.Error("SavePR with empty ID should fail")
		}

		longTitle := string(make([]byte, 600))
		longPR := &domain.PullRequest{
			ID:        "pr_long",
			Title:     longTitle,
			AuthorID:  "user_1",
			Status:    domain.PRStatusOpen,
			CreatedAt: &now,
		}
		err = repo.SavePR(ctx, longPR)
		if err == nil {
			t.Error("SavePR with very long title should fail")
		}

		invalidStatusPR := &domain.PullRequest{
			ID:        "pr_invalid_status",
			Title:     "Test",
			AuthorID:  "user_1",
			Status:    domain.PRStatus("INVALID_STATUS"),
			CreatedAt: &now,
		}
		err = repo.SavePR(ctx, invalidStatusPR)
		if err == nil {
			t.Error("SavePR with invalid status should fail")
		}
	})
}
