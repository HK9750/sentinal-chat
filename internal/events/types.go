package events

const (
	ConnectionReady                = "connection:ready"
	PresenceUpdate                 = "presence:update"
	TypingStarted                  = "typing:started"
	TypingStopped                  = "typing:stopped"
	ConversationCreated            = "conversation:created"
	ConversationParticipantAdded   = "conversation:participant_added"
	ConversationParticipantRemoved = "conversation:participant_removed"
	MessageNew                     = "message:new"
	MessageEdited                  = "message:edited"
	MessageDeleted                 = "message:deleted"
	MessageReaction                = "message:reaction"
	MessagePinned                  = "message:pinned"
	MessageUnpinned                = "message:unpinned"
	ReceiptUpdate                  = "receipt:update"
	PollUpdate                     = "poll:update"
	CallIncoming                   = "call:incoming"
	CallOffer                      = "call:offer"
	CallAnswer                     = "call:answer"
	CallICE                        = "call:ice"
	CallEnded                      = "call:ended"
	ErrorEvent                     = "error"
	InboundPing                    = "ping"
	InboundTypingStart             = "typing:start"
	InboundTypingStop              = "typing:stop"
	InboundMessageSend             = "message:send"
	InboundMessageEdit             = "message:edit"
	InboundMessageDelete           = "message:delete"
	InboundMessageReactionAdd      = "message:reaction:add"
	InboundMessageReactionRemove   = "message:reaction:remove"
	InboundMessagePin              = "message:pin"
	InboundMessageUnpin            = "message:unpin"
	InboundReceiptDelivered        = "receipt:delivered"
	InboundReceiptRead             = "receipt:read"
	InboundReceiptPlayed           = "receipt:played"
	InboundPollCreate              = "poll:create"
	InboundPollVote                = "poll:vote"
	InboundPollClose               = "poll:close"
	InboundCallStart               = "call:start"
	InboundCallOffer               = "call:offer"
	InboundCallAnswer              = "call:answer"
	InboundCallICE                 = "call:ice"
	InboundCallEnd                 = "call:end"
)

func ConversationChannel(conversationID string) string {
	return "conversation:" + conversationID
}

func UserChannel(userID string) string {
	return "user:" + userID
}

func CallChannel(callID string) string {
	return "call:" + callID
}
