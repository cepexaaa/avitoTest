package pullrequest

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	"avito-test-task/internal/domain"
)

type PRRepository struct {
	db *sql.DB
}

func NewPRRepository(db *sql.DB) *PRRepository {
	return &PRRepository{db: db}
}

func (r *PRRepository) SavePR(ctx context.Context, pr *domain.PullRequest) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("Error starting transaction: %v", err)
		return err
	}
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	if pr.CreatedAt == nil || pr.CreatedAt.IsZero() {
		now := time.Now()
		pr.CreatedAt = &now
	}

	if pr.ID == "" {
		return errors.New("ID should not be empty")
	}
	if pr.Status != domain.PRStatusMerged && pr.Status != domain.PRStatusOpen {
		return errors.New("Uncorrect status of pull request")
	}

	query := `
        INSERT INTO pull_requests (id, title, author_id, status, created_at, merged_at)
        VALUES ($1, $2, $3, $4, $5, $6)
        ON CONFLICT (id) DO UPDATE SET
            title = EXCLUDED.title,
            status = EXCLUDED.status,
            merged_at = EXCLUDED.merged_at
    `

	log.Printf("Executing PR query: %s", query)
	_, err = tx.ExecContext(ctx, query,
		pr.ID,
		pr.Title,
		pr.AuthorID,
		string(pr.Status),
		pr.CreatedAt,
		pr.MergedAt,
	)
	if err != nil {
		log.Printf("Error saving PR: %v", err)
		return err
	}

	for _, reviewerID := range pr.AssignedReviewers {
		log.Printf("Saving reviewer: %s", reviewerID)
		_, err = tx.ExecContext(ctx,
			"INSERT INTO pr_reviewers (pr_id, reviewer_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
			pr.ID,
			reviewerID,
		)
		if err != nil {
			log.Printf("Error saving reviewer %s: %v", reviewerID, err)
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	committed = true
	return nil
}

func (r *PRRepository) FindByID(ctx context.Context, prID string) (*domain.PullRequest, error) {
	var pr domain.PullRequest

	err := r.db.QueryRowContext(ctx,
		"SELECT id, title, author_id, status, created_at, merged_at FROM pull_requests WHERE id = $1",
		prID,
	).Scan(&pr.ID, &pr.Title, &pr.AuthorID, &pr.Status, &pr.CreatedAt, &pr.MergedAt)

	if err == sql.ErrNoRows {
		return nil, domain.ErrPRNotFound
	}
	if err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx,
		"SELECT reviewer_id FROM pr_reviewers WHERE pr_id = $1",
		prID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var reviewerID string
		if err := rows.Scan(&reviewerID); err != nil {
			return nil, err
		}
		pr.AssignedReviewers = append(pr.AssignedReviewers, reviewerID)
	}

	return &pr, nil
}

func (r *PRRepository) UpdateStatus(ctx context.Context, prID string, status domain.PRStatus, mergedAt *time.Time) error {
	var utcTime time.Time
	if mergedAt != nil {
		utcTime = (*mergedAt).UTC()
	}

	result, err := r.db.ExecContext(ctx,
		"UPDATE pull_requests SET status = $1, merged_at = $2 WHERE id = $3",
		string(status), &utcTime, prID,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return domain.ErrPRNotFound
	}

	return nil
}

func (r *PRRepository) ReplaceReviewer(ctx context.Context, prID, oldReviewerID, newReviewerID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx,
		"DELETE FROM pr_reviewers WHERE pr_id = $1 AND reviewer_id = $2",
		prID, oldReviewerID,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return domain.ErrReviewerNotAssigned
	}

	_, err = tx.ExecContext(ctx,
		"INSERT INTO pr_reviewers (pr_id, reviewer_id) VALUES ($1, $2)",
		prID, newReviewerID,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *PRRepository) FindByReviewerID(ctx context.Context, reviewerID string) ([]*domain.PullRequest, error) {
	query := `
	SELECT pr.id, pr.title, pr.author_id, pr.status, pr.created_at, pr.merged_at
	    FROM pull_requests pr
	    JOIN pr_reviewers rev ON pr.id = rev.pr_id
	    WHERE rev.reviewer_id = $1
	    ORDER BY pr.id
	`

	rows, err := r.db.QueryContext(ctx, query, reviewerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prs []*domain.PullRequest
	for rows.Next() {
		var pr domain.PullRequest
		if err := rows.Scan(
			&pr.ID,
			&pr.Title,
			&pr.AuthorID,
			&pr.Status,
			&pr.CreatedAt,
			&pr.MergedAt,
		); err != nil {
			return nil, err
		}

		reviewerRows, err := r.db.QueryContext(ctx,
			"SELECT reviewer_id FROM pr_reviewers WHERE pr_id = $1",
			pr.ID,
		)
		if err != nil {
			return nil, err
		}

		for reviewerRows.Next() {
			var revID string
			if err := reviewerRows.Scan(&revID); err != nil {
				reviewerRows.Close()
				return nil, err
			}
			pr.AssignedReviewers = append(pr.AssignedReviewers, revID)
		}
		reviewerRows.Close()

		prs = append(prs, &pr)
	}

	return prs, rows.Err()
}
