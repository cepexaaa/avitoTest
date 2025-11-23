package user

import (
	"avito-test-task/internal/domain"
	"context"
	"database/sql"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (ur *UserRepository) SaveUser(ctx context.Context, user *domain.User) error {
	query := `
	INSERT INTO users (id, username, team_id, is_active)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (id) DO UPDATE SET
            username = EXCLUDED.username,
            team_id = EXCLUDED.team_id,
            is_active = EXCLUDED.is_active
			`

	_, err := ur.db.ExecContext(ctx, query,
		user.ID,
		user.Username,
		user.TeamID,
		user.IsActive)

	return err
}

func (r *UserRepository) FindByID(ctx context.Context, userID string) (*domain.User, error) {
	query := `
        SELECT u.id, u.username, u.team_id, u.is_active, t.name
        FROM users u
        JOIN teams t ON u.team_id = t.id
        WHERE u.id = $1
    `

	var user domain.User
	var teamName string
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&user.ID,
		&user.Username,
		&user.TeamID,
		&user.IsActive,
		&teamName,
	)

	if err == sql.ErrNoRows {
		return nil, domain.ErrUserNotFound
	}

	user.TeamName = teamName
	return &user, err
}

// FindActiveByTeamID ищет активных пользователей команды (исключая автора)
func (r *UserRepository) FindActiveByTeamID(ctx context.Context, teamID int, excludeUserID string) ([]*domain.User, error) {
	query := `
        SELECT id, username, team_id, is_active
        FROM users 
        WHERE team_id = $1 
        AND is_active = true 
        AND id != $2
        ORDER BY id
    `

	rows, err := r.db.QueryContext(ctx, query, teamID, excludeUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		var user domain.User
		if err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.TeamID,
			&user.IsActive,
			// &user.CreatedAt,
			// &user.UpdatedAt,
		); err != nil {
			return nil, err
		}
		users = append(users, &user)
	}

	return users, rows.Err()
}

// UpdateActivity обновляет флаг активности
func (r *UserRepository) UpdateActivity(ctx context.Context, userID string, isActive bool) error {
	query := `UPDATE users SET is_active = $1 WHERE id = $2`
	// , updated_at = $2

	result, err := r.db.ExecContext(ctx, query, isActive, userID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return domain.ErrUserNotFound
	}

	return nil
}

// FindByTeamID возвращает всех пользователей команды
func (r *UserRepository) FindByTeamID(ctx context.Context, teamID int) ([]*domain.User, error) {
	query := `
        SELECT id, username, team_id, is_active
        FROM users 
        WHERE team_id = $1
        ORDER BY id
    `

	rows, err := r.db.QueryContext(ctx, query, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		var user domain.User
		if err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.TeamID,
			&user.IsActive,
		); err != nil {
			return nil, err
		}
		users = append(users, &user)
	}

	return users, rows.Err()
}
