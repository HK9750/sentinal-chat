package services

import (
	"time"

	"github.com/google/uuid"
)

type MessageKind string

const (
	MessageKindText   MessageKind = "TEXT"
	MessageKindAudio  MessageKind = "AUDIO"
	MessageKindFile   MessageKind = "FILE"
	MessageKindPoll   MessageKind = "POLL"
	MessageKindSystem MessageKind = "SYSTEM"
)

type ActionKind string

const (
	ActionEdit           ActionKind = "EDIT"
	ActionDelete         ActionKind = "DELETE"
	ActionReactionAdd    ActionKind = "REACTION_ADD"
	ActionReactionRemove ActionKind = "REACTION_REMOVE"
	ActionPin            ActionKind = "PIN"
	ActionUnpin          ActionKind = "UNPIN"
)

type ConversationView struct {
	ID               string              `json:"id"`
	Type             string              `json:"type"`
	Subject          *string             `json:"subject,omitempty"`
	Description      *string             `json:"description,omitempty"`
	AvatarURL        *string             `json:"avatar_url,omitempty"`
	DisappearingMode string              `json:"disappearing_mode"`
	CreatedBy        *string             `json:"created_by,omitempty"`
	CreatedAt        time.Time           `json:"created_at"`
	UpdatedAt        time.Time           `json:"updated_at"`
	LastMessageAt    *time.Time          `json:"last_message_at,omitempty"`
	Participants     []ParticipantView   `json:"participants"`
	LastMessage      *MessageSummaryView `json:"last_message,omitempty"`
	UnreadCount      int64               `json:"unread_count"`
	LastReadSequence int64               `json:"last_read_sequence"`
}

type ParticipantView struct {
	UserID      string     `json:"user_id"`
	DisplayName string     `json:"display_name"`
	Username    string     `json:"username"`
	AvatarURL   string     `json:"avatar_url"`
	Role        string     `json:"role"`
	JoinedAt    time.Time  `json:"joined_at"`
	MutedUntil  *time.Time `json:"muted_until,omitempty"`
	Archived    bool       `json:"archived"`
	IsOnline    bool       `json:"is_online"`
	LastReadSeq int64      `json:"last_read_sequence"`
}

type MessageSummaryView struct {
	ID                 string     `json:"id"`
	SenderID           string     `json:"sender_id"`
	Kind               string     `json:"kind"`
	Content            *string    `json:"content,omitempty"`
	IsForwarded        bool       `json:"is_forwarded"`
	CreatedAt          time.Time  `json:"created_at"`
	SeqID              int64      `json:"seq_id"`
	ReceiptStatus      *string    `json:"receipt_status,omitempty"`
	DeletedAt          *time.Time `json:"deleted_at,omitempty"`
	AttachmentMimeType *string    `json:"attachment_mime_type,omitempty"`
	AttachmentFilename *string    `json:"attachment_filename,omitempty"`
	DurationSeconds    *int32     `json:"duration_seconds,omitempty"`
}

type AttachmentView struct {
	ID              string     `json:"id"`
	FileURL         string     `json:"file_url"`
	Filename        *string    `json:"filename,omitempty"`
	MimeType        string     `json:"mime_type"`
	SizeBytes       int64      `json:"size_bytes"`
	ThumbnailURL    *string    `json:"thumbnail_url,omitempty"`
	Width           *int32     `json:"width,omitempty"`
	Height          *int32     `json:"height,omitempty"`
	DurationSeconds *int32     `json:"duration_seconds,omitempty"`
	ViewOnce        bool       `json:"view_once"`
	ViewedAt        *time.Time `json:"viewed_at,omitempty"`
}

type PollOptionView struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Position int    `json:"position"`
	Votes    int    `json:"votes"`
}

type PollView struct {
	ID             string           `json:"id"`
	Question       string           `json:"question"`
	AllowsMultiple bool             `json:"allows_multiple"`
	ClosesAt       *time.Time       `json:"closes_at,omitempty"`
	Closed         bool             `json:"closed"`
	Options        []PollOptionView `json:"options"`
	MyVotes        []string         `json:"my_votes,omitempty"`
}

