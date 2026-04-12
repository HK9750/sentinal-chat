DROP INDEX IF EXISTS idx_notifications_user_unread;
DROP INDEX IF EXISTS idx_notifications_user_created;
DROP INDEX IF EXISTS idx_notifications_user_dedupe;

DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS user_notification_settings;
