-- users
CREATE TABLE IF NOT EXISTS users (
  id BIGSERIAL,
  email VARCHAR(255) NOT NULL,
  password VARCHAR(255),
  email_verified_at TIMESTAMPTZ,
  first_name VARCHAR(255),
  last_name VARCHAR(255),
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  username VARCHAR(255),
  PRIMARY KEY (id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users (email);
-- Unique so usernames cannot collide at the database level (NULLs are exempt in
-- Postgres, so users without a username are unaffected). Complements limen's
-- app-level availability check.
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users (username);

-- sessions
CREATE TABLE IF NOT EXISTS sessions (
  id BIGSERIAL,
  token VARCHAR(255) NOT NULL,
  user_id BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  expires_at TIMESTAMPTZ NOT NULL,
  last_access TIMESTAMPTZ NOT NULL,
  metadata JSONB,
  PRIMARY KEY (id),
  CONSTRAINT fk_sessions_user_id FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE RESTRICT ON UPDATE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_sessions_token ON sessions (token);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions (user_id);

-- accounts
CREATE TABLE IF NOT EXISTS accounts (
  id BIGSERIAL,
  user_id BIGINT NOT NULL,
  provider VARCHAR(255) NOT NULL,
  provider_account_id VARCHAR(255),
  access_token TEXT NOT NULL,
  refresh_token TEXT,
  access_token_expires_at TIMESTAMPTZ,
  scope VARCHAR(255) NOT NULL,
  id_token TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  CONSTRAINT fk_accounts_users_user_id FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE RESTRICT ON UPDATE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_accounts_user_id_provider ON accounts (user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_accounts_provider_provider_account_id ON accounts (provider, provider_account_id);

-- verifications
CREATE TABLE IF NOT EXISTS verifications (
  id BIGSERIAL,
  subject VARCHAR(255) NOT NULL,
  value TEXT NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_verifications_value ON verifications (value);
CREATE INDEX IF NOT EXISTS idx_verifications_subject ON verifications (subject);

-- rate_limits
CREATE TABLE IF NOT EXISTS rate_limits (
  id BIGSERIAL,
  key VARCHAR(255) NOT NULL,
  count INTEGER NOT NULL,
  last_request_at BIGINT NOT NULL,
  PRIMARY KEY (id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_rate_limits_key ON rate_limits (key);
