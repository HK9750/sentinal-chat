package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"sentinal-chat/internal/domain/broadcast"
	"sentinal-chat/internal/domain/call"
	"sentinal-chat/internal/domain/command"
	"sentinal-chat/internal/domain/conversation"
	"sentinal-chat/internal/domain/encryption"
	"sentinal-chat/internal/domain/message"
	"sentinal-chat/internal/domain/outbox"
	"sentinal-chat/internal/domain/upload"
	"sentinal-chat/internal/domain/user"
)

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

	GetUserSettings(ctx context.Context, userID uuid.UUID) (user.UserSettings, error)
	UpdateUserSettings(ctx context.Context, s user.UserSettings) error
	CreateUserSettings(ctx context.Context, s *user.UserSettings) error

	AddDevice(ctx context.Context, d *user.Device) error
	GetUserDevices(ctx context.Context, userID uuid.UUID) ([]user.Device, error)
	GetDeviceByID(ctx context.Context, deviceID uuid.UUID) (user.Device, error)
	DeactivateDevice(ctx context.Context, deviceID uuid.UUID) error
	UpdateDeviceLastSeen(ctx context.Context, deviceID uuid.UUID) error

	AddPushToken(ctx context.Context, pt *user.PushToken) error
	GetUserPushTokens(ctx context.Context, userID uuid.UUID) ([]user.PushToken, error)
	DeactivatePushToken(ctx context.Context, tokenID uuid.UUID) error

	CreateSession(ctx context.Context, s *user.UserSession) error
	GetSessionByID(ctx context.Context, sessionID uuid.UUID) (user.UserSession, error)
	GetUserSessions(ctx context.Context, userID uuid.UUID) ([]user.UserSession, error)
	UpdateSession(ctx context.Context, s user.UserSession) error
	RevokeSession(ctx context.Context, sessionID uuid.UUID) error
	RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) error
	CleanExpiredSessions(ctx context.Context) error
}

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
	PinConversation(ctx context.Context, conversationID, userID uuid.UUID) error
	UnpinConversation(ctx context.Context, conversationID, userID uuid.UUID) error
	ArchiveConversation(ctx context.Context, conversationID, userID uuid.UUID) error
	UnarchiveConversation(ctx context.Context, conversationID, userID uuid.UUID) error
	UpdateLastReadSequence(ctx context.Context, conversationID, userID uuid.UUID, seqID int64) error

	GetConversationSequence(ctx context.Context, conversationID uuid.UUID) (conversation.ConversationSequence, error)
	IncrementSequence(ctx context.Context, conversationID uuid.UUID) (int64, error)
}

type MessageRepository interface {
	Create(ctx context.Context, m *message.Message) error
	GetByID(ctx context.Context, id uuid.UUID) (message.Message, error)
	Update(ctx context.Context, m message.Message) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	HardDelete(ctx context.Context, id uuid.UUID) error
	CreateCiphertext(ctx context.Context, c *message.MessageCiphertext) error

	GetConversationMessages(ctx context.Context, conversationID uuid.UUID, beforeSeq int64, limit int, recipientDeviceID uuid.UUID) ([]message.Message, error)
	GetMessagesBySeqRange(ctx context.Context, conversationID uuid.UUID, startSeq, endSeq int64) ([]message.Message, error)
	GetUnreadMessages(ctx context.Context, conversationID, userID uuid.UUID) ([]message.Message, error)
	SearchMessages(ctx context.Context, conversationID uuid.UUID, query string, page, limit int) ([]message.Message, int64, error)
	GetMessagesByType(ctx context.Context, conversationID uuid.UUID, msgType string, limit int) ([]message.Message, error)
	GetLatestMessage(ctx context.Context, conversationID uuid.UUID) (message.Message, error)

	MarkAsEdited(ctx context.Context, messageID uuid.UUID) error
	GetMessageCountSince(ctx context.Context, conversationID uuid.UUID, since time.Time) (int64, error)

	GetByIdempotencyKey(ctx context.Context, key string) (message.Message, error)
	GetByClientMessageID(ctx context.Context, clientMsgID string) (message.Message, error)

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

	CreateAttachment(ctx context.Context, a *message.Attachment) error
	GetAttachmentByID(ctx context.Context, id uuid.UUID) (message.Attachment, error)
	LinkAttachmentToMessage(ctx context.Context, ma *message.MessageAttachment) error
	GetMessageAttachments(ctx context.Context, messageID uuid.UUID) ([]message.Attachment, error)
	MarkViewOnceViewed(ctx context.Context, attachmentID uuid.UUID) error

	CreateLinkPreview(ctx context.Context, lp *message.LinkPreview) error
	GetLinkPreviewByHash(ctx context.Context, urlHash string) (message.LinkPreview, error)
	GetLinkPreviewByID(ctx context.Context, id uuid.UUID) (message.LinkPreview, error)

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

	RecordQualityMetric(ctx context.Context, m *call.CallQualityMetric) error
	GetCallQualityMetrics(ctx context.Context, callID uuid.UUID) ([]call.CallQualityMetric, error)
	GetUserCallQualityMetrics(ctx context.Context, callID, userID uuid.UUID) ([]call.CallQualityMetric, error)
	GetAverageCallQuality(ctx context.Context, callID uuid.UUID) (float64, error)
}

