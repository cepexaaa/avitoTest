package team

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
			name VARCHAR(255) UNIQUE NOT NULL CHECK (name <> '')
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			id VARCHAR(255) PRIMARY KEY,
			username VARCHAR(255) NOT NULL,
			team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
			is_active BOOLEAN DEFAULT TRUE
		)`,
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
			users,
			teams 
		RESTART IDENTITY CASCADE
	`)
	return err
}

func setupBasicTeams(db *sql.DB) error {
	_, err := db.Exec(`
		INSERT INTO teams (name) VALUES 
			('backend-team'),
			('frontend-team'),
			('mobile-team')
		ON CONFLICT (name) DO NOTHING
	`)
	return err
}

func cleanAndSetup(t *testing.T) {
	t.Helper()
	if err := cleanupTestDB(testDB); err != nil {
		t.Fatalf("Failed to cleanup DB: %v", err)
	}
	if err := setupBasicTeams(testDB); err != nil {
		t.Fatalf("Failed to setup basic teams: %v", err)
	}
}

func TestTeamRepository_SaveTeam(t *testing.T) {
	repo := NewTeamRepository(testDB)
	ctx := context.Background()

	tests := []struct {
		name    string
		team    *domain.Team
		wantErr bool
		errType error
	}{
		{
			name: "successful save new team",
			team: &domain.Team{
				Name: "devops-team",
			},
			wantErr: false,
		},
		{
			name: "save team with duplicate name",
			team: &domain.Team{
				Name: "backend-team",
			},
			wantErr: true,
			errType: domain.ErrTeamExists,
		},
		{
			name: "successful save another new team",
			team: &domain.Team{
				Name: "qa-team",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanAndSetup(t)

			err := repo.SaveTeam(ctx, tt.team)

			if (err != nil) != tt.wantErr {
				t.Errorf("SaveTeam() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {

				if tt.errType != nil && err != tt.errType {
					t.Errorf("SaveTeam() error = %v, wantErr %v", err, tt.errType)
				}
				return
			}

			if tt.team.ID == 0 {
				t.Error("SaveTeam() team ID should be set after save")
				return
			}

			var name string
			err = testDB.QueryRow("SELECT name FROM teams WHERE id = $1", tt.team.ID).
				Scan(&name)
			if err != nil {
				t.Errorf("Failed to verify team save: %v", err)
				return
			}

			if name != tt.team.Name {
				t.Errorf("Team name mismatch: got %s, want %s", name, tt.team.Name)
			}
		})
	}
}

func TestTeamRepository_FindByName(t *testing.T) {
	repo := NewTeamRepository(testDB)
	ctx := context.Background()

	tests := []struct {
		name     string
		teamName string
		wantTeam *domain.Team
		wantErr  bool
	}{
		{
			name:     "successful find existing team",
			teamName: "backend-team",
			wantTeam: &domain.Team{
				ID:   1,
				Name: "backend-team",
			},
			wantErr: false,
		},
		{
			name:     "find another existing team",
			teamName: "frontend-team",
			wantTeam: &domain.Team{
				ID:   2,
				Name: "frontend-team",
			},
			wantErr: false,
		},
		{
			name:     "team not found",
			teamName: "non-existent-team",
			wantTeam: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanAndSetup(t)

			gotTeam, err := repo.FindByName(ctx, tt.teamName)

			if (err != nil) != tt.wantErr {
				t.Errorf("FindByName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if gotTeam != nil {
					t.Errorf("FindByName() should return nil team on error")
				}
				return
			}

			if gotTeam == nil {
				t.Error("FindByName() returned nil team without error")
				return
			}

			if gotTeam.ID == 0 {
				t.Error("FindByName() team ID should not be 0")
			}

			if gotTeam.Name != tt.wantTeam.Name {
				t.Errorf("FindByName() got name = %s, want %s", gotTeam.Name, tt.wantTeam.Name)
			}
		})
	}
}

