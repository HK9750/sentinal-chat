package proxy

import (
	"context"

	"sentinal-chat/internal/commands"
	"sentinal-chat/internal/repository"
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
)

// AccessControl implements the Proxy pattern for authorization
type AccessControl struct {
	eventRepo        repository.EventRepository
	conversationRepo repository.ConversationRepository
	broadcastRepo    repository.BroadcastRepository
	uploadRepo       repository.UploadRepository
	callRepo         repository.CallRepository
	messageRepo      repository.MessageRepository
}

// NewAccessControl creates a new AccessControl proxy
func NewAccessControl(
	eventRepo repository.EventRepository,
	conversationRepo repository.ConversationRepository,
	broadcastRepo repository.BroadcastRepository,
	uploadRepo repository.UploadRepository,
) *AccessControl {
	return &AccessControl{
		eventRepo:        eventRepo,
		conversationRepo: conversationRepo,
		broadcastRepo:    broadcastRepo,
		uploadRepo:       uploadRepo,
	}
}

// SetCallRepo sets the call repository (optional dependency)
func (a *AccessControl) SetCallRepo(repo repository.CallRepository) {
	a.callRepo = repo
}

// SetMessageRepo sets the message repository (optional dependency)
func (a *AccessControl) SetMessageRepo(repo repository.MessageRepository) {
	a.messageRepo = repo
}

// CanSendMessage checks if user can send message to conversation
func (a *AccessControl) CanSendMessage(ctx context.Context, userID, conversationID uuid.UUID) error {
	if a.eventRepo != nil {
		ok, err := a.eventRepo.HasAccessPolicy(ctx, "conversation", conversationID, "user", userID, "message.send")
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}
	return a.ensureParticipant(ctx, conversationID, userID)
}

// CanViewConversation checks if user can view a conversation
func (a *AccessControl) CanViewConversation(ctx context.Context, userID, conversationID uuid.UUID) error {
	if a.eventRepo != nil {
		ok, err := a.eventRepo.HasAccessPolicy(ctx, "conversation", conversationID, "user", userID, "conversation.view")
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}
	return a.ensureParticipant(ctx, conversationID, userID)
}

// CanManageGroup checks if user can manage group (add/remove members, update info)
func (a *AccessControl) CanManageGroup(ctx context.Context, userID, conversationID uuid.UUID) error {
	if a.conversationRepo == nil {
		return sentinal_errors.ErrForbidden
	}
	participant, err := a.conversationRepo.GetParticipant(ctx, conversationID, userID)
	if err != nil {
		return err
	}
	if participant.Role != "OWNER" && participant.Role != "ADMIN" {
		return sentinal_errors.ErrForbidden
	}
	return nil
}

// CanDeleteGroup checks if user can delete the group (owner only)
func (a *AccessControl) CanDeleteGroup(ctx context.Context, userID, conversationID uuid.UUID) error {
	if a.conversationRepo == nil {
		return sentinal_errors.ErrForbidden
	}
	participant, err := a.conversationRepo.GetParticipant(ctx, conversationID, userID)
	if err != nil {
		return err
	}
	if participant.Role != "OWNER" {
		return sentinal_errors.ErrForbidden
	}
	return nil
}

// CanInitiateCall checks if user can initiate a call
func (a *AccessControl) CanInitiateCall(ctx context.Context, userID, conversationID uuid.UUID) error {
	if a.eventRepo != nil {
		ok, err := a.eventRepo.HasAccessPolicy(ctx, "conversation", conversationID, "user", userID, "call.start")
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}
	return a.ensureParticipant(ctx, conversationID, userID)
}

// CanJoinCall checks if user can join a call
func (a *AccessControl) CanJoinCall(ctx context.Context, userID, callID uuid.UUID) error {
	if a.callRepo == nil {
		return sentinal_errors.ErrForbidden
	}
	call, err := a.callRepo.GetByID(ctx, callID)
	if err != nil {
		return err
	}
	return a.ensureParticipant(ctx, call.ConversationID, userID)
}

