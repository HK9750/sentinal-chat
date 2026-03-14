package services

import (
	"context"
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
