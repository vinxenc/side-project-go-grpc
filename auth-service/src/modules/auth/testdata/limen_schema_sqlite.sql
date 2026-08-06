-- SQLite-flavored translation of auth-service/migrations/0001_init_limen.up.sql.
-- Used by auth.testsupport_test.go to bootstrap per-test in-memory SQLite DBs.
--
-- Key differences from the Postgres DDL:
--   BIGSERIAL     -> INTEGER PRIMARY KEY AUTOINCREMENT
--   VARCHAR(n)    -> TEXT
--   TIMESTAMPTZ   -> TEXT
--   JSONB         -> TEXT (limen serialises metadata as JSON text)
--   BIGINT        -> INTEGER
--   Named FOREIGN KEY constraint kept inline (SQLite syntax - FK enforcement off by default)

-- users
CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  email TEXT NOT NULL,
  password TEXT,
  email_verified_at TEXT,
  first_name TEXT,
  last_name TEXT,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  username TEXT
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users (email);
CREATE INDEX IF NOT EXISTS idx_users_username ON users (username);

-- sessions
CREATE TABLE IF NOT EXISTS sessions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  token TEXT NOT NULL,
  user_id INTEGER NOT NULL,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  expires_at TEXT NOT NULL,
  last_access TEXT NOT NULL,
  metadata TEXT,
  FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE RESTRICT ON UPDATE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_sessions_token ON sessions (token);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions (user_id);

-- accounts
CREATE TABLE IF NOT EXISTS accounts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  provider TEXT NOT NULL,
  provider_account_id TEXT,
  access_token TEXT NOT NULL,
  refresh_token TEXT,
  access_token_expires_at TEXT,
  scope TEXT NOT NULL,
  id_token TEXT,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE RESTRICT ON UPDATE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_accounts_user_id_provider ON accounts (user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_accounts_provider_provider_account_id ON accounts (provider, provider_account_id);

-- verifications
CREATE TABLE IF NOT EXISTS verifications (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  subject TEXT NOT NULL,
  value TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_verifications_value ON verifications (value);
CREATE INDEX IF NOT EXISTS idx_verifications_subject ON verifications (subject);

-- rate_limits
CREATE TABLE IF NOT EXISTS rate_limits (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  key TEXT NOT NULL,
  count INTEGER NOT NULL,
  last_request_at INTEGER NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_rate_limits_key ON rate_limits (key);