// CanManageCall checks if user can manage call (end, etc.)
func (a *AccessControl) CanManageCall(ctx context.Context, userID, callID uuid.UUID) error {
	if a.callRepo == nil {
		return sentinal_errors.ErrForbidden
	}
	// Call initiator or participant can manage
	participants, err := a.callRepo.GetCallParticipants(ctx, callID)
	if err != nil {
		return err
	}
	for _, p := range participants {
		if p.UserID == userID {
			return nil
		}
	}
	return sentinal_errors.ErrForbidden
}

// CanManageBroadcast checks if user owns the broadcast list
func (a *AccessControl) CanManageBroadcast(ctx context.Context, userID, broadcastID uuid.UUID) error {
	if a.broadcastRepo == nil {
		return sentinal_errors.ErrForbidden
	}
	b, err := a.broadcastRepo.GetByID(ctx, broadcastID)
	if err != nil {
		return err
	}
	if b.OwnerID != userID {
		return sentinal_errors.ErrForbidden
	}
	return nil
}

// CanAccessUpload checks if user owns the upload
func (a *AccessControl) CanAccessUpload(ctx context.Context, userID, uploadID uuid.UUID) error {
	if a.uploadRepo == nil {
		return sentinal_errors.ErrForbidden
	}
	u, err := a.uploadRepo.GetByID(ctx, uploadID)
	if err != nil {
		return err
	}
	if u.UploaderID != userID {
		return sentinal_errors.ErrForbidden
	}
	return nil
}

// CanEditMessage checks if user can edit a message
func (a *AccessControl) CanEditMessage(ctx context.Context, userID, messageID uuid.UUID) error {
	if a.messageRepo == nil {
		return sentinal_errors.ErrForbidden
	}
	msg, err := a.messageRepo.GetByID(ctx, messageID)
	if err != nil {
		return err
	}
	if msg.SenderID != userID {
		return sentinal_errors.ErrForbidden
	}
	return nil
}

// CanDeleteMessage checks if user can delete a message
func (a *AccessControl) CanDeleteMessage(ctx context.Context, userID, messageID uuid.UUID, deleteForEveryone bool) error {
	if a.messageRepo == nil {
		return sentinal_errors.ErrForbidden
	}
	msg, err := a.messageRepo.GetByID(ctx, messageID)
	if err != nil {
		return err
	}
	// User can always delete for self
	if !deleteForEveryone {
		return a.ensureParticipant(ctx, msg.ConversationID, userID)
	}
	// Delete for everyone: must be sender or admin/owner
	if msg.SenderID == userID {
		return nil
	}
	if a.conversationRepo != nil {
		participant, err := a.conversationRepo.GetParticipant(ctx, msg.ConversationID, userID)
		if err != nil {
			return err
		}
		if participant.Role == "OWNER" || participant.Role == "ADMIN" {
			return nil
		}
	}
	return sentinal_errors.ErrForbidden
}

// CanReactToMessage checks if user can react to a message
func (a *AccessControl) CanReactToMessage(ctx context.Context, userID, messageID uuid.UUID) error {
	if a.messageRepo == nil {
		return sentinal_errors.ErrForbidden
	}
	msg, err := a.messageRepo.GetByID(ctx, messageID)
	if err != nil {
		return err
	}
	return a.ensureParticipant(ctx, msg.ConversationID, userID)
}

// CanChangeRole checks if user can change another user's role
func (a *AccessControl) CanChangeRole(ctx context.Context, userID, conversationID uuid.UUID, targetRole string) error {
	if a.conversationRepo == nil {
		return sentinal_errors.ErrForbidden
	}
	actor, err := a.conversationRepo.GetParticipant(ctx, conversationID, userID)
	if err != nil {
		return err
	}
	// Only owner can change to owner or demote from admin
	if targetRole == "OWNER" && actor.Role != "OWNER" {
		return sentinal_errors.ErrForbidden
	}
	// Admin can change member roles but not other admins
	if actor.Role == "ADMIN" && targetRole != "MEMBER" {
		return sentinal_errors.ErrForbidden
	}
	if actor.Role == "MEMBER" {
		return sentinal_errors.ErrForbidden
	}
	return nil
}

