// Package repository defines data access interfaces.
package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"sentinal-chat/internal/domain/call"
	"sentinal-chat/internal/domain/command"
	"sentinal-chat/internal/domain/conversation"
	"sentinal-chat/internal/domain/message"
	"sentinal-chat/internal/domain/outbox"
	"sentinal-chat/internal/domain/user"
)

type OAuthIdentity struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	Provider       string
	ProviderUserID string
	ProviderEmail  string
	EmailVerified  bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// UserRepository manages user data and related entities.
type UserRepository interface {
	Create(ctx context.Context, u *user.User) error
	GetAllUsers(ctx context.Context, page, limit int) ([]user.User, int64, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (user.User, error)
	UpdateUser(ctx context.Context, u user.User) error
	DeleteUser(ctx context.Context, id uuid.UUID) error

	GetUserByEmail(ctx context.Context, email string) (user.User, error)
	GetUserByUsername(ctx context.Context, username string) (user.User, error)
	GetUserByPhoneNumber(ctx context.Context, phone string) (user.User, error)
	SearchUsers(ctx context.Context, query string, page, limit int) ([]user.User, int64, error)

	UpdateOnlineStatus(ctx context.Context, userID uuid.UUID, isOnline bool) error
	UpdateLastSeen(ctx context.Context, userID uuid.UUID, lastSeen time.Time) error

	GetUserContacts(ctx context.Context, userID uuid.UUID) ([]user.UserContact, error)
	AddUserContact(ctx context.Context, c *user.UserContact) error
	RemoveUserContact(ctx context.Context, userID, contactUserID uuid.UUID) error
	BlockContact(ctx context.Context, userID, contactUserID uuid.UUID) error
	UnblockContact(ctx context.Context, userID, contactUserID uuid.UUID) error
	GetBlockedContacts(ctx context.Context, userID uuid.UUID) ([]user.UserContact, error)

	AddDevice(ctx context.Context, d *user.Device) error
	GetUserDevices(ctx context.Context, userID uuid.UUID) ([]user.Device, error)
	GetDeviceByID(ctx context.Context, deviceID uuid.UUID) (user.Device, error)
	DeactivateDevice(ctx context.Context, deviceID uuid.UUID) error
	UpdateDeviceLastSeen(ctx context.Context, deviceID uuid.UUID) error

	AddFcmToken(ctx context.Context, ft *user.FcmToken) error
	GetUserFcmTokens(ctx context.Context, userID uuid.UUID) ([]user.FcmToken, error)
	DeactivateFcmToken(ctx context.Context, tokenID uuid.UUID) error

	CreateSession(ctx context.Context, s *user.UserSession) error
	GetSessionByID(ctx context.Context, sessionID uuid.UUID) (user.UserSession, error)
	GetSessionByRefreshTokenHash(ctx context.Context, refreshTokenHash string) (user.UserSession, error)
	GetUserSessions(ctx context.Context, userID uuid.UUID) ([]user.UserSession, error)
	UpdateSession(ctx context.Context, s user.UserSession) error
	RevokeSession(ctx context.Context, sessionID uuid.UUID) error
	RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) error
	CleanExpiredSessions(ctx context.Context) error

	UpsertDevice(ctx context.Context, d *user.Device) error
}

type OAuthIdentityRepository interface {
	Create(ctx context.Context, identity *OAuthIdentity) error
	GetByProviderSubject(ctx context.Context, provider, providerUserID string) (OAuthIdentity, error)
}