type BroadcastRepository interface {
	Create(ctx context.Context, b *broadcast.BroadcastList) error
	GetByID(ctx context.Context, id uuid.UUID) (broadcast.BroadcastList, error)
	Update(ctx context.Context, b broadcast.BroadcastList) error
	Delete(ctx context.Context, id uuid.UUID) error

	GetUserBroadcastLists(ctx context.Context, ownerID uuid.UUID) ([]broadcast.BroadcastList, error)
	SearchBroadcastLists(ctx context.Context, ownerID uuid.UUID, query string) ([]broadcast.BroadcastList, error)

	AddRecipient(ctx context.Context, r *broadcast.BroadcastRecipient) error
	RemoveRecipient(ctx context.Context, broadcastID, userID uuid.UUID) error
	GetRecipients(ctx context.Context, broadcastID uuid.UUID) ([]broadcast.BroadcastRecipient, error)
	GetRecipientCount(ctx context.Context, broadcastID uuid.UUID) (int64, error)
	IsRecipient(ctx context.Context, broadcastID, userID uuid.UUID) (bool, error)
	BulkAddRecipients(ctx context.Context, broadcastID uuid.UUID, userIDs []uuid.UUID) error
	BulkRemoveRecipients(ctx context.Context, broadcastID uuid.UUID, userIDs []uuid.UUID) error
}

type EncryptionRepository interface {
	IsDeviceOwnedByUser(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID) (bool, error)
	CreateIdentityKey(ctx context.Context, k *encryption.IdentityKey) error
	GetIdentityKey(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID) (encryption.IdentityKey, error)
	GetUserIdentityKeys(ctx context.Context, userID uuid.UUID) ([]encryption.IdentityKey, error)
	DeactivateIdentityKey(ctx context.Context, id uuid.UUID) error
	DeleteIdentityKey(ctx context.Context, id uuid.UUID) error

	CreateSignedPreKey(ctx context.Context, k *encryption.SignedPreKey) error
	GetSignedPreKey(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID, keyID int) (encryption.SignedPreKey, error)
	GetActiveSignedPreKey(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID) (encryption.SignedPreKey, error)
	RotateSignedPreKey(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID, newKey *encryption.SignedPreKey) error
	DeactivateSignedPreKey(ctx context.Context, id uuid.UUID) error

	UploadOneTimePreKeys(ctx context.Context, keys []encryption.OneTimePreKey) error
	ConsumeOneTimePreKey(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID, consumedBy uuid.UUID, consumedByDeviceID uuid.UUID) (encryption.OneTimePreKey, error)
	GetAvailablePreKeyCount(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID) (int64, error)
	DeleteConsumedPreKeys(ctx context.Context, olderThan time.Time) (int64, error)

	HasActiveKeys(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID) (bool, error)
}

type UploadRepository interface {
	Create(ctx context.Context, u *upload.UploadSession) error
	GetByID(ctx context.Context, id uuid.UUID) (upload.UploadSession, error)
	Update(ctx context.Context, u upload.UploadSession) error
	Delete(ctx context.Context, id uuid.UUID) error

	GetUserUploadSessions(ctx context.Context, uploaderID uuid.UUID) ([]upload.UploadSession, error)
	GetInProgressUploads(ctx context.Context, uploaderID uuid.UUID) ([]upload.UploadSession, error)
	GetCompletedUploads(ctx context.Context, uploaderID uuid.UUID, page, limit int) ([]upload.UploadSession, int64, error)

	UpdateProgress(ctx context.Context, sessionID uuid.UUID, uploadedBytes int64) error
	MarkCompleted(ctx context.Context, sessionID uuid.UUID) error
	MarkFailed(ctx context.Context, sessionID uuid.UUID) error

	GetStaleUploads(ctx context.Context, olderThan time.Duration) ([]upload.UploadSession, error)
	DeleteStaleUploads(ctx context.Context, olderThan time.Duration) (int64, error)
}

type OutboxRepository interface {
	Create(ctx context.Context, tx *gorm.DB, event *outbox.OutboxEvent) error
	GetPending(ctx context.Context, limit int) ([]outbox.OutboxEvent, error)
	MarkProcessing(ctx context.Context, id string) error
	MarkCompleted(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id string, errorMsg string) error
	IncrementRetry(ctx context.Context, id string) error
}

type CommandRepository interface {
	CreateLog(ctx context.Context, log *command.CommandLog) error
	UpdateLog(ctx context.Context, log *command.CommandLog) error
	GetLogByID(ctx context.Context, id uuid.UUID) (command.CommandLog, error)
	GetPendingCommands(ctx context.Context, limit int) ([]command.CommandLog, error)
	GetCommandsByUser(ctx context.Context, userID uuid.UUID, limit int) ([]command.CommandLog, error)
	CanUndo(ctx context.Context, commandID uuid.UUID, userID uuid.UUID) (bool, error)
}
