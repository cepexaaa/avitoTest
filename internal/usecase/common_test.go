package usecase

// DB_HOST=localhost DB_PORT=5433 DB_USER=postgres DB_PASSWORD=password go test -v ./internal/usecase/...

import (
	pullrequest "avito-test-task/internal/repository/pull_request"
	"avito-test-task/internal/repository/team"
	"avito-test-task/internal/repository/user"
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var testDB *sql.DB
var userRepo *user.UserRepository
var userUseCase *UserUseCase
var teamRepo *team.TeamRepository
var teamUseCase *TeamUseCase
var prRepo *pullrequest.PRRepository
var prUseCase PRUseCase

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

	teamRepo = team.NewTeamRepository(testDB)
	userRepo = user.NewUserRepository(testDB)
	userUseCase = NewUserUseCase(*userRepo)
	teamUseCase = NewTeamUseCase(*teamRepo, *userRepo)
	prRepo = pullrequest.NewPRRepository(testDB)
	prUseCase = *NewPRUseCase(*prRepo, *userRepo, *teamRepo)
	code := m.Run()

	testDB.Close()
	os.Exit(code)
}

func setupTestDB(db *sql.DB) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS teams (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) UNIQUE NOT NULL CHECK (name <> '')
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			id VARCHAR(255) PRIMARY KEY,
			username VARCHAR(255) NOT NULL CHECK (username <> ''),
			team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
			is_active BOOLEAN DEFAULT TRUE
		)`,
		`CREATE TABLE IF NOT EXISTS pull_requests (
			id VARCHAR(255) PRIMARY KEY,
			title VARCHAR(500) NOT NULL CHECK (title <> ''),
			author_id VARCHAR(255) NOT NULL REFERENCES users(id),
			status VARCHAR(50) NOT NULL DEFAULT 'OPEN',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			merged_at TIMESTAMP WITH TIME ZONE NULL
		);
		CREATE INDEX IF NOT EXISTS idx_pr_author_id ON pull_requests(author_id);
		CREATE INDEX IF NOT EXISTS idx_pr_status ON pull_requests(status);`,
		`CREATE TABLE IF NOT EXISTS pr_reviewers (
			pr_id VARCHAR(255) NOT NULL REFERENCES pull_requests(id) ON DELETE CASCADE,
			reviewer_id VARCHAR(255) NOT NULL REFERENCES users(id),
			PRIMARY KEY(pr_id, reviewer_id)
		);
		CREATE INDEX IF NOT EXISTS idx_pr_reviewers_pr_id ON pr_reviewers(pr_id);
		CREATE INDEX IF NOT EXISTS idx_pr_reviewers_reviewer_id ON pr_reviewers(reviewer_id);`,
		// Test data
		`INSERT INTO teams (name) VALUES 
			('backend-team'),
			('frontend-team')
		ON CONFLICT (name) DO NOTHING`,
		`INSERT INTO users (id, username, team_id, is_active) VALUES 
			('user_1', 'alice', 1, true),
			('user_2', 'bob', 1, false),
			('user_3', 'charlie', 2, true),
			('user_4', 'dave', 2, true),
			('user_5', 'tom', 1, true)
		ON CONFLICT (id) DO NOTHING`,
	}

	for _, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}
	return nil
}

func setupTestData(t *testing.T) {
	t.Helper()
	if err := cleanupTestDB(testDB); err != nil {
		t.Fatalf("Failed to cleanup DB: %v", err)
	}
	if err := setupTestDB(testDB); err != nil {
		t.Fatalf("Failed to setup test data: %v", err)
	}
}

func cleanupTestDB(db *sql.DB) error {
	_, err := db.Exec(`
		TRUNCATE TABLE 
			users,
			teams,
			pull_requests,
			pr_reviewers
		RESTART IDENTITY CASCADE
	`)
	return err
}
