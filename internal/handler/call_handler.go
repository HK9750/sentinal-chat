package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"sentinal-chat/internal/domain/call"
	"sentinal-chat/internal/repository"
	"sentinal-chat/internal/services"
	"sentinal-chat/internal/transport/httpdto"
	"sentinal-chat/pkg/database"
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type CallHandler struct {
	service *services.CallService
}

func NewCallHandler(service *services.CallService) *CallHandler {
	return &CallHandler{service: service}
}

func (h *CallHandler) Create(c *gin.Context) {
	var req call.Call
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	if req.Topology != "" && req.Topology != "P2P" {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("unsupported call topology", "INVALID_REQUEST"))
		return
	}
	if req.Type != "AUDIO" && req.Type != "VIDEO" {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("unsupported call type", "INVALID_REQUEST"))
		return
	}
	req.Topology = "P2P"
	req.IsGroupCall = false
	if err := h.ensureDMCall(c.Request.Context(), req.ConversationID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	if err := h.service.Create(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(req))
}

func (h *CallHandler) GetByID(c *gin.Context) {
	callID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid call id", "INVALID_REQUEST"))
		return
	}
	item, err := h.service.GetByID(c.Request.Context(), callID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(item))
}

func (h *CallHandler) ListByConversation(c *gin.Context) {
	conversationID, err := uuid.Parse(c.Query("conversation_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid conversation_id", "INVALID_REQUEST"))
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	items, total, err := h.service.GetConversationCalls(c.Request.Context(), conversationID, page, limit)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(gin.H{"calls": items, "total": total}))
}

func (h *CallHandler) ListByUser(c *gin.Context) {
	userID, err := uuid.Parse(c.Query("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id", "INVALID_REQUEST"))
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	items, total, err := h.service.GetUserCalls(c.Request.Context(), userID, page, limit)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(gin.H{"calls": items, "total": total}))
}

func (h *CallHandler) ActiveCalls(c *gin.Context) {
	userID, err := uuid.Parse(c.Query("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id", "INVALID_REQUEST"))
		return
	}
	items, err := h.service.GetActiveCalls(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(gin.H{"calls": items}))
}

func (h *CallHandler) MissedCalls(c *gin.Context) {
	userID, err := uuid.Parse(c.Query("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id", "INVALID_REQUEST"))
		return
	}
	since, _ := time.Parse(time.RFC3339, c.Query("since"))
	items, err := h.service.GetMissedCalls(c.Request.Context(), userID, since)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(gin.H{"calls": items}))
}