func TestTeamRepository_FindByID(t *testing.T) {
	repo := NewTeamRepository(testDB)
	ctx := context.Background()

	tests := []struct {
		name     string
		teamID   int
		wantTeam *domain.Team
		wantErr  bool
	}{
		{
			name:   "successful find by ID",
			teamID: 1,
			wantTeam: &domain.Team{
				ID:   1,
				Name: "backend-team",
			},
			wantErr: false,
		},
		{
			name:   "find another team by ID",
			teamID: 2,
			wantTeam: &domain.Team{
				ID:   2,
				Name: "frontend-team",
			},
			wantErr: false,
		},
		{
			name:     "team not found by ID",
			teamID:   999,
			wantTeam: nil,
			wantErr:  true,
		},
		{
			name:     "invalid ID zero",
			teamID:   0,
			wantTeam: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanAndSetup(t)

			gotTeam, err := repo.FindByID(ctx, tt.teamID)

			if (err != nil) != tt.wantErr {
				t.Errorf("FindByID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if gotTeam != nil {
					t.Errorf("FindByID() should return nil team on error")
				}
				return
			}

			if gotTeam == nil {
				t.Error("FindByID() returned nil team without error")
				return
			}

			if gotTeam.ID != tt.wantTeam.ID {
				t.Errorf("FindByID() got ID = %d, want %d", gotTeam.ID, tt.wantTeam.ID)
			}

			if gotTeam.Name != tt.wantTeam.Name {
				t.Errorf("FindByID() got name = %s, want %s", gotTeam.Name, tt.wantTeam.Name)
			}
		})
	}
}

func TestTeamRepository_Integration_SaveAndFind(t *testing.T) {
	repo := NewTeamRepository(testDB)
	ctx := context.Background()

	t.Run("save team and then find it by name and ID", func(t *testing.T) {
		cleanAndSetup(t)

		newTeam := &domain.Team{Name: "integration-team"}
		err := repo.SaveTeam(ctx, newTeam)
		if err != nil {
			t.Fatalf("Failed to save team: %v", err)
		}

		if newTeam.ID == 0 {
			t.Fatal("Team ID should be set after save")
		}

		foundByName, err := repo.FindByName(ctx, "integration-team")
		if err != nil {
			t.Fatalf("Failed to find team by name: %v", err)
		}

		if foundByName.ID != newTeam.ID {
			t.Errorf("FindByName() got ID = %d, want %d", foundByName.ID, newTeam.ID)
		}

		foundByID, err := repo.FindByID(ctx, newTeam.ID)
		if err != nil {
			t.Fatalf("Failed to find team by ID: %v", err)
		}

		if foundByID.Name != newTeam.Name {
			t.Errorf("FindByID() got name = %s, want %s", foundByID.Name, newTeam.Name)
		}

		if foundByName.ID != foundByID.ID || foundByName.Name != foundByID.Name {
			t.Error("Team found by name and by ID should be the same")
		}
	})
}

func TestTeamRepository_ConcurrentOperations(t *testing.T) {
	repo := NewTeamRepository(testDB)
	ctx := context.Background()

	t.Run("handle concurrent team creation", func(t *testing.T) {
		cleanAndSetup(t)

		teamNames := []string{"team-alpha", "team-beta", "team-gamma"}
		errors := make(chan error, len(teamNames))

		for _, name := range teamNames {
			go func(teamName string) {
				team := &domain.Team{Name: teamName}
				err := repo.SaveTeam(ctx, team)
				errors <- err
			}(name)
		}

		var successCount int
		for i := 0; i < len(teamNames); i++ {
			err := <-errors
			if err == nil {
				successCount++
			} else {

				if err != domain.ErrTeamExists {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		}

		if successCount == 0 {
			t.Error("At least one team should be created successfully")
		}

		var count int
		err := testDB.QueryRow("SELECT COUNT(*) FROM teams WHERE name LIKE 'team-%'").Scan(&count)
		if err != nil {
			t.Errorf("Failed to count teams: %v", err)
		}

		if count != successCount {
			t.Errorf("Expected %d teams, got %d", successCount, count)
		}
	})
}

func TestTeamRepository_EmptyAndNullCases(t *testing.T) {
	repo := NewTeamRepository(testDB)
	ctx := context.Background()

	t.Run("handle edge cases", func(t *testing.T) {
		cleanAndSetup(t)

		emptyTeam := &domain.Team{Name: ""}
		err := repo.SaveTeam(ctx, emptyTeam)
		if err == nil {
			t.Error("SaveTeam with empty name should fail")
		}

		found, err := repo.FindByName(ctx, "")
		if err == nil || found != nil {
			t.Error("FindByName with empty string should fail")
		}
	})
}