// CanRemoveMember checks if user can remove a member
func (a *AccessControl) CanRemoveMember(ctx context.Context, userID, conversationID, memberID uuid.UUID) error {
	if a.conversationRepo == nil {
		return sentinal_errors.ErrForbidden
	}
	actor, err := a.conversationRepo.GetParticipant(ctx, conversationID, userID)
	if err != nil {
		return err
	}
	if actor.Role == "MEMBER" {
		return sentinal_errors.ErrForbidden
	}
	// Admin cannot remove owner or other admins
	if actor.Role == "ADMIN" {
		target, err := a.conversationRepo.GetParticipant(ctx, conversationID, memberID)
		if err != nil {
			return err
		}
		if target.Role == "OWNER" || target.Role == "ADMIN" {
			return sentinal_errors.ErrForbidden
		}
	}
	return nil
}

// CanGenerateInviteLink checks if user can generate invite link
func (a *AccessControl) CanGenerateInviteLink(ctx context.Context, userID, conversationID uuid.UUID) error {
	return a.CanManageGroup(ctx, userID, conversationID)
}

// ensureParticipant verifies user is a participant
func (a *AccessControl) ensureParticipant(ctx context.Context, conversationID, userID uuid.UUID) error {
	if a.conversationRepo == nil {
		return sentinal_errors.ErrForbidden
	}
	ok, err := a.conversationRepo.IsParticipant(ctx, conversationID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return sentinal_errors.ErrForbidden
	}
	return nil
}

