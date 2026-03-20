package events

const (
	ConnectionReady                = "connection:ready"
	PresenceUpdate                 = "presence:update"
	TypingStarted                  = "typing:started"
	TypingStopped                  = "typing:stopped"
	ConversationCleared            = "conversation:cleared"
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
	CommandUndone                  = "command:undone"
	CommandRedone                  = "command:redone"
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
	InboundPollVote                = "poll:vote"
	InboundPollClose               = "poll:close"
	InboundCommandUndo             = "command:undo"
	InboundCommandRedo             = "command:redo"
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

func DeviceChannel(deviceID string) string {
	return "device:" + deviceID
}

func CallChannel(callID string) string {
	return "call:" + callID
}
