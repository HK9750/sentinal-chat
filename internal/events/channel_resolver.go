package events

import (
	"fmt"
)

// ChannelResolver determines which Redis channels to publish to
type ChannelResolver interface {
	ResolveChannels(event Event) []string
}

// HybridChannelResolver routes events to appropriate channels
type HybridChannelResolver struct{}

func NewHybridChannelResolver() *HybridChannelResolver {
	return &HybridChannelResolver{}
}

func (r *HybridChannelResolver) ResolveChannels(event Event) []string {
	var channels []string

	switch e := event.(type) {
	case *MessageNewEvent:
		channels = append(channels, fmt.Sprintf("channel:conversation:%s", e.ConversationID))
	case *MessageReadEvent:
		channels = append(channels, fmt.Sprintf("channel:conversation:%s", e.ConversationID))
	case *MessageDeliveredEvent:
		channels = append(channels, fmt.Sprintf("channel:user:%s", e.RecipientID))
	case *TypingEvent:
		channels = append(channels, fmt.Sprintf("channel:conversation:%s", e.ConversationID))
	case *PresenceEvent:
		channels = append(channels, fmt.Sprintf("channel:user:%s", e.UserID))
	case *CallSignalingEvent:
		channels = append(channels, fmt.Sprintf("channel:user:%s", e.ToID))
	case *CallEndedEvent:
		channels = append(channels, fmt.Sprintf("channel:conversation:%s", e.ConversationID))
	}

	return channels
}
