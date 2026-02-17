package handler

import (
	"net/http"
	"strconv"
	"time"

	"sentinal-chat/internal/domain/conversation"
	"sentinal-chat/internal/services"
	"sentinal-chat/internal/transport/httpdto"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ConversationHandler struct {
	service *services.ConversationService
}

func NewConversationHandler(service *services.ConversationService) *ConversationHandler {
	return &ConversationHandler{service: service}
}

func (h *ConversationHandler) Create(c *gin.Context) {
	var req httpdto.CreateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}

	creatorID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}

	participantIDs := make([]uuid.UUID, 0, len(req.Participants)+1)
	participantIDs = append(participantIDs, creatorID)
	for _, idStr := range req.Participants {
		id, err := uuid.Parse(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid participant id", "INVALID_REQUEST"))
			return
		}
		if id != creatorID {
			participantIDs = append(participantIDs, id)
		}
	}

	res, err := h.service.Create(c.Request.Context(), services.CreateConversationInput{
		Type:           req.Type,
		Subject:        req.Subject,
		Description:    req.Description,
		CreatorID:      creatorID,
		ParticipantIDs: participantIDs,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}

	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.FromConversation(res)))
}

func (h *ConversationHandler) List(c *gin.Context) {
	userID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}

	page, _ := strconv.Atoi(c.Query("page"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	items, total, err := h.service.GetUserConversations(c.Request.Context(), userID, page, limit)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}

	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.ListConversationsResponse{
		Conversations: httpdto.FromConversationSlice(items),
		Total:         total,
	}))
}

func (h *ConversationHandler) GetByID(c *gin.Context) {
	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid conversation id", "INVALID_REQUEST"))
		return
	}
	item, err := h.service.GetByID(c.Request.Context(), conversationID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.FromConversation(item)))
}

func (h *ConversationHandler) Update(c *gin.Context) {
	var req httpdto.UpdateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid conversation id", "INVALID_REQUEST"))
		return
	}

	// Get existing conversation
	existing, err := h.service.GetByID(c.Request.Context(), conversationID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}

	// Update fields
	if req.Subject != "" {
		existing.Subject.String = req.Subject
		existing.Subject.Valid = true
	}
	if req.Description != "" {
		existing.Description.String = req.Description
		existing.Description.Valid = true
	}
	if req.AvatarURL != "" {
		existing.AvatarURL.String = req.AvatarURL
		existing.AvatarURL.Valid = true
	}

	if err := h.service.Update(c.Request.Context(), existing); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.FromConversation(existing)))
}

func (h *ConversationHandler) Delete(c *gin.Context) {
	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid conversation id", "INVALID_REQUEST"))
		return
	}
	if err := h.service.Delete(c.Request.Context(), conversationID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *ConversationHandler) GetDirect(c *gin.Context) {
	userID1, err := uuid.Parse(c.Query("user_id_1"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id_1", "INVALID_REQUEST"))
		return
	}
	userID2, err := uuid.Parse(c.Query("user_id_2"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id_2", "INVALID_REQUEST"))
		return
	}
	item, err := h.service.GetDirectConversation(c.Request.Context(), userID1, userID2)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.FromConversation(item)))
}

func (h *ConversationHandler) Search(c *gin.Context) {
	userID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}
	query := c.Query("query")
	items, err := h.service.SearchConversations(c.Request.Context(), userID, query)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.ListConversationsResponse{
		Conversations: httpdto.FromConversationSlice(items),
	}))
}

func (h *ConversationHandler) GetByType(c *gin.Context) {
	userID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}
	convType := c.Query("type")
	items, err := h.service.GetConversationsByType(c.Request.Context(), userID, convType)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.ListConversationsResponse{
		Conversations: httpdto.FromConversationSlice(items),
	}))
}

func (h *ConversationHandler) GetByInviteLink(c *gin.Context) {
	link := c.Query("link")
	item, err := h.service.GetByInviteLink(c.Request.Context(), link)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.FromConversation(item)))
}

