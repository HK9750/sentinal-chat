package services

import (
	"context"
	"time"

	"sentinal-chat/internal/commands"
	"sentinal-chat/internal/domain/user"
	"sentinal-chat/internal/repository"
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
)

type UserService struct {
	repo      repository.UserRepository
	eventRepo repository.EventRepository
	bus       *commands.Bus
}

func NewUserService(repo repository.UserRepository, eventRepo repository.EventRepository, bus *commands.Bus) *UserService {
	if bus == nil {
		bus = commands.NewBus()
	}
	return &UserService{repo: repo, eventRepo: eventRepo, bus: bus}
}

func (s *UserService) List(ctx context.Context, page, limit int, search string) ([]user.User, int64, error) {
	if search != "" {
		return s.repo.SearchUsers(ctx, search, page, limit)
	}
	return s.repo.GetAllUsers(ctx, page, limit)
}

func (s *UserService) GetByID(ctx context.Context, actorID, userID uuid.UUID) (user.User, error) {
	if actorID != userID {
		return user.User{}, sentinal_errors.ErrForbidden
	}
	return s.repo.GetUserByID(ctx, userID)
}

func (s *UserService) GetByEmail(ctx context.Context, email string) (user.User, error) {
	return s.repo.GetUserByEmail(ctx, email)
}

func (s *UserService) GetByUsername(ctx context.Context, username string) (user.User, error) {
	return s.repo.GetUserByUsername(ctx, username)
}

func (s *UserService) GetByPhoneNumber(ctx context.Context, phone string) (user.User, error) {
	return s.repo.GetUserByPhoneNumber(ctx, phone)
}

func (s *UserService) UpdateProfile(ctx context.Context, actorID uuid.UUID, input user.User) (user.User, error) {
	if actorID != input.ID {
		return user.User{}, sentinal_errors.ErrForbidden
	}
	input.UpdatedAt = time.Now()
	if err := s.repo.UpdateUser(ctx, input); err != nil {
		return user.User{}, err
	}
	_ = createOutboxEvent(ctx, s.eventRepo, "user", "user.updated", input.ID, input)
	return s.repo.GetUserByID(ctx, input.ID)
}

