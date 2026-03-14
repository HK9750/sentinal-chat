package httpdto

type UserSearchQuery struct {
	Query string `form:"query" binding:"omitempty,max=255"`
	Page  int    `form:"page"`
	Limit int    `form:"limit"`
}

type AddContactRequest struct {
	ContactUserID string `json:"contact_user_id" binding:"required,uuid"`
	Nickname      string `json:"nickname,omitempty" binding:"omitempty,max=255"`
}

type RemoveContactPayload struct {
	Removed bool `json:"removed"`
}
