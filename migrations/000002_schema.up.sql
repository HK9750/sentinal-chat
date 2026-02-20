-- Users & Auth
CREATE TABLE IF NOT EXISTS users (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  phone_number CITEXT UNIQUE,
  username CITEXT UNIQUE,
  email CITEXT UNIQUE,
  password_hash TEXT NOT NULL,
  display_name TEXT NOT NULL,
  role user_role DEFAULT 'USER',
  bio TEXT,
  avatar_url TEXT,
  is_online BOOLEAN DEFAULT FALSE,
  last_seen_at TIMESTAMP,
  is_active BOOLEAN DEFAULT TRUE,
  is_verified BOOLEAN DEFAULT FALSE,
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_settings (
  user_id UUID PRIMARY KEY REFERENCES users(id),
  privacy_last_seen privacy_setting DEFAULT 'EVERYONE',
  privacy_profile_photo privacy_setting DEFAULT 'EVERYONE',
  privacy_about privacy_setting DEFAULT 'EVERYONE',
  privacy_groups privacy_setting DEFAULT 'EVERYONE',
  read_receipts BOOLEAN DEFAULT TRUE,
  notifications_enabled BOOLEAN DEFAULT TRUE,
  notification_sound TEXT DEFAULT 'default',
  notification_vibrate BOOLEAN DEFAULT TRUE,
  theme theme_mode DEFAULT 'SYSTEM',
  language language_code DEFAULT 'en',
  enter_to_send BOOLEAN DEFAULT TRUE,
  media_auto_download_wifi BOOLEAN DEFAULT TRUE,
  media_auto_download_mobile BOOLEAN DEFAULT FALSE,
  updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS devices (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  device_id TEXT NOT NULL,
  device_name TEXT,
  device_type TEXT,
  is_active BOOLEAN DEFAULT TRUE,
  registered_at TIMESTAMP DEFAULT NOW(),
  last_seen_at TIMESTAMP,
  UNIQUE (user_id, device_id)
);

CREATE TABLE IF NOT EXISTS push_tokens (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  platform TEXT NOT NULL,
  token TEXT NOT NULL,
  is_active BOOLEAN DEFAULT TRUE,
  created_at TIMESTAMP DEFAULT NOW(),
  last_used_at TIMESTAMP,
  UNIQUE (device_id, token)
);

CREATE TABLE IF NOT EXISTS user_sessions (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  device_id UUID REFERENCES devices(id) ON DELETE SET NULL,
  refresh_token_hash TEXT NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  is_revoked BOOLEAN DEFAULT FALSE,
  created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_contacts (
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  contact_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  nickname TEXT,
  is_blocked BOOLEAN DEFAULT FALSE,
  created_at TIMESTAMP DEFAULT NOW(),
  PRIMARY KEY (user_id, contact_user_id)
);

-- Conversations & Participants
CREATE TABLE IF NOT EXISTS conversations (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  type conversation_type NOT NULL,
  subject TEXT,
  description TEXT,
  avatar_url TEXT,
  expiry_seconds INTEGER,
  disappearing_mode disappearing_mode DEFAULT 'OFF',
  message_expiry_seconds INTEGER,
  group_permissions JSONB,
  invite_link TEXT,
  invite_link_revoked_at TIMESTAMP,
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS participants (
  conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role participant_role DEFAULT 'MEMBER',
  joined_at TIMESTAMP DEFAULT NOW(),
  added_by UUID REFERENCES users(id),
  muted_until TIMESTAMP,
  pinned_at TIMESTAMP,
  archived BOOLEAN DEFAULT FALSE,
  last_read_sequence BIGINT DEFAULT 0,
  permissions JSONB,
  PRIMARY KEY (conversation_id, user_id)
);

CREATE TABLE IF NOT EXISTS conversation_sequences (
  conversation_id UUID PRIMARY KEY REFERENCES conversations(id) ON DELETE CASCADE,
  last_sequence BIGINT DEFAULT 0,
  updated_at TIMESTAMP DEFAULT NOW()
);

-- Messages & Content
CREATE TABLE IF NOT EXISTS messages (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  sender_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  client_message_id TEXT,
  idempotency_key TEXT,
  seq_id BIGINT,
  type message_type DEFAULT 'TEXT',
  metadata JSONB,
  is_forwarded BOOLEAN DEFAULT FALSE,
  forwarded_from_msg_id UUID REFERENCES messages(id),
  reply_to_msg_id UUID REFERENCES messages(id),
  poll_id UUID,
  link_preview_id UUID,
  mention_count INTEGER DEFAULT 0,
  created_at TIMESTAMP DEFAULT NOW(),
  edited_at TIMESTAMP,
  deleted_at TIMESTAMP,
  expires_at TIMESTAMP,
  UNIQUE (conversation_id, client_message_id)
);

CREATE TABLE IF NOT EXISTS message_ciphertexts (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
  recipient_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  recipient_device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  sender_device_id UUID REFERENCES devices(id) ON DELETE SET NULL,
  ciphertext BYTEA NOT NULL,
  header JSONB NOT NULL,
  created_at TIMESTAMP DEFAULT NOW(),
  UNIQUE (message_id, recipient_device_id)
);

CREATE TABLE IF NOT EXISTS message_reactions (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  reaction_code VARCHAR NOT NULL,
  created_at TIMESTAMP DEFAULT NOW(),
  UNIQUE (message_id, user_id, reaction_code)
);

CREATE TABLE IF NOT EXISTS message_receipts (
  message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  status delivery_status DEFAULT 'PENDING',
  delivered_at TIMESTAMP,
  read_at TIMESTAMP,
  played_at TIMESTAMP,
  updated_at TIMESTAMP DEFAULT NOW(),
  PRIMARY KEY (message_id, user_id)
);

CREATE TABLE IF NOT EXISTS message_mentions (
  message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  "offset" INTEGER NOT NULL,
  length INTEGER NOT NULL,
  PRIMARY KEY (message_id, user_id, "offset")
);

CREATE TABLE IF NOT EXISTS starred_messages (
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
  starred_at TIMESTAMP DEFAULT NOW(),
  PRIMARY KEY (user_id, message_id)
);

CREATE TABLE IF NOT EXISTS link_previews (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  url TEXT NOT NULL,
  url_hash TEXT NOT NULL,
  title TEXT,
  description TEXT,
  image_url TEXT,
  site_name TEXT,
  fetched_at TIMESTAMP DEFAULT NOW(),
  UNIQUE (url_hash)
);

-- Attachments
CREATE TABLE IF NOT EXISTS attachments (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  uploader_id UUID REFERENCES users(id) ON DELETE SET NULL,
  url TEXT NOT NULL,
  filename TEXT,
  mime_type TEXT NOT NULL,
  size_bytes BIGINT NOT NULL,
  view_once BOOLEAN DEFAULT FALSE,
  viewed_at TIMESTAMP,
  thumbnail_url TEXT,
  width INTEGER,
  height INTEGER,
  duration_seconds INTEGER,
  encryption_key_hash TEXT,
  encryption_iv TEXT,
  created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS message_attachments (
  message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
  attachment_id UUID NOT NULL REFERENCES attachments(id) ON DELETE CASCADE,
  PRIMARY KEY (message_id, attachment_id)
);

-- Broadcast Lists
CREATE TABLE IF NOT EXISTS broadcast_lists (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  description TEXT,
  created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS broadcast_recipients (
  broadcast_id UUID NOT NULL REFERENCES broadcast_lists(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  added_at TIMESTAMP DEFAULT NOW(),
  PRIMARY KEY (broadcast_id, user_id)
);

-- Polls
CREATE TABLE IF NOT EXISTS polls (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  message_id UUID REFERENCES messages(id) ON DELETE CASCADE,
  question TEXT NOT NULL,
  allows_multiple BOOLEAN DEFAULT FALSE,
  closes_at TIMESTAMP,
  created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS poll_options (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  poll_id UUID NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
  option_text TEXT NOT NULL,
  position INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS poll_votes (
  poll_id UUID NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
  option_id UUID NOT NULL REFERENCES poll_options(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  voted_at TIMESTAMP DEFAULT NOW(),
  PRIMARY KEY (poll_id, option_id, user_id)
);

-- Chat Labels
CREATE TABLE IF NOT EXISTS chat_labels (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  color TEXT,
  position INTEGER,
  created_at TIMESTAMP DEFAULT NOW(),
  UNIQUE (user_id, name)
);

CREATE TABLE IF NOT EXISTS conversation_labels (
  conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  label_id UUID NOT NULL REFERENCES chat_labels(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at TIMESTAMP DEFAULT NOW(),
  PRIMARY KEY (conversation_id, label_id, user_id)
);

-- Calls & WebRTC
CREATE TABLE IF NOT EXISTS calls (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  initiated_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  type call_type NOT NULL,
  topology call_topology NOT NULL,
  is_group_call BOOLEAN DEFAULT FALSE,
  started_at TIMESTAMP DEFAULT NOW(),
  connected_at TIMESTAMP,
  ended_at TIMESTAMP,
  end_reason call_end_reason,
  duration_seconds INTEGER,
  created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS call_participants (
  call_id UUID NOT NULL REFERENCES calls(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  status participant_call_status DEFAULT 'INVITED',
  joined_at TIMESTAMP,
  left_at TIMESTAMP,
  muted_audio BOOLEAN DEFAULT FALSE,
  muted_video BOOLEAN DEFAULT FALSE,
  device_type TEXT,
  PRIMARY KEY (call_id, user_id)
);

CREATE TABLE IF NOT EXISTS call_quality_metrics (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  call_id UUID NOT NULL REFERENCES calls(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  recorded_at TIMESTAMP DEFAULT NOW(),
  packets_sent BIGINT,
  packets_received BIGINT,
  packets_lost BIGINT,
  jitter_ms DECIMAL,
  round_trip_time_ms DECIMAL,
  bitrate_kbps INTEGER,
  frame_rate INTEGER,
  resolution_width INTEGER,
  resolution_height INTEGER,
  audio_level DECIMAL,
  connection_type TEXT,
  ice_candidate_type TEXT
);

-- E2E Encryption
CREATE TABLE IF NOT EXISTS identity_keys (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  public_key BYTEA NOT NULL,
  is_active BOOLEAN DEFAULT TRUE,
  created_at TIMESTAMP DEFAULT NOW(),
  UNIQUE (user_id, device_id)
);

CREATE TABLE IF NOT EXISTS signed_prekeys (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  key_id INTEGER NOT NULL,
  public_key BYTEA NOT NULL,
  signature BYTEA NOT NULL,
  created_at TIMESTAMP DEFAULT NOW(),
  is_active BOOLEAN DEFAULT TRUE,
  UNIQUE (device_id, key_id)
);

CREATE TABLE IF NOT EXISTS onetime_prekeys (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  key_id INTEGER NOT NULL,
  public_key BYTEA NOT NULL,
  uploaded_at TIMESTAMP DEFAULT NOW(),
  consumed_at TIMESTAMP,
  consumed_by UUID REFERENCES users(id),
  consumed_by_device_id UUID REFERENCES devices(id),
  UNIQUE (device_id, key_id)
);

CREATE TABLE IF NOT EXISTS outbox_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_type VARCHAR(50) NOT NULL,
    aggregate_type VARCHAR(50) NOT NULL,
    aggregate_id VARCHAR(36) NOT NULL,
    payload JSONB NOT NULL,
    status outbox_status DEFAULT 'PENDING',
    retry_count INT DEFAULT 0,
    error TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMP
);

-- Additional Tables
CREATE TABLE IF NOT EXISTS message_user_states (
  message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  is_deleted BOOLEAN DEFAULT FALSE,
  deleted_at TIMESTAMP,
  is_starred BOOLEAN DEFAULT FALSE,
  is_pinned BOOLEAN DEFAULT FALSE,
  PRIMARY KEY (message_id, user_id)
);

CREATE TABLE IF NOT EXISTS conversation_clears (
  conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  cleared_at TIMESTAMP DEFAULT NOW(),
  PRIMARY KEY (conversation_id, user_id)
);

CREATE TABLE IF NOT EXISTS upload_sessions (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  uploader_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  filename TEXT NOT NULL,
  mime_type TEXT NOT NULL,
  size_bytes BIGINT NOT NULL,
  chunk_size INTEGER NOT NULL,
  uploaded_bytes BIGINT DEFAULT 0,
  status upload_status DEFAULT 'IN_PROGRESS',
  object_key TEXT,
  file_url TEXT,
  completed_at TIMESTAMP,
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW()
);

-- Command Log table for audit trail and undo
CREATE TABLE IF NOT EXISTS command_logs (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  command_type VARCHAR(50) NOT NULL,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  status command_status NULL DEFAULT 'PENDING',
  payload JSONB NOT NULL,
  result JSONB,
  undo_data JSONB,
  error_message TEXT,
  execution_time_ms INT,
  created_at TIMESTAMP DEFAULT NOW(),
  executed_at TIMESTAMP,
  undone_at TIMESTAMP
);

-- Scheduled messages for delayed delivery
CREATE TABLE IF NOT EXISTS scheduled_messages (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
  conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  sender_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  content TEXT NOT NULL,
  scheduled_for TIMESTAMP NOT NULL,
  timezone VARCHAR(50) DEFAULT 'UTC',
  status scheduled_messages_status DEFAULT 'PENDING',
  created_at TIMESTAMP DEFAULT NOW(),
  sent_at TIMESTAMP
);

-- Message versions for edit history
CREATE TABLE IF NOT EXISTS message_versions (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
  content TEXT NOT NULL,
  edited_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  edited_at TIMESTAMP DEFAULT NOW(),
  version_number INT NOT NULL
);