func (s *UserService) RegisterHandlers() {
	if s.bus == nil {
		return
	}

	// user.update - Update user profile (legacy handler)
	s.bus.Register("user.update", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.SimpleCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		payload, ok := c.Payload.(user.User)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		updated, err := s.UpdateProfile(ctx, payload.ID, payload)
		if err != nil {
			return commands.Result{}, err
		}
		return commands.Result{AggregateID: updated.ID.String(), Payload: updated}, nil
	}))

	// user.update_profile - Update user profile
	s.bus.Register("user.update_profile", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.UpdateProfileCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		existing, err := s.repo.GetUserByID(ctx, c.UserID)
		if err != nil {
			return commands.Result{}, err
		}
		if c.DisplayName != "" {
			existing.DisplayName = c.DisplayName
		}
		if c.Bio != "" {
			existing.Bio = c.Bio
		}
		if c.AvatarURL != "" {
			existing.AvatarURL = c.AvatarURL
		}
		existing.UpdatedAt = time.Now()
		if err := s.repo.UpdateUser(ctx, existing); err != nil {
			return commands.Result{}, err
		}
		_ = createOutboxEvent(ctx, s.eventRepo, "user", "user.updated", c.UserID, existing)
		return commands.Result{AggregateID: c.UserID.String(), Payload: existing}, nil
	}))

	// user.block - Block a user
	s.bus.Register("user.block", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.BlockUserCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		if err := s.repo.BlockContact(ctx, c.UserID, c.BlockedUserID); err != nil {
			return commands.Result{}, err
		}
		_ = createOutboxEvent(ctx, s.eventRepo, "user", "user.blocked", c.UserID, map[string]any{
			"user_id":         c.UserID,
			"blocked_user_id": c.BlockedUserID,
		})
		return commands.Result{AggregateID: c.UserID.String()}, nil
	}))

	// user.unblock - Unblock a user
	s.bus.Register("user.unblock", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.UnblockUserCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		if err := s.repo.UnblockContact(ctx, c.UserID, c.BlockedUserID); err != nil {
			return commands.Result{}, err
		}
		_ = createOutboxEvent(ctx, s.eventRepo, "user", "user.unblocked", c.UserID, map[string]any{
			"user_id":           c.UserID,
			"unblocked_user_id": c.BlockedUserID,
		})
		return commands.Result{AggregateID: c.UserID.String()}, nil
	}))

	// user.update_settings - Update user settings
	s.bus.Register("user.update_settings", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.UpdateSettingsCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		settings, err := s.repo.GetUserSettings(ctx, c.UserID)
		if err != nil {
			return commands.Result{}, err
		}
		if c.PrivacyLastSeen != "" {
			settings.PrivacyLastSeen = c.PrivacyLastSeen
		}
		if c.PrivacyProfilePhoto != "" {
			settings.PrivacyProfilePhoto = c.PrivacyProfilePhoto
		}
		if c.PrivacyAbout != "" {
			settings.PrivacyAbout = c.PrivacyAbout
		}
		if c.PrivacyGroups != "" {
			settings.PrivacyGroups = c.PrivacyGroups
		}
		if c.ReadReceipts != nil {
			settings.ReadReceipts = *c.ReadReceipts
		}
		if c.NotificationsEnabled != nil {
			settings.NotificationsEnabled = *c.NotificationsEnabled
		}
		if c.Theme != "" {
			settings.Theme = c.Theme
		}
		if c.Language != "" {
			settings.Language = c.Language
		}
		settings.UpdatedAt = time.Now()
		if err := s.repo.UpdateUserSettings(ctx, settings); err != nil {
			return commands.Result{}, err
		}
		_ = createOutboxEvent(ctx, s.eventRepo, "user", "user.settings_updated", c.UserID, settings)
		return commands.Result{AggregateID: c.UserID.String(), Payload: settings}, nil
	}))

	// user.add_contact - Add a contact
	s.bus.Register("user.add_contact", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.AddContactCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		contact := user.UserContact{
			UserID:        c.UserID,
			ContactUserID: c.ContactUserID,
			Nickname:      c.Nickname,
			CreatedAt:     time.Now(),
		}
		if err := s.repo.AddUserContact(ctx, &contact); err != nil {
			return commands.Result{}, err
		}
		_ = createOutboxEvent(ctx, s.eventRepo, "user", "user.contact_added", c.UserID, contact)
		return commands.Result{AggregateID: c.UserID.String(), Payload: contact}, nil
	}))

	// user.remove_contact - Remove a contact
	s.bus.Register("user.remove_contact", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.RemoveContactCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		if err := s.repo.RemoveUserContact(ctx, c.UserID, c.ContactUserID); err != nil {
			return commands.Result{}, err
		}
		_ = createOutboxEvent(ctx, s.eventRepo, "user", "user.contact_removed", c.UserID, map[string]any{
			"user_id":         c.UserID,
			"contact_user_id": c.ContactUserID,
		})
		return commands.Result{AggregateID: c.UserID.String()}, nil
	}))

	// user.register_device - Register a new device
	s.bus.Register("user.register_device", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.RegisterDeviceCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		device := user.Device{
			ID:           uuid.New(),
			UserID:       c.UserID,
			DeviceID:     c.DeviceID,
			DeviceName:   c.DeviceName,
			DeviceType:   c.DeviceType,
			IsActive:     true,
			RegisteredAt: time.Now(),
		}
		if err := s.repo.AddDevice(ctx, &device); err != nil {
			return commands.Result{}, err
		}
		_ = createOutboxEvent(ctx, s.eventRepo, "user", "user.device_registered", c.UserID, device)
		return commands.Result{AggregateID: device.ID.String(), Payload: device}, nil
	}))

	// user.update_presence - Update user presence
	s.bus.Register("user.update_presence", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.UpdatePresenceCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		if err := s.repo.UpdateOnlineStatus(ctx, c.UserID, c.IsOnline); err != nil {
			return commands.Result{}, err
		}
		eventType := "presence.online"
		if !c.IsOnline {
			eventType = "presence.offline"
		}
		_ = createOutboxEvent(ctx, s.eventRepo, "presence", eventType, c.UserID, map[string]any{
			"user_id":      c.UserID,
			"is_online":    c.IsOnline,
			"last_seen_at": time.Now(),
		})
		return commands.Result{AggregateID: c.UserID.String()}, nil
	}))
}

