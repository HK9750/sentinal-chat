package commands

import (
	"strings"
	"time"

	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
)

// CreateDMCommand creates a direct message conversation
type CreateDMCommand struct {
	CreatorID           uuid.UUID
	OtherUserID         uuid.UUID
	IdempotencyKeyValue string
}

func (CreateDMCommand) CommandType() string { return "conversation.create_dm" }

func (c CreateDMCommand) Validate() error {
	if c.CreatorID == uuid.Nil || c.OtherUserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	if c.CreatorID == c.OtherUserID {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c CreateDMCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c CreateDMCommand) ActorID() uuid.UUID { return c.CreatorID }

// CreateGroupCommand creates a group conversation
type CreateGroupCommand struct {
	CreatorID           uuid.UUID
	Subject             string
	Description         string
	AvatarURL           string
	ParticipantIDs      []uuid.UUID
	IdempotencyKeyValue string
}

func (CreateGroupCommand) CommandType() string { return "conversation.create_group" }

func (c CreateGroupCommand) Validate() error {
	if c.CreatorID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	if strings.TrimSpace(c.Subject) == "" {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c CreateGroupCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c CreateGroupCommand) ActorID() uuid.UUID { return c.CreatorID }

// CreateConversationCommand creates a conversation (generic)
type CreateConversationCommand struct {
	Type           string // DM or GROUP
	Subject        string
	Description    string
	CreatorID      uuid.UUID
	ParticipantIDs []uuid.UUID
}

func (CreateConversationCommand) CommandType() string {
	return "conversation.create"
}

func (c CreateConversationCommand) Validate() error {
	if c.CreatorID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	if c.Type != "DM" && c.Type != "GROUP" {
		return sentinal_errors.ErrInvalidInput
	}
	if c.Type == "GROUP" && strings.TrimSpace(c.Subject) == "" {
		return sentinal_errors.ErrInvalidInput
	}
	if len(c.ParticipantIDs) == 0 {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c CreateConversationCommand) IdempotencyKey() string {
	return ""
}

func (c CreateConversationCommand) ActorID() uuid.UUID {
	return c.CreatorID
}

// UpdateGroupCommand updates group info
type UpdateGroupCommand struct {
	ConversationID       uuid.UUID
	UserID               uuid.UUID
	Subject              string
	Description          string
	AvatarURL            string
	DisappearingMode     string
	MessageExpirySeconds *int
	IdempotencyKeyValue  string
}

func (UpdateGroupCommand) CommandType() string { return "conversation.update_group" }

func (c UpdateGroupCommand) Validate() error {
	if c.ConversationID == uuid.Nil || c.UserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c UpdateGroupCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c UpdateGroupCommand) ActorID() uuid.UUID { return c.UserID }

// AddMemberCommand adds a member to a group
type AddMemberCommand struct {
	ConversationID      uuid.UUID
	UserID              uuid.UUID // actor
	NewMemberID         uuid.UUID
	Role                string
	IdempotencyKeyValue string
}

func (AddMemberCommand) CommandType() string { return "conversation.add_member" }

func (c AddMemberCommand) Validate() error {
	if c.ConversationID == uuid.Nil || c.UserID == uuid.Nil || c.NewMemberID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c AddMemberCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c AddMemberCommand) ActorID() uuid.UUID { return c.UserID }

// RemoveMemberCommand removes a member from a group
type RemoveMemberCommand struct {
	ConversationID      uuid.UUID
	UserID              uuid.UUID // actor
	MemberID            uuid.UUID
	IdempotencyKeyValue string
}

func (RemoveMemberCommand) CommandType() string { return "conversation.remove_member" }

func (c RemoveMemberCommand) Validate() error {
	if c.ConversationID == uuid.Nil || c.UserID == uuid.Nil || c.MemberID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c RemoveMemberCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c RemoveMemberCommand) ActorID() uuid.UUID { return c.UserID }

// LeaveGroupCommand leaves a group
type LeaveGroupCommand struct {
	ConversationID      uuid.UUID
	UserID              uuid.UUID
	IdempotencyKeyValue string
}

func (LeaveGroupCommand) CommandType() string { return "conversation.leave" }

func (c LeaveGroupCommand) Validate() error {
	if c.ConversationID == uuid.Nil || c.UserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c LeaveGroupCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c LeaveGroupCommand) ActorID() uuid.UUID { return c.UserID }

// ChangeRoleCommand changes a member's role
type ChangeRoleCommand struct {
	ConversationID      uuid.UUID
	UserID              uuid.UUID // actor
	MemberID            uuid.UUID
	NewRole             string // OWNER, ADMIN, MEMBER
	IdempotencyKeyValue string
}

func (ChangeRoleCommand) CommandType() string { return "conversation.change_role" }

func (c ChangeRoleCommand) Validate() error {
	if c.ConversationID == uuid.Nil || c.UserID == uuid.Nil || c.MemberID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	validRoles := map[string]bool{"OWNER": true, "ADMIN": true, "MEMBER": true}
	if !validRoles[c.NewRole] {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c ChangeRoleCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c ChangeRoleCommand) ActorID() uuid.UUID { return c.UserID }

// MuteConversationCommand mutes a conversation
type MuteConversationCommand struct {
	ConversationID      uuid.UUID
	UserID              uuid.UUID
	MutedUntil          time.Time
	IdempotencyKeyValue string
}

func (MuteConversationCommand) CommandType() string { return "conversation.mute" }

func (c MuteConversationCommand) Validate() error {
	if c.ConversationID == uuid.Nil || c.UserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c MuteConversationCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c MuteConversationCommand) ActorID() uuid.UUID { return c.UserID }

// UnmuteConversationCommand unmutes a conversation
type UnmuteConversationCommand struct {
	ConversationID      uuid.UUID
	UserID              uuid.UUID
	IdempotencyKeyValue string
}

func (UnmuteConversationCommand) CommandType() string { return "conversation.unmute" }

func (c UnmuteConversationCommand) Validate() error {
	if c.ConversationID == uuid.Nil || c.UserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c UnmuteConversationCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c UnmuteConversationCommand) ActorID() uuid.UUID { return c.UserID }

// ArchiveConversationCommand archives a conversation
type ArchiveConversationCommand struct {
	ConversationID      uuid.UUID
	UserID              uuid.UUID
	IdempotencyKeyValue string
}

func (ArchiveConversationCommand) CommandType() string { return "conversation.archive" }

func (c ArchiveConversationCommand) Validate() error {
	if c.ConversationID == uuid.Nil || c.UserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c ArchiveConversationCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c ArchiveConversationCommand) ActorID() uuid.UUID { return c.UserID }

// UnarchiveConversationCommand unarchives a conversation
type UnarchiveConversationCommand struct {
	ConversationID      uuid.UUID
	UserID              uuid.UUID
	IdempotencyKeyValue string
}

func (UnarchiveConversationCommand) CommandType() string { return "conversation.unarchive" }

func (c UnarchiveConversationCommand) Validate() error {
	if c.ConversationID == uuid.Nil || c.UserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c UnarchiveConversationCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c UnarchiveConversationCommand) ActorID() uuid.UUID { return c.UserID }

// PinConversationCommand pins a conversation
type PinConversationCommand struct {
	ConversationID      uuid.UUID
	UserID              uuid.UUID
	IdempotencyKeyValue string
}

func (PinConversationCommand) CommandType() string { return "conversation.pin" }

func (c PinConversationCommand) Validate() error {
	if c.ConversationID == uuid.Nil || c.UserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c PinConversationCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c PinConversationCommand) ActorID() uuid.UUID { return c.UserID }

// UnpinConversationCommand unpins a conversation
type UnpinConversationCommand struct {
	ConversationID      uuid.UUID
	UserID              uuid.UUID
	IdempotencyKeyValue string
}

func (UnpinConversationCommand) CommandType() string { return "conversation.unpin" }

func (c UnpinConversationCommand) Validate() error {
	if c.ConversationID == uuid.Nil || c.UserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c UnpinConversationCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c UnpinConversationCommand) ActorID() uuid.UUID { return c.UserID }

// GenerateInviteLinkCommand generates a group invite link
type GenerateInviteLinkCommand struct {
	ConversationID      uuid.UUID
	UserID              uuid.UUID
	IdempotencyKeyValue string
}

func (GenerateInviteLinkCommand) CommandType() string { return "conversation.generate_invite_link" }

func (c GenerateInviteLinkCommand) Validate() error {
	if c.ConversationID == uuid.Nil || c.UserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c GenerateInviteLinkCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c GenerateInviteLinkCommand) ActorID() uuid.UUID { return c.UserID }

// RevokeInviteLinkCommand revokes a group invite link
type RevokeInviteLinkCommand struct {
	ConversationID      uuid.UUID
	UserID              uuid.UUID
	IdempotencyKeyValue string
}

func (RevokeInviteLinkCommand) CommandType() string { return "conversation.revoke_invite_link" }

func (c RevokeInviteLinkCommand) Validate() error {
	if c.ConversationID == uuid.Nil || c.UserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c RevokeInviteLinkCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c RevokeInviteLinkCommand) ActorID() uuid.UUID { return c.UserID }

// JoinViaInviteLinkCommand joins a group via invite link
type JoinViaInviteLinkCommand struct {
	InviteLink          string
	UserID              uuid.UUID
	IdempotencyKeyValue string
}

func (JoinViaInviteLinkCommand) CommandType() string { return "conversation.join_via_invite_link" }

func (c JoinViaInviteLinkCommand) Validate() error {
	if c.InviteLink == "" || c.UserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c JoinViaInviteLinkCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c JoinViaInviteLinkCommand) ActorID() uuid.UUID { return c.UserID }

// ClearConversationCommand clears conversation history for a user
type ClearConversationCommand struct {
	ConversationID      uuid.UUID
	UserID              uuid.UUID
	IdempotencyKeyValue string
}

func (ClearConversationCommand) CommandType() string { return "conversation.clear" }

func (c ClearConversationCommand) Validate() error {
	if c.ConversationID == uuid.Nil || c.UserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c ClearConversationCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c ClearConversationCommand) ActorID() uuid.UUID { return c.UserID }

// UpdateReadPositionCommand updates last read sequence
type UpdateReadPositionCommand struct {
	ConversationID uuid.UUID
	UserID         uuid.UUID
	LastReadSeqID  int64
}

func (UpdateReadPositionCommand) CommandType() string { return "conversation.update_read_position" }

func (c UpdateReadPositionCommand) Validate() error {
	if c.ConversationID == uuid.Nil || c.UserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c UpdateReadPositionCommand) IdempotencyKey() string { return "" }

func (c UpdateReadPositionCommand) ActorID() uuid.UUID { return c.UserID }
