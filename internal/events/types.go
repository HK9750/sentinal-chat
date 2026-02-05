package events

// Event type constants as defined in Appendix O of database.md
// These follow the format: domain.action

// Message events
const (
	EventTypeMessageCreated   = "message.created"
	EventTypeMessageUpdated   = "message.updated"
	EventTypeMessageDeleted   = "message.deleted"
	EventTypeMessageExpired   = "message.expired"
	EventTypeMessageForwarded = "message.forwarded"
	EventTypeMessageStarred   = "message.starred"
	EventTypeMessageUnstarred = "message.unstarred"
	EventTypeMessagePinned    = "message.pinned"
	EventTypeMessageUnpinned  = "message.unpinned"
)

// Receipt events
const (
	EventTypeReceiptSent      = "receipt.sent"
	EventTypeReceiptDelivered = "receipt.delivered"
	EventTypeReceiptRead      = "receipt.read"
	EventTypeReceiptPlayed    = "receipt.played"
)

// Reaction events
const (
	EventTypeReactionAdded   = "reaction.added"
	EventTypeReactionRemoved = "reaction.removed"
)

// Typing and presence events
const (
	EventTypeTypingStarted           = "typing.started"
	EventTypeTypingStopped           = "typing.stopped"
	EventTypePresenceOnline          = "presence.online"
	EventTypePresenceOffline         = "presence.offline"
	EventTypePresenceLastSeenUpdated = "presence.last_seen_updated"
)

// Conversation events
const (
	EventTypeConversationCreated           = "conversation.created"
	EventTypeConversationUpdated           = "conversation.updated"
	EventTypeConversationArchived          = "conversation.archived"
	EventTypeConversationUnarchived        = "conversation.unarchived"
	EventTypeConversationMuted             = "conversation.muted"
	EventTypeConversationUnmuted           = "conversation.unmuted"
	EventTypeConversationPinned            = "conversation.pinned"
	EventTypeConversationUnpinned          = "conversation.unpinned"
	EventTypeConversationInviteLinkCreated = "conversation.invite_link_created"
	EventTypeConversationInviteLinkRevoked = "conversation.invite_link_revoked"
)

// Participant events
const (
	EventTypeParticipantAdded       = "participant.added"
	EventTypeParticipantRemoved     = "participant.removed"
	EventTypeParticipantLeft        = "participant.left"
	EventTypeParticipantRoleChanged = "participant.role_changed"
	EventTypeParticipantMuted       = "participant.muted"
	EventTypeParticipantUnmuted     = "participant.unmuted"
)

// Call events
const (
	EventTypeCallInitiated          = "call.initiated"
	EventTypeCallRinging            = "call.ringing"
	EventTypeCallAccepted           = "call.accepted"
	EventTypeCallRejected           = "call.rejected"
	EventTypeCallEnded              = "call.ended"
	EventTypeCallMissed             = "call.missed"
	EventTypeCallParticipantJoined  = "call.participant_joined"
	EventTypeCallParticipantLeft    = "call.participant_left"
	EventTypeCallVideoEnabled       = "call.video_enabled"
	EventTypeCallVideoDisabled      = "call.video_disabled"
	EventTypeCallScreenShareStarted = "call.screen_share_started"
	EventTypeCallScreenShareStopped = "call.screen_share_stopped"
)

// WebRTC signaling events (real-time only, not persisted)
const (
	EventTypeCallOffer        = "call.offer"
	EventTypeCallAnswer       = "call.answer"
	EventTypeCallICECandidate = "call.ice_candidate"
)

// Poll events
const (
	EventTypePollCreated       = "poll.created"
	EventTypePollVoted         = "poll.voted"
	EventTypePollVoteRetracted = "poll.vote_retracted"
	EventTypePollClosed        = "poll.closed"
)

// Broadcast events
const (
	EventTypeBroadcastCreated   = "broadcast.created"
	EventTypeBroadcastUpdated   = "broadcast.updated"
	EventTypeBroadcastDeleted   = "broadcast.deleted"
	EventTypeBroadcastSent      = "broadcast.sent"
	EventTypeBroadcastDelivered = "broadcast.delivered"
)

// Upload events
const (
	EventTypeUploadCreated   = "upload.created"
	EventTypeUploadProgress  = "upload.progress"
	EventTypeUploadCompleted = "upload.completed"
	EventTypeUploadFailed    = "upload.failed"
	EventTypeUploadDeleted   = "upload.deleted"
)

// Encryption key events
const (
	EventTypeKeysPrekeyLow       = "keys.prekey_low"
	EventTypeKeysIdentityChanged = "keys.identity_changed"
	EventTypeKeysPrekeyUploaded  = "keys.prekey_uploaded"
)

// User events
const (
	EventTypeUserRegistered       = "user.registered"
	EventTypeUserUpdated          = "user.updated"
	EventTypeUserBlocked          = "user.blocked"
	EventTypeUserUnblocked        = "user.unblocked"
	EventTypeUserContactAdded     = "user.contact_added"
	EventTypeUserContactRemoved   = "user.contact_removed"
	EventTypeUserDeviceRegistered = "user.device_registered"
	EventTypeUserSettingsUpdated  = "user.settings_updated"
)

// Aggregate type constants
const (
	AggregateTypeMessage        = "message"
	AggregateTypeMessageReceipt = "message_receipt"
	AggregateTypeReaction       = "reaction"
	AggregateTypeTyping         = "typing"
	AggregateTypePresence       = "presence"
	AggregateTypeConversation   = "conversation"
	AggregateTypeParticipant    = "participant"
	AggregateTypeCall           = "call"
	AggregateTypePoll           = "poll"
	AggregateTypeBroadcast      = "broadcast"
	AggregateTypeUpload         = "upload"
	AggregateTypeEncryption     = "encryption"
	AggregateTypeUser           = "user"
)

// Redis channel prefixes as defined in Appendix B
const (
	ChannelPrefixConversation = "channel:conversation:"
	ChannelPrefixCall         = "channel:call:"
	ChannelPrefixPresence     = "channel:presence:"
	ChannelPrefixUser         = "channel:user:"
	ChannelPrefixBroadcast    = "channel:broadcast:"
	ChannelPrefixUpload       = "channel:upload:"
	ChannelSystemOutbox       = "channel:system:outbox"
)
