package team

import (
	"context"
	"database/sql"

	"avito-test-task/internal/domain"

	"github.com/lib/pq"
)

type TeamRepository struct {
	db *sql.DB
}

func NewTeamRepository(db *sql.DB) *TeamRepository {
	return &TeamRepository{db: db}
}

func (r *TeamRepository) SaveTeam(ctx context.Context, team *domain.Team) error {
	query := `INSERT INTO teams (name) VALUES ($1) RETURNING id`

	err := r.db.QueryRowContext(ctx, query, team.Name).Scan(&team.ID)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrTeamExists
		}
		return err
	}

	return nil
}

func (r *TeamRepository) FindByName(ctx context.Context, name string) (*domain.Team, error) {
	query := `SELECT id, name FROM teams WHERE name = $1`

	var team domain.Team
	err := r.db.QueryRowContext(ctx, query, name).Scan(&team.ID, &team.Name)

	if err == sql.ErrNoRows {
		return nil, domain.ErrTeamNotFound
	}

	return &team, err
}

func (r *TeamRepository) FindByID(ctx context.Context, id int) (*domain.Team, error) {
	query := `SELECT id, name FROM teams WHERE id = $1`

	var team domain.Team
	err := r.db.QueryRowContext(ctx, query, id).Scan(&team.ID, &team.Name)

	if err == sql.ErrNoRows {
		return nil, domain.ErrTeamNotFound
	}

	return &team, err
}

func isUniqueViolation(err error) bool {
	if err, ok := err.(*pq.Error); ok {
		return err.Code == "23505"
	}
	return false
}