// ConversationRepository manages conversations and participants.
type ConversationRepository interface {
	Create(ctx context.Context, c *conversation.Conversation) error
	GetByID(ctx context.Context, id uuid.UUID) (conversation.Conversation, error)
	Update(ctx context.Context, c conversation.Conversation) error
	Delete(ctx context.Context, id uuid.UUID) error

	GetUserConversations(ctx context.Context, userID uuid.UUID, page, limit int) ([]conversation.Conversation, int64, error)
	GetDirectConversation(ctx context.Context, userID1, userID2 uuid.UUID) (conversation.Conversation, error)
	SearchConversations(ctx context.Context, userID uuid.UUID, query string) ([]conversation.Conversation, error)
	GetConversationsByType(ctx context.Context, userID uuid.UUID, convType string) ([]conversation.Conversation, error)

	GetByInviteLink(ctx context.Context, link string) (conversation.Conversation, error)
	RegenerateInviteLink(ctx context.Context, conversationID uuid.UUID) (string, error)

	AddParticipant(ctx context.Context, p *conversation.Participant) error
	RemoveParticipant(ctx context.Context, conversationID, userID uuid.UUID) error
	GetParticipants(ctx context.Context, conversationID uuid.UUID) ([]conversation.Participant, error)
	GetParticipant(ctx context.Context, conversationID, userID uuid.UUID) (conversation.Participant, error)
	UpdateParticipantRole(ctx context.Context, conversationID, userID uuid.UUID, role string) error
	IsParticipant(ctx context.Context, conversationID, userID uuid.UUID) (bool, error)
	GetParticipantCount(ctx context.Context, conversationID uuid.UUID) (int64, error)

	MuteConversation(ctx context.Context, conversationID, userID uuid.UUID, until time.Time) error
	UnmuteConversation(ctx context.Context, conversationID, userID uuid.UUID) error
	ArchiveConversation(ctx context.Context, conversationID, userID uuid.UUID) error
	UnarchiveConversation(ctx context.Context, conversationID, userID uuid.UUID) error
	UpdateLastReadSequence(ctx context.Context, conversationID, userID uuid.UUID, seqID int64) error

	ClearConversation(ctx context.Context, conversationID, userID uuid.UUID) error
	GetConversationClear(ctx context.Context, conversationID, userID uuid.UUID) (conversation.ConversationClear, error)

	GetConversationSequence(ctx context.Context, conversationID uuid.UUID) (conversation.ConversationSequence, error)
	IncrementSequence(ctx context.Context, conversationID uuid.UUID) (int64, error)
}

// MessageRepository manages messages and related data.
type MessageRepository interface {
	Create(ctx context.Context, m *message.Message) error
	GetByID(ctx context.Context, id uuid.UUID) (message.Message, error)
	Update(ctx context.Context, m message.Message) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	HardDelete(ctx context.Context, id uuid.UUID) error

	GetConversationMessages(ctx context.Context, conversationID uuid.UUID, beforeSeq int64, limit int) ([]message.Message, error)
	GetMessagesBySeqRange(ctx context.Context, conversationID uuid.UUID, startSeq, endSeq int64) ([]message.Message, error)
	GetUnreadMessages(ctx context.Context, conversationID, userID uuid.UUID) ([]message.Message, error)
	SearchMessages(ctx context.Context, conversationID uuid.UUID, query string, page, limit int) ([]message.Message, int64, error)
	GetMessagesByType(ctx context.Context, conversationID uuid.UUID, msgType string, limit int) ([]message.Message, error)
	GetLatestMessage(ctx context.Context, conversationID uuid.UUID) (message.Message, error)

	MarkAsEdited(ctx context.Context, messageID uuid.UUID) error
	GetMessageCountSince(ctx context.Context, conversationID uuid.UUID, since time.Time) (int64, error)

	GetByClientMessageID(ctx context.Context, conversationID uuid.UUID, clientMsgID string) (message.Message, error)

	AddReaction(ctx context.Context, r *message.MessageReaction) error
	RemoveReaction(ctx context.Context, messageID, userID uuid.UUID, reactionCode string) error
	GetMessageReactions(ctx context.Context, messageID uuid.UUID) ([]message.MessageReaction, error)
	GetUserReaction(ctx context.Context, messageID, userID uuid.UUID) (message.MessageReaction, error)

	CreateReceipt(ctx context.Context, r *message.MessageReceipt) error
	UpdateReceipt(ctx context.Context, r message.MessageReceipt) error
	GetMessageReceipts(ctx context.Context, messageID uuid.UUID) ([]message.MessageReceipt, error)
	MarkAsDelivered(ctx context.Context, messageID, userID uuid.UUID) error
	MarkAsRead(ctx context.Context, messageID, userID uuid.UUID) error
	MarkAsPlayed(ctx context.Context, messageID, userID uuid.UUID) error
	BulkMarkAsDelivered(ctx context.Context, messageIDs []uuid.UUID, userID uuid.UUID) error
	BulkMarkAsRead(ctx context.Context, messageIDs []uuid.UUID, userID uuid.UUID) error

	AddMention(ctx context.Context, m *message.MessageMention) error
	GetMessageMentions(ctx context.Context, messageID uuid.UUID) ([]message.MessageMention, error)
	GetUserMentions(ctx context.Context, userID uuid.UUID, page, limit int) ([]message.Message, int64, error)

	StarMessage(ctx context.Context, s *message.StarredMessage) error
	UnstarMessage(ctx context.Context, userID, messageID uuid.UUID) error
	GetUserStarredMessages(ctx context.Context, userID uuid.UUID, page, limit int) ([]message.StarredMessage, int64, error)
	IsMessageStarred(ctx context.Context, userID, messageID uuid.UUID) (bool, error)

	PinMessage(ctx context.Context, p *message.PinnedMessage) error
	UnpinMessage(ctx context.Context, conversationID, messageID uuid.UUID) error
	GetPinnedMessages(ctx context.Context, conversationID uuid.UUID) ([]message.PinnedMessage, error)

	CreateMessageEdit(ctx context.Context, e *message.MessageEdit) error
	GetMessageEdits(ctx context.Context, messageID uuid.UUID) ([]message.MessageEdit, error)

	CreateAttachment(ctx context.Context, a *message.Attachment) error
	CreateAttachmentWithLink(ctx context.Context, a *message.Attachment, ma *message.MessageAttachment) error
	GetAttachmentByID(ctx context.Context, id uuid.UUID) (message.Attachment, error)
	CanUserAccessAttachment(ctx context.Context, attachmentID, userID uuid.UUID) (bool, error)
	LinkAttachmentToMessage(ctx context.Context, ma *message.MessageAttachment) error
	GetMessageAttachments(ctx context.Context, messageID uuid.UUID) ([]message.Attachment, error)
	MarkViewOnceViewed(ctx context.Context, attachmentID uuid.UUID) error

	CreatePoll(ctx context.Context, p *message.Poll) error
	GetPollByID(ctx context.Context, id uuid.UUID) (message.Poll, error)
	ClosePoll(ctx context.Context, pollID uuid.UUID) error
	AddPollOption(ctx context.Context, o *message.PollOption) error
	GetPollOptions(ctx context.Context, pollID uuid.UUID) ([]message.PollOption, error)
	VotePoll(ctx context.Context, v *message.PollVote) error
	RemoveVote(ctx context.Context, pollID, optionID, userID uuid.UUID) error
	GetPollVotes(ctx context.Context, pollID uuid.UUID) ([]message.PollVote, error)
	GetUserVotes(ctx context.Context, pollID, userID uuid.UUID) ([]message.PollVote, error)

	DeleteExpiredMessages(ctx context.Context) (int64, error)
}

