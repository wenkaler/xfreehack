ALTER TABLE chats ADD COLUMN IF NOT EXISTS timezone_offset INTEGER DEFAULT 3;
ALTER TABLE chats ADD COLUMN IF NOT EXISTS notification_hour INTEGER DEFAULT 18;
-- Drop old column if needed, or keep for backward compat until data migrated
-- ALTER TABLE chats DROP COLUMN IF EXISTS notification_schedule;
