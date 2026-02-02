package repository

import (
	"context"

	"github.com/google/uuid"

	"sentinal-chat/internal/domain/user"
)


type UserRepository interface {
	Create(ctx context.Context, u *user.User) error
	GetAllUsers(ctx context.Context, page, limit int) ([]user.User, int64, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (user.User, error)
	UpdateUser(ctx context.Context, u user.User) error
	DeleteUser(ctx context.Context, id uuid.UUID) error

	GetUserContacts(ctx context.Context, userID uuid.UUID) ([]user.UserContact, error)
	AddUserContact(ctx context.Context, c *user.UserContact) error
	RemoveUserContact(ctx context.Context, userID, contactUserID uuid.UUID) error

	GetUserSettings(ctx context.Context, userID uuid.UUID) (user.UserSettings, error)
	UpdateUserSettings(ctx context.Context, s user.UserSettings) error

	AddDevice(ctx context.Context, d *user.Device) error
	GetUserDevices(ctx context.Context, userID uuid.UUID) ([]user.Device, error)
	DeactivateDevice(ctx context.Context, deviceID uuid.UUID) error
}