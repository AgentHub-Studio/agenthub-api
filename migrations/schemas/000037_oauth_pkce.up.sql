-- Migration to add PKCE support (code_verifier) to oauth_credential table
ALTER TABLE oauth_credential ADD COLUMN IF NOT EXISTS code_verifier TEXT;
