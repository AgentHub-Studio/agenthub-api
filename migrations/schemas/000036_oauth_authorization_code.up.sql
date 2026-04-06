-- Add fields for OAuth2 Authorization Code flow and interactive connection
ALTER TABLE oauth_credential ADD COLUMN IF NOT EXISTS auth_url TEXT;
ALTER TABLE oauth_credential ADD COLUMN IF NOT EXISTS redirect_url TEXT;
ALTER TABLE oauth_credential ADD COLUMN IF NOT EXISTS refresh_token TEXT;
ALTER TABLE oauth_credential ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;
