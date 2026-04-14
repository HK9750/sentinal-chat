package services

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"sentinal-chat/internal/domain/user"
	"sentinal-chat/internal/repository"
	sentinal_errors "sentinal-chat/pkg/errors"
)

type UserSearchView struct {
	ID          string  `json:"id"`
	DisplayName string  `json:"display_name"`
	Username    *string `json:"username,omitempty"`
	Email       *string `json:"email,omitempty"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
	IsOnline    bool    `json:"is_online"`
	IsContact   bool    `json:"is_contact"`
	IsBlocked   bool    `json:"is_blocked"`
	Nickname    *string `json:"nickname,omitempty"`
}

type ContactView struct {
	ID          string     `json:"id"`
	DisplayName string     `json:"display_name"`
	Username    *string    `json:"username,omitempty"`
	Email       *string    `json:"email,omitempty"`
	AvatarURL   *string    `json:"avatar_url,omitempty"`
	IsOnline    bool       `json:"is_online"`
	IsBlocked   bool       `json:"is_blocked"`
	Nickname    *string    `json:"nickname,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	LastSeenAt  *time.Time `json:"last_seen_at,omitempty"`
}

type AddContactInput struct {
	UserID        uuid.UUID
	ContactUserID uuid.UUID
	Nickname      string
}