type ReceiptView struct {
	UserID      string     `json:"user_id"`
	Status      string     `json:"status"`
	DeliveredAt *time.Time `json:"delivered_at,omitempty"`
	ReadAt      *time.Time `json:"read_at,omitempty"`
	PlayedAt    *time.Time `json:"played_at,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type ReactionView struct {
	UserID       string    `json:"user_id"`
	ReactionCode string    `json:"reaction_code"`
	CreatedAt    time.Time `json:"created_at"`
}

type MessageView struct {
	ID              string           `json:"id"`
	ConversationID  string           `json:"conversation_id"`
	SenderID        string           `json:"sender_id"`
	ClientMessageID *string          `json:"client_message_id,omitempty"`
	SeqID           int64            `json:"seq_id"`
	Type            string           `json:"type"`
	Content         *string          `json:"content,omitempty"`
	IsForwarded     bool             `json:"is_forwarded"`
	ReplyToMsgID    *string          `json:"reply_to_msg_id,omitempty"`
	MentionCount    int              `json:"mention_count"`
	CreatedAt       time.Time        `json:"created_at"`
	EditedAt        *time.Time       `json:"edited_at,omitempty"`
	DeletedAt       *time.Time       `json:"deleted_at,omitempty"`
	ExpiresAt       *time.Time       `json:"expires_at,omitempty"`
	Attachments     []AttachmentView `json:"attachments,omitempty"`
	Receipts        []ReceiptView    `json:"receipts,omitempty"`
	Reactions       []ReactionView   `json:"reactions,omitempty"`
	Poll            *PollView        `json:"poll,omitempty"`
	Pinned          bool             `json:"pinned"`
	IsStarred       bool             `json:"is_starred"`
}

type SendMessageInput struct {
	ConversationID  uuid.UUID
	SenderID        uuid.UUID
	ClientMessageID string
	Type            string
	Content         string
	IsForwarded     bool
	ReplyToMsgID    *uuid.UUID
	ExpiresAt       *time.Time
	AttachmentIDs   []uuid.UUID
	MentionUserIDs  []uuid.UUID
	Poll            *CreatePollInput
}

type EditMessageInput struct {
	ConversationID uuid.UUID
	MessageID      uuid.UUID
	EditorID       uuid.UUID
	Content        string
	ExpiresAt      *time.Time
}

type DeleteMessageInput struct {
	ConversationID uuid.UUID
	MessageID      uuid.UUID
	ActorID        uuid.UUID
}

type BulkDeleteMessagesInput struct {
	ConversationID uuid.UUID
	ActorID        uuid.UUID
	MessageIDs     []uuid.UUID
	DeleteMode     string
}

type ReactionInput struct {
	ConversationID uuid.UUID
	MessageID      uuid.UUID
	ActorID        uuid.UUID
	Code           string
}

type PinMessageInput struct {
	ConversationID uuid.UUID
	MessageID      uuid.UUID
	ActorID        uuid.UUID
	Pinned         bool
}

type ReceiptInput struct {
	ConversationID uuid.UUID
	MessageIDs     []uuid.UUID
	ActorID        uuid.UUID
	Status         string
	UpToSeqID      *int64
}

type ReceiptUpdateResult struct {
	MessageIDs []uuid.UUID
	Status     string
	UpToSeqID  *int64
}

type CreateConversationInput struct {
	CreatorID        uuid.UUID
	Type             string
	Subject          string
	Description      string
	AvatarURL        string
	ParticipantIDs   []uuid.UUID
	DisappearingMode string
}

type AddParticipantInput struct {
	ConversationID uuid.UUID
	ActorID        uuid.UUID
	UserID         uuid.UUID
	Role           string
}

type RemoveParticipantInput struct {
	ConversationID uuid.UUID
	ActorID        uuid.UUID
	UserID         uuid.UUID
}

type ClearConversationInput struct {
	ConversationID uuid.UUID
	ActorID        uuid.UUID
}

type UpdateDisappearingModeInput struct {
	ConversationID   uuid.UUID
	ActorID          uuid.UUID
	DisappearingMode string
}

type UpdateConversationMuteInput struct {
	ConversationID uuid.UUID
	ActorID        uuid.UUID
	MutedUntil     *time.Time
}

type DeleteConversationInput struct {
	ConversationID uuid.UUID
	ActorID        uuid.UUID
}

type CreatePollInput struct {
	Question       string
	AllowsMultiple bool
	ClosesAt       *time.Time
	Options        []string
}

type VotePollInput struct {
	ConversationID uuid.UUID
	PollID         uuid.UUID
	ActorID        uuid.UUID
	OptionIDs      []uuid.UUID
}

type CallStartInput struct {
	ConversationID uuid.UUID
	CallerID       uuid.UUID
	Type           string
}

type CallSignalInput struct {
	CallID         uuid.UUID
	ConversationID uuid.UUID
	FromUserID     uuid.UUID
	ToUserID       uuid.UUID
	SignalType     string
	Payload        map[string]any
}

type CallEndInput struct {
	CallID  uuid.UUID
	ActorID uuid.UUID
	Reason  string
}

type CallHistoryItemView struct {
	ID              string     `json:"id"`
	ConversationID  string     `json:"conversation_id"`
	Type            string     `json:"type"`
	InitiatedBy     string     `json:"initiated_by"`
	StartedAt       time.Time  `json:"started_at"`
	ConnectedAt     *time.Time `json:"connected_at,omitempty"`
	EndedAt         *time.Time `json:"ended_at,omitempty"`
	EndReason       *string    `json:"end_reason,omitempty"`
	DurationSeconds *int32     `json:"duration_seconds,omitempty"`
}

type CommandResult struct {
	CommandID      string     `json:"command_id"`
	Type           string     `json:"type"`
	ConversationID *string    `json:"conversation_id,omitempty"`
	Status         string     `json:"status"`
	UndoneAt       *time.Time `json:"undone_at,omitempty"`
	ExecutedAt     *time.Time `json:"executed_at,omitempty"`
}
