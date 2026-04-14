package httpdto

type UserSearchQuery struct {
	Query string `form:"query" binding:"omitempty,max=255"`
	Page  int    `form:"page"`
	Limit int    `form:"limit"`
}

type UpdateProfileRequest struct {
	DisplayName *string `json:"display_name,omitempty" binding:"omitempty,max=255"`
	Email       *string `json:"email,omitempty" binding:"omitempty,email,max=255"`
	PhoneNumber *string `json:"phone_number,omitempty" binding:"omitempty,max=32"`
	AvatarURL   *string `json:"avatar_url,omitempty" binding:"omitempty,url,max=2048"`
}

type UserProfilePayload struct {
	ID          string  `json:"id"`
	DisplayName string  `json:"display_name"`
	Email       *string `json:"email,omitempty"`
	Username    *string `json:"username,omitempty"`
	PhoneNumber *string `json:"phone_number,omitempty"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
	IsVerified  bool    `json:"is_verified"`
}

type AddContactRequest struct {
	ContactUserID string `json:"contact_user_id" binding:"required,uuid"`
	Nickname      string `json:"nickname,omitempty" binding:"omitempty,max=255"`
}

type RemoveContactPayload struct {
	Removed bool `json:"removed"`
}