type CallRepository interface {
	Create(ctx context.Context, c *call.Call) error
	GetByID(ctx context.Context, id uuid.UUID) (call.Call, error)
	Update(ctx context.Context, c call.Call) error

	GetConversationCalls(ctx context.Context, conversationID uuid.UUID, page, limit int) ([]call.Call, int64, error)
	GetUserCalls(ctx context.Context, userID uuid.UUID, page, limit int) ([]call.Call, int64, error)
	GetActiveCalls(ctx context.Context, userID uuid.UUID) ([]call.Call, error)
	GetMissedCalls(ctx context.Context, userID uuid.UUID, since time.Time) ([]call.Call, error)

	MarkConnected(ctx context.Context, callID uuid.UUID) error
	EndCall(ctx context.Context, callID uuid.UUID, reason string) error
	GetCallDuration(ctx context.Context, callID uuid.UUID) (int32, error)

	AddParticipant(ctx context.Context, p *call.CallParticipant) error
	RemoveParticipant(ctx context.Context, callID, userID uuid.UUID) error
	GetCallParticipants(ctx context.Context, callID uuid.UUID) ([]call.CallParticipant, error)
	IsCallParticipant(ctx context.Context, callID, userID uuid.UUID) (bool, error)
	UpdateParticipantStatus(ctx context.Context, callID, userID uuid.UUID, status string) error
	UpdateParticipantMuteStatus(ctx context.Context, callID, userID uuid.UUID, audioMuted, videoMuted bool) error
	GetActiveParticipantCount(ctx context.Context, callID uuid.UUID) (int64, error)
}

type OutboxRepository interface {
	Create(ctx context.Context, tx DBTX, event *outbox.OutboxEvent) error
	GetPending(ctx context.Context, limit int) ([]outbox.OutboxEvent, error)
	MarkProcessing(ctx context.Context, id string) (bool, error)
	MarkCompleted(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id string, errorMsg string) error
	IncrementRetry(ctx context.Context, id string) error
	ScheduleRetry(ctx context.Context, id string, nextRetryAt time.Time, errorMsg string) error
}

type CommandRepository interface {
	CreateLog(ctx context.Context, log *command.CommandLog) error
	UpdateLog(ctx context.Context, log *command.CommandLog) error
	GetLogByID(ctx context.Context, id uuid.UUID) (command.CommandLog, error)
	GetPendingCommands(ctx context.Context, limit int) ([]command.CommandLog, error)
	GetCommandsByUser(ctx context.Context, userID uuid.UUID, limit int) ([]command.CommandLog, error)
	CanUndo(ctx context.Context, commandID uuid.UUID, userID uuid.UUID) (bool, error)
}