func (h *CallHandler) AddParticipant(c *gin.Context) {
	callID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid call id", "INVALID_REQUEST"))
		return
	}
	var req call.CallParticipant
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	req.CallID = callID
	callItem, err := h.service.GetByID(c.Request.Context(), callID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	if err := h.ensureDMCall(c.Request.Context(), callItem.ConversationID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	activeCount, err := h.service.GetActiveParticipantCount(c.Request.Context(), callID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	if activeCount >= 2 {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("dm calls are limited to two participants", "REQUEST_FAILED"))
		return
	}
	if err := h.service.AddParticipant(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(req))
}

func (h *CallHandler) RemoveParticipant(c *gin.Context) {
	callID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid call id", "INVALID_REQUEST"))
		return
	}
	userID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id", "INVALID_REQUEST"))
		return
	}
	callItem, err := h.service.GetByID(c.Request.Context(), callID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	if err := h.ensureDMCall(c.Request.Context(), callItem.ConversationID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	if err := h.service.RemoveParticipant(c.Request.Context(), callID, userID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *CallHandler) ListParticipants(c *gin.Context) {
	callID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid call id", "INVALID_REQUEST"))
		return
	}
	items, err := h.service.GetCallParticipants(c.Request.Context(), callID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(gin.H{"participants": items}))
}

func (h *CallHandler) UpdateParticipantStatus(c *gin.Context) {
	callID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid call id", "INVALID_REQUEST"))
		return
	}
	userID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id", "INVALID_REQUEST"))
		return
	}
	var req httpdto.UpdateParticipantStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	callItem, err := h.service.GetByID(c.Request.Context(), callID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	if err := h.ensureDMCall(c.Request.Context(), callItem.ConversationID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	if err := h.service.UpdateParticipantStatus(c.Request.Context(), callID, userID, req.Status); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *CallHandler) UpdateParticipantMute(c *gin.Context) {
	callID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid call id", "INVALID_REQUEST"))
		return
	}
	userID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id", "INVALID_REQUEST"))
		return
	}
	var req httpdto.UpdateParticipantMuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	callItem, err := h.service.GetByID(c.Request.Context(), callID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	if err := h.ensureDMCall(c.Request.Context(), callItem.ConversationID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	if err := h.service.UpdateParticipantMuteStatus(c.Request.Context(), callID, userID, req.AudioMuted, req.VideoMuted); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *CallHandler) RecordQualityMetric(c *gin.Context) {
	var req call.CallQualityMetric
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	callItem, err := h.service.GetByID(c.Request.Context(), req.CallID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	if err := h.ensureDMCall(c.Request.Context(), callItem.ConversationID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	if err := h.service.RecordQualityMetric(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(req))
}

func (h *CallHandler) MarkConnected(c *gin.Context) {
	callID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid call id", "INVALID_REQUEST"))
		return
	}
	callItem, err := h.service.GetByID(c.Request.Context(), callID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	if err := h.ensureDMCall(c.Request.Context(), callItem.ConversationID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	if err := h.service.MarkConnected(c.Request.Context(), callID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *CallHandler) EndCall(c *gin.Context) {
	callID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid call id", "INVALID_REQUEST"))
		return
	}
	var req httpdto.EndCallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	callItem, err := h.service.GetByID(c.Request.Context(), callID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	if err := h.ensureDMCall(c.Request.Context(), callItem.ConversationID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	if err := h.service.EndCall(c.Request.Context(), callID, req.Reason); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *CallHandler) GetCallDuration(c *gin.Context) {
	callID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid call id", "INVALID_REQUEST"))
		return
	}
	duration, err := h.service.GetCallDuration(c.Request.Context(), callID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(gin.H{"duration": duration}))
}

func (h *CallHandler) GetCallQualityMetrics(c *gin.Context) {
	callID, err := uuid.Parse(c.Query("call_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid call_id", "INVALID_REQUEST"))
		return
	}
	callItem, err := h.service.GetByID(c.Request.Context(), callID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	if err := h.ensureDMCall(c.Request.Context(), callItem.ConversationID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	items, err := h.service.GetCallQualityMetrics(c.Request.Context(), callID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(gin.H{"metrics": items}))
}

func (h *CallHandler) GetUserCallQualityMetrics(c *gin.Context) {
	callID, err := uuid.Parse(c.Query("call_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid call_id", "INVALID_REQUEST"))
		return
	}
	userID, err := uuid.Parse(c.Query("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id", "INVALID_REQUEST"))
		return
	}
	callItem, err := h.service.GetByID(c.Request.Context(), callID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	if err := h.ensureDMCall(c.Request.Context(), callItem.ConversationID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	items, err := h.service.GetUserCallQualityMetrics(c.Request.Context(), callID, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(gin.H{"metrics": items}))
}

func (h *CallHandler) GetAverageCallQuality(c *gin.Context) {
	callID, err := uuid.Parse(c.Query("call_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid call_id", "INVALID_REQUEST"))
		return
	}
	callItem, err := h.service.GetByID(c.Request.Context(), callID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	if err := h.ensureDMCall(c.Request.Context(), callItem.ConversationID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	avg, err := h.service.GetAverageCallQuality(c.Request.Context(), callID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(gin.H{"average": avg}))
}

func (h *CallHandler) ensureDMCall(ctx context.Context, conversationID uuid.UUID) error {
	if conversationID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	conv := repository.NewConversationRepository(database.GetDB())
	item, err := conv.GetByID(ctx, conversationID)
	if err != nil {
		return err
	}
	if item.Type != "DM" {
		return sentinal_errors.ErrForbidden
	}
	return nil
}