// Authorize implements the commands.Proxy interface
func (a *AccessControl) Authorize(ctx context.Context, cmd commands.Command) error {
	requiresAuth, ok := cmd.(commands.RequiresAuth)
	if !ok {
		return nil
	}
	actorID := requiresAuth.ActorID()

	switch cmd.CommandType() {
	// Message commands
	case "message.send":
		c, ok := cmd.(commands.SendMessageCommand)
		if !ok {
			return sentinal_errors.ErrInvalidInput
		}
		return a.CanSendMessage(ctx, actorID, c.ConversationID)

	case "message.edit":
		c, ok := cmd.(commands.EditMessageCommand)
		if !ok {
			return sentinal_errors.ErrInvalidInput
		}
		return a.CanEditMessage(ctx, actorID, c.MessageID)

	case "message.delete":
		c, ok := cmd.(commands.DeleteMessageCommand)
		if !ok {
			return sentinal_errors.ErrInvalidInput
		}
		return a.CanDeleteMessage(ctx, actorID, c.MessageID, c.DeleteForEveryone)

	case "message.react", "message.remove_reaction":
		if c, ok := cmd.(commands.ReactToMessageCommand); ok {
			return a.CanReactToMessage(ctx, actorID, c.MessageID)
		}
		if c, ok := cmd.(commands.RemoveReactionCommand); ok {
			return a.CanReactToMessage(ctx, actorID, c.MessageID)
		}
		return sentinal_errors.ErrInvalidInput

	case "message.read", "message.delivered":
		if c, ok := cmd.(commands.MarkMessageReadCommand); ok {
			return a.CanViewConversation(ctx, actorID, c.ConversationID)
		}
		if c, ok := cmd.(commands.MarkMessageDeliveredCommand); ok {
			return a.CanViewConversation(ctx, actorID, c.ConversationID)
		}
		return sentinal_errors.ErrInvalidInput

	case "message.typing":
		c, ok := cmd.(commands.TypingCommand)
		if !ok {
			return sentinal_errors.ErrInvalidInput
		}
		return a.CanViewConversation(ctx, actorID, c.ConversationID)

	// Conversation commands
	case "conversation.create", "conversation.create_dm", "conversation.create_group":
		return nil // Anyone can create

	case "conversation.update_group":
		c, ok := cmd.(commands.UpdateGroupCommand)
		if !ok {
			return sentinal_errors.ErrInvalidInput
		}
		return a.CanManageGroup(ctx, actorID, c.ConversationID)

	case "conversation.add_member":
		c, ok := cmd.(commands.AddMemberCommand)
		if !ok {
			return sentinal_errors.ErrInvalidInput
		}
		return a.CanManageGroup(ctx, actorID, c.ConversationID)

	case "conversation.remove_member":
		c, ok := cmd.(commands.RemoveMemberCommand)
		if !ok {
			return sentinal_errors.ErrInvalidInput
		}
		return a.CanRemoveMember(ctx, actorID, c.ConversationID, c.MemberID)

	case "conversation.leave":
		c, ok := cmd.(commands.LeaveGroupCommand)
		if !ok {
			return sentinal_errors.ErrInvalidInput
		}
		return a.ensureParticipant(ctx, c.ConversationID, actorID)

	case "conversation.change_role":
		c, ok := cmd.(commands.ChangeRoleCommand)
		if !ok {
			return sentinal_errors.ErrInvalidInput
		}
		return a.CanChangeRole(ctx, actorID, c.ConversationID, c.NewRole)

	case "conversation.mute", "conversation.unmute", "conversation.archive", "conversation.unarchive", "conversation.pin", "conversation.unpin", "conversation.clear", "conversation.update_read_position":
		// User actions on their own participation
		switch c := cmd.(type) {
		case commands.MuteConversationCommand:
			return a.ensureParticipant(ctx, c.ConversationID, actorID)
		case commands.UnmuteConversationCommand:
			return a.ensureParticipant(ctx, c.ConversationID, actorID)
		case commands.ArchiveConversationCommand:
			return a.ensureParticipant(ctx, c.ConversationID, actorID)
		case commands.UnarchiveConversationCommand:
			return a.ensureParticipant(ctx, c.ConversationID, actorID)
		case commands.PinConversationCommand:
			return a.ensureParticipant(ctx, c.ConversationID, actorID)
		case commands.UnpinConversationCommand:
			return a.ensureParticipant(ctx, c.ConversationID, actorID)
		case commands.ClearConversationCommand:
			return a.ensureParticipant(ctx, c.ConversationID, actorID)
		case commands.UpdateReadPositionCommand:
			return a.ensureParticipant(ctx, c.ConversationID, actorID)
		}
		return sentinal_errors.ErrInvalidInput

	case "conversation.generate_invite_link", "conversation.revoke_invite_link":
		switch c := cmd.(type) {
		case commands.GenerateInviteLinkCommand:
			return a.CanGenerateInviteLink(ctx, actorID, c.ConversationID)
		case commands.RevokeInviteLinkCommand:
			return a.CanGenerateInviteLink(ctx, actorID, c.ConversationID)
		}
		return sentinal_errors.ErrInvalidInput

	// Call commands
	case "call.initiate":
		c, ok := cmd.(commands.InitiateCallCommand)
		if !ok {
			return sentinal_errors.ErrInvalidInput
		}
		return a.CanInitiateCall(ctx, actorID, c.ConversationID)

	case "call.accept", "call.reject", "call.join", "call.leave", "call.toggle_mute":
		switch c := cmd.(type) {
		case commands.AcceptCallCommand:
			return a.CanJoinCall(ctx, actorID, c.CallID)
		case commands.RejectCallCommand:
			return a.CanJoinCall(ctx, actorID, c.CallID)
		case commands.JoinCallCommand:
			return a.CanJoinCall(ctx, actorID, c.CallID)
		case commands.LeaveCallCommand:
			return a.CanManageCall(ctx, actorID, c.CallID)
		case commands.ToggleMuteCommand:
			return a.CanManageCall(ctx, actorID, c.CallID)
		}
		return sentinal_errors.ErrInvalidInput

	case "call.end":
		c, ok := cmd.(commands.EndCallCommand)
		if !ok {
			return sentinal_errors.ErrInvalidInput
		}
		return a.CanManageCall(ctx, actorID, c.CallID)

	case "call.offer", "call.answer", "call.ice":
		// Signaling: participant check
		switch c := cmd.(type) {
		case commands.SendOfferCommand:
			return a.CanJoinCall(ctx, actorID, c.CallID)
		case commands.SendAnswerCommand:
			return a.CanJoinCall(ctx, actorID, c.CallID)
		case commands.SendICECandidateCommand:
			return a.CanJoinCall(ctx, actorID, c.CallID)
		}
		return sentinal_errors.ErrInvalidInput

	// Broadcast commands
	case "broadcast.create":
		return nil // Anyone can create

	case "broadcast.update", "broadcast.delete", "broadcast.add_recipient", "broadcast.remove_recipient", "broadcast.send_message":
		switch c := cmd.(type) {
		case commands.UpdateBroadcastListCommand:
			return a.CanManageBroadcast(ctx, actorID, c.BroadcastID)
		case commands.DeleteBroadcastListCommand:
			return a.CanManageBroadcast(ctx, actorID, c.BroadcastID)
		case commands.AddBroadcastRecipientCommand:
			return a.CanManageBroadcast(ctx, actorID, c.BroadcastID)
		case commands.RemoveBroadcastRecipientCommand:
			return a.CanManageBroadcast(ctx, actorID, c.BroadcastID)
		case commands.SendBroadcastMessageCommand:
			return a.CanManageBroadcast(ctx, actorID, c.BroadcastID)
		}
		return sentinal_errors.ErrInvalidInput

	// Upload commands
	case "upload.create":
		return nil // Anyone can create

	case "upload.progress", "upload.complete", "upload.fail", "upload.delete":
		switch c := cmd.(type) {
		case commands.UpdateUploadProgressCommand:
			return a.CanAccessUpload(ctx, actorID, c.SessionID)
		case commands.CompleteUploadCommand:
			return a.CanAccessUpload(ctx, actorID, c.SessionID)
		case commands.FailUploadCommand:
			return a.CanAccessUpload(ctx, actorID, c.SessionID)
		case commands.DeleteUploadCommand:
			return a.CanAccessUpload(ctx, actorID, c.SessionID)
		}
		return sentinal_errors.ErrInvalidInput

	// Encryption commands - user can only manage their own keys
	case "encryption.register_identity_key", "encryption.upload_signed_prekey", "encryption.upload_onetime_prekeys", "encryption.rotate_signed_prekey", "encryption.create_session", "encryption.update_session", "encryption.upsert_key_bundle":
		return nil // Self-service

	case "encryption.consume_prekey":
		return nil // Any authenticated user can consume

	// User commands - self-service
	case "user.register":
		return nil

	case "user.update_profile", "user.update_settings", "user.add_contact", "user.remove_contact", "user.block", "user.unblock", "user.register_device", "user.update_presence":
		return nil // Self-service validated in service layer

	// Simple commands fallback
	default:
		if c, ok := cmd.(commands.SimpleCommand); ok {
			return a.authorizeSimpleCommand(ctx, actorID, c)
		}
		return sentinal_errors.ErrInvalidInput
	}

	return nil
}

// authorizeSimpleCommand handles SimpleCommand authorization
func (a *AccessControl) authorizeSimpleCommand(ctx context.Context, actorID uuid.UUID, c commands.SimpleCommand) error {
	payloadID, ok := parsePayloadUUID(c.Payload)
	if !ok {
		return sentinal_errors.ErrInvalidInput
	}

	switch c.Type {
	case "broadcast.update":
		return a.CanManageBroadcast(ctx, actorID, payloadID)
	case "upload.update":
		return a.CanAccessUpload(ctx, actorID, payloadID)
	}
	return nil
}

func parsePayloadUUID(payload any) (uuid.UUID, bool) {
	if id, ok := payload.(uuid.UUID); ok {
		return id, true
	}
	if raw, ok := payload.([]byte); ok {
		id, err := uuid.Parse(string(raw))
		if err != nil {
			return uuid.UUID{}, false
		}
		return id, true
	}
	return uuid.UUID{}, false
}
