ALTER TABLE identities ADD COLUMN user_id TEXT NULL REFERENCES users(id);
