package handler

import (
	"net/http"
	"strconv"

	"sentinal-chat/internal/commands"
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
		participantIDs = append(participantIDs, id)
	}

	res, err := h.service.Create(c.Request.Context(), commands.CreateConversationCommand{
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

	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(res))
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

	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(gin.H{"conversations": items, "total": total}))
}
