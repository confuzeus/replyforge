CREATE TABLE IF NOT EXISTS comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    display_id TEXT NOT NULL DEFAULT '',
    post_id TEXT NOT NULL,
    author_name TEXT NOT NULL,
    body TEXT NOT NULL,
    approved INTEGER NOT NULL DEFAULT 0,
    ip_address TEXT NOT NULL,
    user_agent TEXT DEFAULT '',
    turnstile_verified INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_comments_post_approved_created
ON comments(post_id, approved, created_at DESC);
