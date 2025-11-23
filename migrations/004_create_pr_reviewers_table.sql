-- +goose Up
CREATE TABLE pr_reviewers (
    pr_id VARCHAR(255) NOT NULL REFERENCES pull_requests(id) ON DELETE CASCADE,
    reviewer_id VARCHAR(255) NOT NULL REFERENCES users(id),
    PRIMARY KEY(pr_id, reviewer_id)
);

CREATE INDEX idx_pr_reviewers_pr_id ON pr_reviewers(pr_id);
CREATE INDEX idx_pr_reviewers_reviewer_id ON pr_reviewers(reviewer_id);

-- +goose Down
DROP TABLE pr_reviewers;