func (h *ConversationHandler) RegenerateInviteLink(c *gin.Context) {
	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid conversation id", "INVALID_REQUEST"))
		return
	}
	link, err := h.service.RegenerateInviteLink(c.Request.Context(), conversationID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.RegenerateInviteLinkResponse{InviteLink: link}))
}

func (h *ConversationHandler) AddParticipant(c *gin.Context) {
	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid conversation id", "INVALID_REQUEST"))
		return
	}
	var req httpdto.AddParticipantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id", "INVALID_REQUEST"))
		return
	}
	p := &conversation.Participant{
		ConversationID: conversationID,
		UserID:         userID,
		Role:           req.Role,
		JoinedAt:       time.Now(),
	}
	if err := h.service.AddParticipant(c.Request.Context(), p); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.FromParticipant(*p)))
}

func (h *ConversationHandler) RemoveParticipant(c *gin.Context) {
	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid conversation id", "INVALID_REQUEST"))
		return
	}
	userID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id", "INVALID_REQUEST"))
		return
	}
	if err := h.service.RemoveParticipant(c.Request.Context(), conversationID, userID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *ConversationHandler) ListParticipants(c *gin.Context) {
	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid conversation id", "INVALID_REQUEST"))
		return
	}
	items, err := h.service.GetParticipants(c.Request.Context(), conversationID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.ParticipantsResponse{
		Participants: httpdto.FromParticipantSlice(items),
	}))
}

func (h *ConversationHandler) UpdateParticipantRole(c *gin.Context) {
	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid conversation id", "INVALID_REQUEST"))
		return
	}
	userID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id", "INVALID_REQUEST"))
		return
	}
	var req httpdto.UpdateParticipantRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	if err := h.service.UpdateParticipantRole(c.Request.Context(), conversationID, userID, req.Role); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *ConversationHandler) Mute(c *gin.Context) {
	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid conversation id", "INVALID_REQUEST"))
		return
	}
	userID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}
	var req httpdto.MuteConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	until, err := time.Parse(time.RFC3339, req.Until)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid until", "INVALID_REQUEST"))
		return
	}
	if err := h.service.MuteConversation(c.Request.Context(), conversationID, userID, until); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *ConversationHandler) Unmute(c *gin.Context) {
	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid conversation id", "INVALID_REQUEST"))
		return
	}
	userID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}
	if err := h.service.UnmuteConversation(c.Request.Context(), conversationID, userID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *ConversationHandler) Pin(c *gin.Context) {
	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid conversation id", "INVALID_REQUEST"))
		return
	}
	userID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}
	if err := h.service.PinConversation(c.Request.Context(), conversationID, userID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *ConversationHandler) Unpin(c *gin.Context) {
	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid conversation id", "INVALID_REQUEST"))
		return
	}
	userID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}
	if err := h.service.UnpinConversation(c.Request.Context(), conversationID, userID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *ConversationHandler) Archive(c *gin.Context) {
	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid conversation id", "INVALID_REQUEST"))
		return
	}
	userID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}
	if err := h.service.ArchiveConversation(c.Request.Context(), conversationID, userID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *ConversationHandler) Unarchive(c *gin.Context) {
	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid conversation id", "INVALID_REQUEST"))
		return
	}
	userID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}
	if err := h.service.UnarchiveConversation(c.Request.Context(), conversationID, userID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *ConversationHandler) UpdateLastReadSequence(c *gin.Context) {
	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid conversation id", "INVALID_REQUEST"))
		return
	}
	userID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}
	var req httpdto.UpdateLastReadSequenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	if err := h.service.UpdateLastReadSequence(c.Request.Context(), conversationID, userID, req.SeqID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *ConversationHandler) GetSequence(c *gin.Context) {
	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid conversation id", "INVALID_REQUEST"))
		return
	}
	item, err := h.service.GetConversationSequence(c.Request.Context(), conversationID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.FromConversationSequence(item)))
}

func (h *ConversationHandler) IncrementSequence(c *gin.Context) {
	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid conversation id", "INVALID_REQUEST"))
		return
	}
	seq, err := h.service.IncrementSequence(c.Request.Context(), conversationID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.SequenceResponse{Sequence: seq}))
}
