package handler

import (
	"net/http"
	"strconv"
	"time"

	"sentinal-chat/internal/domain/call"
	"sentinal-chat/internal/services"
	"sentinal-chat/internal/transport/httpdto"

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
	avg, err := h.service.GetAverageCallQuality(c.Request.Context(), callID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(gin.H{"average": avg}))
}

func (h *CallHandler) CreateTurnCredential(c *gin.Context) {
	var req call.TurnCredential
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	if err := h.service.CreateTurnCredential(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(req))
}

func (h *CallHandler) GetActiveTurnCredentials(c *gin.Context) {
	userID, err := uuid.Parse(c.Query("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id", "INVALID_REQUEST"))
		return
	}
	items, err := h.service.GetActiveTurnCredentials(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(gin.H{"credentials": items}))
}

func (h *CallHandler) DeleteExpiredTurnCredentials(c *gin.Context) {
	count, err := h.service.DeleteExpiredTurnCredentials(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(gin.H{"deleted": count}))
}

func (h *CallHandler) CreateSFUServer(c *gin.Context) {
	var req call.SFUServer
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	if err := h.service.CreateSFUServer(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(req))
}

func (h *CallHandler) GetSFUServerByID(c *gin.Context) {
	serverID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid server id", "INVALID_REQUEST"))
		return
	}
	item, err := h.service.GetSFUServerByID(c.Request.Context(), serverID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(item))
}

func (h *CallHandler) GetHealthySFUServers(c *gin.Context) {
	region := c.Query("region")
	items, err := h.service.GetHealthySFUServers(c.Request.Context(), region)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(gin.H{"servers": items}))
}

func (h *CallHandler) GetLeastLoadedServer(c *gin.Context) {
	region := c.Query("region")
	item, err := h.service.GetLeastLoadedServer(c.Request.Context(), region)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(item))
}

func (h *CallHandler) UpdateServerLoad(c *gin.Context) {
	serverID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid server id", "INVALID_REQUEST"))
		return
	}
	var req httpdto.UpdateServerLoadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	if err := h.service.UpdateServerLoad(c.Request.Context(), serverID, req.Load); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *CallHandler) UpdateServerHealth(c *gin.Context) {
	serverID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid server id", "INVALID_REQUEST"))
		return
	}
	var req httpdto.UpdateServerHealthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	if err := h.service.UpdateServerHealth(c.Request.Context(), serverID, req.IsHealthy); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *CallHandler) UpdateServerHeartbeat(c *gin.Context) {
	serverID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid server id", "INVALID_REQUEST"))
		return
	}
	if err := h.service.UpdateServerHeartbeat(c.Request.Context(), serverID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *CallHandler) AssignCallToServer(c *gin.Context) {
	var req call.CallServerAssignment
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	if err := h.service.AssignCallToServer(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(req))
}

func (h *CallHandler) GetCallServerAssignments(c *gin.Context) {
	callID, err := uuid.Parse(c.Query("call_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid call_id", "INVALID_REQUEST"))
		return
	}
	items, err := h.service.GetCallServerAssignments(c.Request.Context(), callID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(gin.H{"assignments": items}))
}

func (h *CallHandler) RemoveCallServerAssignment(c *gin.Context) {
	callID, err := uuid.Parse(c.Query("call_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid call_id", "INVALID_REQUEST"))
		return
	}
	serverID, err := uuid.Parse(c.Query("server_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid server_id", "INVALID_REQUEST"))
		return
	}
	if err := h.service.RemoveCallServerAssignment(c.Request.Context(), callID, serverID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}
