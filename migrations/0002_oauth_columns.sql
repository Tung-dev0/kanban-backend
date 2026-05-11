ALTER TABLE users ADD COLUMN IF NOT EXISTS email        TEXT UNIQUE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS google_id    TEXT UNIQUE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS display_name TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url   TEXT;

CREATE INDEX IF NOT EXISTS idx_users_email     ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_google_id ON users(google_id);
