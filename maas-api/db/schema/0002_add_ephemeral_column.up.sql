-- Schema for API Key Management: 0002_add_ephemeral_column.up.sql
-- Description: Add ephemeral column for short-lived programmatic tokens (e.g., GenAI Playground)

-- Add ephemeral column to api_keys table
ALTER TABLE api_keys ADD COLUMN ephemeral BOOLEAN NOT NULL DEFAULT FALSE;

-- Index for cleanup job: find expired ephemeral keys efficiently
-- Partial index only includes ephemeral keys to minimize index size
CREATE INDEX idx_api_keys_ephemeral_expired 
ON api_keys(ephemeral, status, expires_at) 
WHERE ephemeral = TRUE;
