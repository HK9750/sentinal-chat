package handler

import (
	"net/http"

	"sentinal-chat/internal/domain/broadcast"
	"sentinal-chat/internal/services"
	"sentinal-chat/internal/transport/httpdto"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type BroadcastHandler struct {
	service *services.BroadcastService
}

func NewBroadcastHandler(service *services.BroadcastService) *BroadcastHandler {
	return &BroadcastHandler{service: service}
}

func (h *BroadcastHandler) Create(c *gin.Context) {
	var req httpdto.CreateBroadcastRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	ownerID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}

	item := &broadcast.BroadcastList{
		OwnerID: ownerID,
		Name:    req.Name,
	}
	if req.Description != "" {
		item.Description.String = req.Description
		item.Description.Valid = true
	}
	if err := h.service.Create(c.Request.Context(), item); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.FromBroadcastList(*item)))
}

func (h *BroadcastHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid broadcast id", "INVALID_REQUEST"))
		return
	}
	item, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.FromBroadcastList(item)))
}

func (h *BroadcastHandler) Update(c *gin.Context) {
	var req httpdto.UpdateBroadcastRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid broadcast id", "INVALID_REQUEST"))
		return
	}
	item, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	if req.Name != "" {
		item.Name = req.Name
	}
	if req.Description != "" {
		item.Description.String = req.Description
		item.Description.Valid = true
	}
	if err := h.service.Update(c.Request.Context(), item); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.FromBroadcastList(item)))
}

func (h *BroadcastHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid broadcast id", "INVALID_REQUEST"))
		return
	}
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *BroadcastHandler) ListByOwner(c *gin.Context) {
	ownerID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}
	items, err := h.service.GetUserBroadcastLists(c.Request.Context(), ownerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.ListBroadcastsResponse{
		Broadcasts: httpdto.FromBroadcastListSlice(items),
	}))
}

func (h *BroadcastHandler) Search(c *gin.Context) {
	ownerID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}
	query := c.Query("query")
	items, err := h.service.SearchBroadcastLists(c.Request.Context(), ownerID, query)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.ListBroadcastsResponse{
		Broadcasts: httpdto.FromBroadcastListSlice(items),
	}))
}

func (h *BroadcastHandler) AddRecipient(c *gin.Context) {
	var req httpdto.AddRecipientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	broadcastID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid broadcast id", "INVALID_REQUEST"))
		return
	}
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id", "INVALID_REQUEST"))
		return
	}
	item := &broadcast.BroadcastRecipient{
		BroadcastID: broadcastID,
		UserID:      userID,
	}
	if err := h.service.AddRecipient(c.Request.Context(), item); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.FromBroadcastRecipient(*item)))
}

func (h *BroadcastHandler) RemoveRecipient(c *gin.Context) {
	broadcastID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid broadcast id", "INVALID_REQUEST"))
		return
	}
	userID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id", "INVALID_REQUEST"))
		return
	}
	if err := h.service.RemoveRecipient(c.Request.Context(), broadcastID, userID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *BroadcastHandler) ListRecipients(c *gin.Context) {
	broadcastID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid broadcast id", "INVALID_REQUEST"))
		return
	}
	items, err := h.service.GetRecipients(c.Request.Context(), broadcastID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.RecipientsResponse{
		Recipients: httpdto.FromBroadcastRecipientSlice(items),
	}))
}

func (h *BroadcastHandler) RecipientCount(c *gin.Context) {
	broadcastID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid broadcast id", "INVALID_REQUEST"))
		return
	}
	count, err := h.service.GetRecipientCount(c.Request.Context(), broadcastID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.RecipientCountResponse{Count: count}))
}

func (h *BroadcastHandler) IsRecipient(c *gin.Context) {
	broadcastID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid broadcast id", "INVALID_REQUEST"))
		return
	}
	userID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id", "INVALID_REQUEST"))
		return
	}
	ok, err := h.service.IsRecipient(c.Request.Context(), broadcastID, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.IsRecipientResponse{IsRecipient: ok}))
}

func (h *BroadcastHandler) BulkAddRecipients(c *gin.Context) {
	broadcastID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid broadcast id", "INVALID_REQUEST"))
		return
	}
	var req httpdto.BulkRecipientsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	ids := make([]uuid.UUID, 0, len(req.UserIDs))
	for _, value := range req.UserIDs {
		id, err := uuid.Parse(value)
		if err != nil {
			c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id", "INVALID_REQUEST"))
			return
		}
		ids = append(ids, id)
	}
	if err := h.service.BulkAddRecipients(c.Request.Context(), broadcastID, ids); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.BulkRecipientsResponse{Count: len(ids)}))
}

func (h *BroadcastHandler) BulkRemoveRecipients(c *gin.Context) {
	broadcastID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid broadcast id", "INVALID_REQUEST"))
		return
	}
	var req httpdto.BulkRecipientsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	ids := make([]uuid.UUID, 0, len(req.UserIDs))
	for _, value := range req.UserIDs {
		id, err := uuid.Parse(value)
		if err != nil {
			c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid user_id", "INVALID_REQUEST"))
			return
		}
		ids = append(ids, id)
	}
	if err := h.service.BulkRemoveRecipients(c.Request.Context(), broadcastID, ids); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(httpdto.BulkRecipientsResponse{Count: len(ids)}))
}
