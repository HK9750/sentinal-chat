DROP FUNCTION IF EXISTS fn_mark_view_once();
DROP FUNCTION IF EXISTS fn_update_timestamp();
DROP FUNCTION IF EXISTS fn_assign_message_sequence();

DROP TYPE IF EXISTS upload_status;
DROP TYPE IF EXISTS command_status;
DROP TYPE IF EXISTS participant_call_status;
DROP TYPE IF EXISTS call_end_reason;
DROP TYPE IF EXISTS call_topology;
DROP TYPE IF EXISTS call_type;
DROP TYPE IF EXISTS disappearing_mode;
DROP TYPE IF EXISTS language_code;
DROP TYPE IF EXISTS theme_mode;
DROP TYPE IF EXISTS privacy_setting;
DROP TYPE IF EXISTS delivery_status;
DROP TYPE IF EXISTS message_type;
DROP TYPE IF EXISTS participant_role;
DROP TYPE IF EXISTS conversation_type;

DROP EXTENSION IF EXISTS "citext";
DROP EXTENSION IF EXISTS "pgcrypto";
DROP EXTENSION IF EXISTS "uuid-ossp";