type UserProfileView struct {
	ID          string  `json:"id"`
	DisplayName string  `json:"display_name"`
	Email       *string `json:"email,omitempty"`
	Username    *string `json:"username,omitempty"`
	PhoneNumber *string `json:"phone_number,omitempty"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
	IsVerified  bool    `json:"is_verified"`
}

type UpdateProfileInput struct {
	UserID      uuid.UUID
	DisplayName *string
	Email       *string
	PhoneNumber *string
	AvatarURL   *string
}

type UserService struct {
	users repository.UserRepository
}

func NewUserService(users repository.UserRepository) *UserService {
	return &UserService{users: users}
}

func (s *UserService) SearchUsers(ctx context.Context, userID uuid.UUID, query string, page, limit int) ([]UserSearchView, int64, error) {
	if s == nil || s.users == nil {
		return nil, 0, sentinal_errors.ErrServiceUnavailable
	}
	if userID == uuid.Nil {
		return nil, 0, sentinal_errors.ErrUnauthorized
	}

	query = strings.TrimSpace(query)
	page = normalizeListPage(page)
	limit = normalizeListLimit(limit, 15)

	users, _, err := s.users.SearchUsers(ctx, query, page, limit)
	if err != nil {
		return nil, 0, err
	}

	contactMap, err := s.contactLookup(ctx, userID)
	if err != nil {
		return nil, 0, err
	}

	items := make([]UserSearchView, 0, len(users))
	for _, item := range users {
		if item.ID == userID {
			continue
		}
		items = append(items, toUserSearchView(item, contactMap[item.ID]))
	}

	return items, int64(len(items)), nil
}

func (s *UserService) ListContacts(ctx context.Context, userID uuid.UUID) ([]ContactView, error) {
	if s == nil || s.users == nil {
		return nil, sentinal_errors.ErrServiceUnavailable
	}
	if userID == uuid.Nil {
		return nil, sentinal_errors.ErrUnauthorized
	}

	relations, err := s.users.GetUserContacts(ctx, userID)
	if err != nil {
		return nil, err
	}

	items := make([]ContactView, 0, len(relations))
	for _, relation := range relations {
		contactUser, getErr := s.users.GetUserByID(ctx, relation.ContactUserID)
		if getErr != nil {
			if getErr == sentinal_errors.ErrNotFound {
				continue
			}
			return nil, getErr
		}
		items = append(items, toContactView(contactUser, relation))
	}

	return items, nil
}

func (s *UserService) AddContact(ctx context.Context, input AddContactInput) (ContactView, error) {
	if s == nil || s.users == nil {
		return ContactView{}, sentinal_errors.ErrServiceUnavailable
	}
	if input.UserID == uuid.Nil || input.ContactUserID == uuid.Nil || input.UserID == input.ContactUserID {
		return ContactView{}, sentinal_errors.ErrInvalidInput
	}

	contactUser, err := s.users.GetUserByID(ctx, input.ContactUserID)
	if err != nil {
		return ContactView{}, err
	}

	relation := user.UserContact{
		UserID:        input.UserID,
		ContactUserID: input.ContactUserID,
		Nickname:      strings.TrimSpace(input.Nickname),
		IsBlocked:     false,
		CreatedAt:     time.Now().UTC(),
	}

	err = s.users.AddUserContact(ctx, &relation)
	if err != nil && err != sentinal_errors.ErrAlreadyExists {
		return ContactView{}, err
	}

	resolved, resolveErr := s.resolveContactRelation(ctx, input.UserID, input.ContactUserID)
	if resolveErr == nil {
		relation = resolved
	}

	return toContactView(contactUser, relation), nil
}

func (s *UserService) RemoveContact(ctx context.Context, userID, contactUserID uuid.UUID) error {
	if s == nil || s.users == nil {
		return sentinal_errors.ErrServiceUnavailable
	}
	if userID == uuid.Nil || contactUserID == uuid.Nil || userID == contactUserID {
		return sentinal_errors.ErrInvalidInput
	}

	return s.users.RemoveUserContact(ctx, userID, contactUserID)
}

func (s *UserService) GetProfile(ctx context.Context, userID uuid.UUID) (UserProfileView, error) {
	if s == nil || s.users == nil {
		return UserProfileView{}, sentinal_errors.ErrServiceUnavailable
	}
	if userID == uuid.Nil {
		return UserProfileView{}, sentinal_errors.ErrUnauthorized
	}

	u, err := s.users.GetUserByID(ctx, userID)
	if err != nil {
		return UserProfileView{}, err
	}

	return toUserProfileView(u), nil
}

func (s *UserService) UpdateProfile(ctx context.Context, input UpdateProfileInput) (UserProfileView, error) {
	if s == nil || s.users == nil {
		return UserProfileView{}, sentinal_errors.ErrServiceUnavailable
	}
	if input.UserID == uuid.Nil {
		return UserProfileView{}, sentinal_errors.ErrUnauthorized
	}

	u, err := s.users.GetUserByID(ctx, input.UserID)
	if err != nil {
		return UserProfileView{}, err
	}

	if input.DisplayName == nil && input.Email == nil && input.PhoneNumber == nil && input.AvatarURL == nil {
		return UserProfileView{}, sentinal_errors.ErrInvalidInput
	}

	if input.DisplayName != nil {
		displayName := strings.TrimSpace(*input.DisplayName)
		if displayName == "" {
			return UserProfileView{}, sentinal_errors.ErrInvalidInput
		}
		u.DisplayName = displayName
	}

	if input.Email != nil {
		normalizedEmail := strings.ToLower(strings.TrimSpace(*input.Email))
		if normalizedEmail == "" {
			u.Email = sql.NullString{}
		} else {
			existing, findErr := s.users.GetUserByEmail(ctx, normalizedEmail)
			if findErr == nil && existing.ID != u.ID {
				return UserProfileView{}, sentinal_errors.ErrConflict
			}
			if findErr != nil && !errors.Is(findErr, sentinal_errors.ErrNotFound) {
				return UserProfileView{}, findErr
			}
			u.Email = toNullString(normalizedEmail)
		}
	}

	if input.PhoneNumber != nil {
		normalizedPhone := strings.TrimSpace(*input.PhoneNumber)
		if normalizedPhone == "" {
			u.PhoneNumber = sql.NullString{}
		} else {
			existing, findErr := s.users.GetUserByPhoneNumber(ctx, normalizedPhone)
			if findErr == nil && existing.ID != u.ID {
				return UserProfileView{}, sentinal_errors.ErrConflict
			}
			if findErr != nil && !errors.Is(findErr, sentinal_errors.ErrNotFound) {
				return UserProfileView{}, findErr
			}
			u.PhoneNumber = toNullString(normalizedPhone)
		}
	}

	if input.AvatarURL != nil {
		avatarURL := strings.TrimSpace(*input.AvatarURL)
		if avatarURL == "" {
			u.AvatarURL = ""
		} else {
			if !strings.HasPrefix(avatarURL, "http://") && !strings.HasPrefix(avatarURL, "https://") {
				return UserProfileView{}, sentinal_errors.ErrInvalidInput
			}
			u.AvatarURL = avatarURL
		}
	}

	u.UpdatedAt = time.Now().UTC()
	if err := s.users.UpdateUser(ctx, u); err != nil {
		if errors.Is(err, sentinal_errors.ErrNotFound) {
			return UserProfileView{}, err
		}
		if errors.Is(err, sentinal_errors.ErrAlreadyExists) {
			return UserProfileView{}, sentinal_errors.ErrConflict
		}
		if strings.Contains(strings.ToLower(err.Error()), "duplicate key") {
			return UserProfileView{}, sentinal_errors.ErrConflict
		}
		return UserProfileView{}, err
	}

	refreshed, err := s.users.GetUserByID(ctx, u.ID)
	if err != nil {
		return UserProfileView{}, err
	}

	return toUserProfileView(refreshed), nil
}

func (s *UserService) contactLookup(ctx context.Context, userID uuid.UUID) (map[uuid.UUID]user.UserContact, error) {
	relations, err := s.users.GetUserContacts(ctx, userID)
	if err != nil {
		return nil, err
	}

	lookup := make(map[uuid.UUID]user.UserContact, len(relations))
	for _, relation := range relations {
		lookup[relation.ContactUserID] = relation
	}

	return lookup, nil
}

func (s *UserService) resolveContactRelation(ctx context.Context, userID, contactUserID uuid.UUID) (user.UserContact, error) {
	relations, err := s.users.GetUserContacts(ctx, userID)
	if err != nil {
		return user.UserContact{}, err
	}

	for _, relation := range relations {
		if relation.ContactUserID == contactUserID {
			return relation, nil
		}
	}

	return user.UserContact{}, sentinal_errors.ErrNotFound
}

func toUserSearchView(item user.User, relation user.UserContact) UserSearchView {
	return UserSearchView{
		ID:          item.ID.String(),
		DisplayName: strings.TrimSpace(item.DisplayName),
		Username:    nullStringPtr(item.Username),
		Email:       nullStringPtr(item.Email),
		AvatarURL:   optionalStringPtr(item.AvatarURL),
		IsOnline:    item.IsOnline,
		IsContact:   relation.ContactUserID != uuid.Nil,
		IsBlocked:   relation.IsBlocked,
		Nickname:    optionalStringPtr(relation.Nickname),
	}
}

func toContactView(item user.User, relation user.UserContact) ContactView {
	var lastSeenAt *time.Time
	if item.LastSeenAt.Valid {
		lastSeen := item.LastSeenAt.Time
		lastSeenAt = &lastSeen
	}

	return ContactView{
		ID:          item.ID.String(),
		DisplayName: strings.TrimSpace(item.DisplayName),
		Username:    nullStringPtr(item.Username),
		Email:       nullStringPtr(item.Email),
		AvatarURL:   optionalStringPtr(item.AvatarURL),
		IsOnline:    item.IsOnline,
		IsBlocked:   relation.IsBlocked,
		Nickname:    optionalStringPtr(relation.Nickname),
		CreatedAt:   relation.CreatedAt,
		LastSeenAt:  lastSeenAt,
	}
}

func toUserProfileView(item user.User) UserProfileView {
	return UserProfileView{
		ID:          item.ID.String(),
		DisplayName: strings.TrimSpace(item.DisplayName),
		Email:       nullStringPtr(item.Email),
		Username:    nullStringPtr(item.Username),
		PhoneNumber: nullStringPtr(item.PhoneNumber),
		AvatarURL:   optionalStringPtr(item.AvatarURL),
		IsVerified:  item.IsVerified,
	}
}

func normalizeListPage(page int) int {
	if page <= 0 {
		return 1
	}
	return page
}

func normalizeListLimit(limit, fallback int) int {
	if limit <= 0 {
		return fallback
	}
	if limit > 50 {
		return 50
	}
	return limit
}