func (s *UserService) UpdateOnlineStatus(ctx context.Context, actorID, userID uuid.UUID, isOnline bool) error {
	if actorID != userID {
		return sentinal_errors.ErrForbidden
	}
	if err := s.repo.UpdateOnlineStatus(ctx, userID, isOnline); err != nil {
		return err
	}
	return createOutboxEvent(ctx, s.eventRepo, "user", "presence.updated", userID, map[string]any{"user_id": userID, "is_online": isOnline})
}

func (s *UserService) UpdateLastSeen(ctx context.Context, actorID, userID uuid.UUID, lastSeen time.Time) error {
	if actorID != userID {
		return sentinal_errors.ErrForbidden
	}
	return s.repo.UpdateLastSeen(ctx, userID, lastSeen)
}

func (s *UserService) Delete(ctx context.Context, actorID, userID uuid.UUID) error {
	if actorID != userID {
		return sentinal_errors.ErrForbidden
	}
	if err := s.repo.DeleteUser(ctx, userID); err != nil {
		return err
	}
	return createOutboxEvent(ctx, s.eventRepo, "user", "user.deleted", userID, map[string]any{"user_id": userID})
}

func (s *UserService) GetSettings(ctx context.Context, actorID, userID uuid.UUID) (user.UserSettings, error) {
	if actorID != userID {
		return user.UserSettings{}, sentinal_errors.ErrForbidden
	}
	return s.repo.GetUserSettings(ctx, userID)
}

func (s *UserService) UpdateSettings(ctx context.Context, actorID uuid.UUID, settings user.UserSettings) (user.UserSettings, error) {
	if actorID != settings.UserID {
		return user.UserSettings{}, sentinal_errors.ErrForbidden
	}
	if err := s.repo.UpdateUserSettings(ctx, settings); err != nil {
		return user.UserSettings{}, err
	}
	_ = createOutboxEvent(ctx, s.eventRepo, "user", "settings.updated", settings.UserID, settings)
	return s.repo.GetUserSettings(ctx, settings.UserID)
}

func (s *UserService) GetContacts(ctx context.Context, actorID, userID uuid.UUID) ([]user.UserContact, error) {
	if actorID != userID {
		return nil, sentinal_errors.ErrForbidden
	}
	return s.repo.GetUserContacts(ctx, userID)
}

func (s *UserService) AddContact(ctx context.Context, actorID uuid.UUID, contact user.UserContact) error {
	if actorID != contact.UserID {
		return sentinal_errors.ErrForbidden
	}
	contact.CreatedAt = time.Now()
	if err := s.repo.AddUserContact(ctx, &contact); err != nil {
		return err
	}
	return createOutboxEvent(ctx, s.eventRepo, "user", "contact.added", contact.UserID, contact)
}

func (s *UserService) RemoveContact(ctx context.Context, actorID, userID, contactUserID uuid.UUID) error {
	if actorID != userID {
		return sentinal_errors.ErrForbidden
	}
	if err := s.repo.RemoveUserContact(ctx, userID, contactUserID); err != nil {
		return err
	}
	return createOutboxEvent(ctx, s.eventRepo, "user", "contact.removed", userID, map[string]any{"user_id": userID, "contact_id": contactUserID})
}

func (s *UserService) BlockContact(ctx context.Context, actorID, userID, contactUserID uuid.UUID) error {
	if actorID != userID {
		return sentinal_errors.ErrForbidden
	}
	if err := s.repo.BlockContact(ctx, userID, contactUserID); err != nil {
		return err
	}
	return createOutboxEvent(ctx, s.eventRepo, "user", "contact.blocked", userID, map[string]any{"user_id": userID, "contact_id": contactUserID})
}

func (s *UserService) UnblockContact(ctx context.Context, actorID, userID, contactUserID uuid.UUID) error {
	if actorID != userID {
		return sentinal_errors.ErrForbidden
	}
	if err := s.repo.UnblockContact(ctx, userID, contactUserID); err != nil {
		return err
	}
	return createOutboxEvent(ctx, s.eventRepo, "user", "contact.unblocked", userID, map[string]any{"user_id": userID, "contact_id": contactUserID})
}

func (s *UserService) GetBlockedContacts(ctx context.Context, actorID, userID uuid.UUID) ([]user.UserContact, error) {
	if actorID != userID {
		return nil, sentinal_errors.ErrForbidden
	}
	return s.repo.GetBlockedContacts(ctx, userID)
}

