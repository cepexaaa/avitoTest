-- +goose Up
CREATE TABLE pull_requests (
    id VARCHAR(255) PRIMARY KEY,
    title VARCHAR(500) NOT NULL CHECK (title <> ''),
    author_id VARCHAR(255) NOT NULL REFERENCES users(id),
    status VARCHAR(50) NOT NULL DEFAULT 'OPEN',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    merged_at TIMESTAMP WITH TIME ZONE NULL
);

CREATE INDEX idx_pr_author_id ON pull_requests(author_id);
CREATE INDEX idx_pr_status ON pull_requests(status);
