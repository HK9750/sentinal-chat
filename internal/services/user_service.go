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