func (s *UserService) GetDevices(ctx context.Context, actorID, userID uuid.UUID) ([]user.Device, error) {
	if actorID != userID {
		return nil, sentinal_errors.ErrForbidden
	}
	return s.repo.GetUserDevices(ctx, userID)
}

func (s *UserService) GetDeviceByID(ctx context.Context, actorID, userID, deviceID uuid.UUID) (user.Device, error) {
	if actorID != userID {
		return user.Device{}, sentinal_errors.ErrForbidden
	}
	return s.repo.GetDeviceByID(ctx, deviceID)
}

func (s *UserService) AddDevice(ctx context.Context, actorID uuid.UUID, device user.Device) error {
	if actorID != device.UserID {
		return sentinal_errors.ErrForbidden
	}
	device.RegisteredAt = time.Now()
	if err := s.repo.AddDevice(ctx, &device); err != nil {
		return err
	}
	return createOutboxEvent(ctx, s.eventRepo, "user", "device.added", device.UserID, device)
}

func (s *UserService) DeactivateDevice(ctx context.Context, actorID, userID, deviceID uuid.UUID) error {
	if actorID != userID {
		return sentinal_errors.ErrForbidden
	}
	if err := s.repo.DeactivateDevice(ctx, deviceID); err != nil {
		return err
	}
	return createOutboxEvent(ctx, s.eventRepo, "user", "device.deactivated", userID, map[string]any{"device_id": deviceID})
}

func (s *UserService) UpdateDeviceLastSeen(ctx context.Context, actorID, userID, deviceID uuid.UUID) error {
	if actorID != userID {
		return sentinal_errors.ErrForbidden
	}
	return s.repo.UpdateDeviceLastSeen(ctx, deviceID)
}

func (s *UserService) AddPushToken(ctx context.Context, actorID uuid.UUID, token user.PushToken) error {
	if actorID != token.UserID {
		return sentinal_errors.ErrForbidden
	}
	if err := s.repo.AddPushToken(ctx, &token); err != nil {
		return err
	}
	return createOutboxEvent(ctx, s.eventRepo, "user", "push_token.added", token.UserID, token)
}

func (s *UserService) GetPushTokens(ctx context.Context, actorID, userID uuid.UUID) ([]user.PushToken, error) {
	if actorID != userID {
		return nil, sentinal_errors.ErrForbidden
	}
	return s.repo.GetUserPushTokens(ctx, userID)
}

func (s *UserService) DeactivatePushToken(ctx context.Context, actorID, userID, tokenID uuid.UUID) error {
	if actorID != userID {
		return sentinal_errors.ErrForbidden
	}
	if err := s.repo.DeactivatePushToken(ctx, tokenID); err != nil {
		return err
	}
	return createOutboxEvent(ctx, s.eventRepo, "user", "push_token.deactivated", userID, map[string]any{"token_id": tokenID})
}

func (s *UserService) GetSessions(ctx context.Context, actorID, userID uuid.UUID) ([]user.UserSession, error) {
	if actorID != userID {
		return nil, sentinal_errors.ErrForbidden
	}
	return s.repo.GetUserSessions(ctx, userID)
}

func (s *UserService) GetSessionByID(ctx context.Context, actorID, userID, sessionID uuid.UUID) (user.UserSession, error) {
	if actorID != userID {
		return user.UserSession{}, sentinal_errors.ErrForbidden
	}
	return s.repo.GetSessionByID(ctx, sessionID)
}

func (s *UserService) CreateSession(ctx context.Context, actorID uuid.UUID, session *user.UserSession) error {
	if actorID != session.UserID {
		return sentinal_errors.ErrForbidden
	}
	return s.repo.CreateSession(ctx, session)
}

func (s *UserService) CleanExpiredSessions(ctx context.Context) error {
	return s.repo.CleanExpiredSessions(ctx)
}

func (s *UserService) CreateUserSettings(ctx context.Context, settings *user.UserSettings) error {
	return s.repo.CreateUserSettings(ctx, settings)
}

func (s *UserService) RevokeSession(ctx context.Context, actorID, userID, sessionID uuid.UUID) error {
	if actorID != userID {
		return sentinal_errors.ErrForbidden
	}
	return s.repo.RevokeSession(ctx, sessionID)
}

func (s *UserService) RevokeAllSessions(ctx context.Context, actorID, userID uuid.UUID) error {
	if actorID != userID {
		return sentinal_errors.ErrForbidden
	}
	return s.repo.RevokeAllUserSessions(ctx, userID)
}
