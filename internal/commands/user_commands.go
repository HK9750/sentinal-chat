package commands

import (
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
)

// RegisterUserCommand creates a new user
type RegisterUserCommand struct {
	PhoneNumber         string
	Email               string
	Username            string
	Password            string
	DisplayName         string
	IdempotencyKeyValue string
}

func (RegisterUserCommand) CommandType() string { return "user.register" }

func (c RegisterUserCommand) Validate() error {
	if c.DisplayName == "" || c.Password == "" {
		return sentinal_errors.ErrInvalidInput
	}
	if c.Email == "" && c.PhoneNumber == "" && c.Username == "" {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c RegisterUserCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

// UpdateProfileCommand updates user profile
type UpdateProfileCommand struct {
	UserID              uuid.UUID
	DisplayName         string
	Bio                 string
	AvatarURL           string
	IdempotencyKeyValue string
}

func (UpdateProfileCommand) CommandType() string { return "user.update_profile" }

func (c UpdateProfileCommand) Validate() error {
	if c.UserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c UpdateProfileCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c UpdateProfileCommand) ActorID() uuid.UUID { return c.UserID }

// BlockUserCommand blocks a user
type BlockUserCommand struct {
	UserID              uuid.UUID
	BlockedUserID       uuid.UUID
	IdempotencyKeyValue string
}

func (BlockUserCommand) CommandType() string { return "user.block" }

func (c BlockUserCommand) Validate() error {
	if c.UserID == uuid.Nil || c.BlockedUserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	if c.UserID == c.BlockedUserID {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c BlockUserCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c BlockUserCommand) ActorID() uuid.UUID { return c.UserID }

// UnblockUserCommand unblocks a user
type UnblockUserCommand struct {
	UserID              uuid.UUID
	BlockedUserID       uuid.UUID
	IdempotencyKeyValue string
}

func (UnblockUserCommand) CommandType() string { return "user.unblock" }

func (c UnblockUserCommand) Validate() error {
	if c.UserID == uuid.Nil || c.BlockedUserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c UnblockUserCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c UnblockUserCommand) ActorID() uuid.UUID { return c.UserID }

// UpdateSettingsCommand updates user settings
type UpdateSettingsCommand struct {
	UserID               uuid.UUID
	PrivacyLastSeen      string
	PrivacyProfilePhoto  string
	PrivacyAbout         string
	PrivacyGroups        string
	ReadReceipts         *bool
	NotificationsEnabled *bool
	Theme                string
	Language             string
	IdempotencyKeyValue  string
}

func (UpdateSettingsCommand) CommandType() string { return "user.update_settings" }

func (c UpdateSettingsCommand) Validate() error {
	if c.UserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c UpdateSettingsCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c UpdateSettingsCommand) ActorID() uuid.UUID { return c.UserID }

// AddContactCommand adds a contact
type AddContactCommand struct {
	UserID              uuid.UUID
	ContactUserID       uuid.UUID
	Nickname            string
	IdempotencyKeyValue string
}

func (AddContactCommand) CommandType() string { return "user.add_contact" }

func (c AddContactCommand) Validate() error {
	if c.UserID == uuid.Nil || c.ContactUserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c AddContactCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c AddContactCommand) ActorID() uuid.UUID { return c.UserID }

// RemoveContactCommand removes a contact
type RemoveContactCommand struct {
	UserID              uuid.UUID
	ContactUserID       uuid.UUID
	IdempotencyKeyValue string
}

func (RemoveContactCommand) CommandType() string { return "user.remove_contact" }

func (c RemoveContactCommand) Validate() error {
	if c.UserID == uuid.Nil || c.ContactUserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c RemoveContactCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c RemoveContactCommand) ActorID() uuid.UUID { return c.UserID }

// RegisterDeviceCommand registers a new device
type RegisterDeviceCommand struct {
	UserID              uuid.UUID
	DeviceID            string
	DeviceName          string
	DeviceType          string
	IdempotencyKeyValue string
}

func (RegisterDeviceCommand) CommandType() string { return "user.register_device" }

func (c RegisterDeviceCommand) Validate() error {
	if c.UserID == uuid.Nil || c.DeviceID == "" {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c RegisterDeviceCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c RegisterDeviceCommand) ActorID() uuid.UUID { return c.UserID }

// UpdatePresenceCommand updates user presence
type UpdatePresenceCommand struct {
	UserID   uuid.UUID
	IsOnline bool
}

func (UpdatePresenceCommand) CommandType() string { return "user.update_presence" }

func (c UpdatePresenceCommand) Validate() error {
	if c.UserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c UpdatePresenceCommand) IdempotencyKey() string { return "" }

func (c UpdatePresenceCommand) ActorID() uuid.UUID { return c.UserID }
