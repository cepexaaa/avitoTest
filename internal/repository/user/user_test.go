package user

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

// Run test: DB_HOST=localhost DB_PORT=5433 DB_USER=postgres DB_PASSWORD=password go test -v ./internal/repository/user/...

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
		`INSERT INTO teams (name) VALUES 
			('backend-team'),
			('frontend-team')
		ON CONFLICT (name) DO NOTHING`,
	}

	for _, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			return err
		}
	}
	return nil
}

func TestUserRepository_SaveUser(t *testing.T) {
	repo := NewUserRepository(testDB)
	ctx := context.Background()

	tests := []struct {
		name    string
		user    *domain.User
		wantErr bool
	}{
		{
			name: "successful save new user",
			user: &domain.User{
				ID:       "user_1",
				Username: "alice",
				TeamID:   1,
				IsActive: true,
			},
			wantErr: false,
		},
		{
			name: "successful update existing user",
			user: &domain.User{
				ID:       "user_1",
				Username: "alice_updated",
				TeamID:   2,
				IsActive: false,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.SaveUser(ctx, tt.user)
			if (err != nil) != tt.wantErr {
				t.Errorf("SaveUser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				var username string
				var teamID int
				var isActive bool
				err := testDB.QueryRow("SELECT username, team_id, is_active FROM users WHERE id = $1", tt.user.ID).
					Scan(&username, &teamID, &isActive)
				if err != nil {
					t.Errorf("Failed to verify user save: %v", err)
					return
				}

				if username != tt.user.Username || teamID != tt.user.TeamID || isActive != tt.user.IsActive {
					t.Errorf("User data mismatch: got (%s, %d, %t), want (%s, %d, %t)",
						username, teamID, isActive, tt.user.Username, tt.user.TeamID, tt.user.IsActive)
				}
			}
		})
	}
}

func TestUserRepository_FindByID(t *testing.T) {
	repo := NewUserRepository(testDB)
	ctx := context.Background()

	testUser := &domain.User{
		ID:       "find_user_1",
		Username: "bob",
		TeamID:   1,
		IsActive: true,
	}

	err := repo.SaveUser(ctx, testUser)
	if err != nil {
		t.Fatalf("Failed to setup test user: %v", err)
	}

	tests := []struct {
		name     string
		userID   string
		wantUser *domain.User
		wantErr  bool
	}{
		{
			name:   "successful find existing user",
			userID: "find_user_1",
			wantUser: &domain.User{
				ID:       "find_user_1",
				Username: "bob",
				TeamID:   1,
				IsActive: true,
				TeamName: "backend-team",
			},
			wantErr: false,
		},
		{
			name:     "user not found",
			userID:   "non_existent_user",
			wantUser: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotUser, err := repo.FindByID(ctx, tt.userID)

			if (err != nil) != tt.wantErr {
				t.Errorf("FindByID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if gotUser != nil {
					t.Errorf("FindByID() should return nil user on error")
				}
				return
			}

			if gotUser.ID != tt.wantUser.ID ||
				gotUser.Username != tt.wantUser.Username ||
				gotUser.TeamID != tt.wantUser.TeamID ||
				gotUser.IsActive != tt.wantUser.IsActive ||
				gotUser.TeamName != tt.wantUser.TeamName {
				t.Errorf("FindByID() got = %+v, want %+v", gotUser, tt.wantUser)
			}
		})
	}
}

func TestUserRepository_FindActiveByTeamID(t *testing.T) {
	cleanupTestDB(testDB)
	repo := NewUserRepository(testDB)
	ctx := context.Background()

	users := []*domain.User{
		{ID: "active_1", Username: "active_user_1", TeamID: 1, IsActive: true},
		{ID: "active_2", Username: "active_user_2", TeamID: 1, IsActive: true},
		{ID: "inactive_1", Username: "inactive_user_1", TeamID: 1, IsActive: false},
		{ID: "active_3", Username: "active_user_3", TeamID: 2, IsActive: true},
	}

	for _, u := range users {
		err := repo.SaveUser(ctx, u)
		if err != nil {
			t.Fatalf("Failed to setup test users: %v", err)
		}
	}

	t.Run("find active users excluding one", func(t *testing.T) {
		users, err := repo.FindActiveByTeamID(ctx, 1, "active_1")
		if err != nil {
			t.Fatalf("FindActiveByTeamID() error = %v", err)
		}

		if len(users) != 1 {
			t.Errorf("Expected 1 active user, got %d", len(users))
			t.Errorf("%s, %s", users[0].ID, users[1].ID)
		}

		if len(users) > 0 && users[0].ID != "active_2" {
			t.Errorf("Expected user ID 'active_2', got '%s'", users[0].ID)
		}
	})
}

func TestUserRepository_UpdateActivity(t *testing.T) {
	repo := NewUserRepository(testDB)
	ctx := context.Background()

	testUser := &domain.User{
		ID:       "activity_user",
		Username: "activity_test",
		TeamID:   1,
		IsActive: true,
	}

	err := repo.SaveUser(ctx, testUser)
	if err != nil {
		t.Fatalf("Failed to setup test user: %v", err)
	}

	tests := []struct {
		name    string
		userID  string
		active  bool
		wantErr bool
	}{
		{
			name:    "deactivate user",
			userID:  "activity_user",
			active:  false,
			wantErr: false,
		},
		{
			name:    "activate user",
			userID:  "activity_user",
			active:  true,
			wantErr: false,
		},
		{
			name:    "update non-existent user",
			userID:  "non_existent",
			active:  true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.UpdateActivity(ctx, tt.userID, tt.active)

			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateActivity() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				var isActive bool
				err := testDB.QueryRow("SELECT is_active FROM users WHERE id = $1", tt.userID).
					Scan(&isActive)
				if err != nil {
					t.Errorf("Failed to verify activity update: %v", err)
					return
				}

				if isActive != tt.active {
					t.Errorf("UpdateActivity() is_active = %t, want %t", isActive, tt.active)
				}
			}
		})
	}
}

func cleanupTestDB(db *sql.DB) error {
	_, err := db.Exec(`
        TRUNCATE TABLE 
            users
        RESTART IDENTITY CASCADE
    `)
	return err
}
