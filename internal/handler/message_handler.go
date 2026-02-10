package handler

import (
	"encoding/base64"
	"net/http"
	"strconv"
	"time"

	"sentinal-chat/internal/services"
	"sentinal-chat/internal/transport/httpdto"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type MessageHandler struct {
	service *services.MessageService
}

func NewMessageHandler(service *services.MessageService) *MessageHandler {
	return &MessageHandler{service: service}
}

func (h *MessageHandler) Send(c *gin.Context) {
	var req httpdto.SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}

	userID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}

	conversationID, err := parseUUID(req.ConversationID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid conversation_id", "INVALID_REQUEST"))
		return
	}

	if len(req.Ciphertexts) == 0 {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("ciphertexts required", "INVALID_REQUEST"))
		return
	}

	items := make([]services.CiphertextPayload, 0, len(req.Ciphertexts))
	for _, payload := range req.Ciphertexts {
		recipientDeviceID, err := parseUUID(payload.RecipientDeviceID)
		if err != nil {
			c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid recipient_device_id", "INVALID_REQUEST"))
			return
		}
		if payload.Ciphertext == "" {
			c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("ciphertext required", "INVALID_REQUEST"))
			return
		}
		ciphertext, err := base64.StdEncoding.DecodeString(payload.Ciphertext)
		if err != nil {
			c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("ciphertext must be base64", "INVALID_REQUEST"))
			return
		}
		items = append(items, services.CiphertextPayload{
			RecipientDeviceID: recipientDeviceID,
			Ciphertext:        ciphertext,
			Header:            payload.Header,
		})
	}

	result, err := h.service.SendMessage(c.Request.Context(), services.SendMessageInput{
		ConversationID: conversationID,
		SenderID:       userID,
		Ciphertexts:    items,
		MessageType:    req.MessageType,
		ClientMsgID:    req.ClientMsgID,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}

	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(gin.H{"message": result}))
}

func (h *MessageHandler) List(c *gin.Context) {
	conversationID, err := parseUUID(c.Query("conversation_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid conversation_id", "INVALID_REQUEST"))
		return
	}

	beforeSeq, err := parseInt64(c.Query("before_seq"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid before_seq", "INVALID_REQUEST"))
		return
	}

	limit, err := parseInt(c.Query("limit"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid limit", "INVALID_REQUEST"))
		return
	}

	userID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}

	items, err := h.service.GetConversationMessages(c.Request.Context(), conversationID, beforeSeq, limit, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}

	response := make([]httpdto.MessageDTO, 0, len(items))
	for _, item := range items {
		dto := httpdto.MessageDTO{
			ID:                item.ID.String(),
			ConversationID:    item.ConversationID.String(),
			SenderID:          item.SenderID.String(),
			ClientMsgID:       item.ClientMessageID.String,
			SequenceNumber:    item.SeqID.Int64,
			IsDeleted:         item.DeletedAt.Valid,
			IsEdited:          item.EditedAt.Valid,
			Ciphertext:        base64.StdEncoding.EncodeToString(item.Ciphertext),
			Header:            item.Header,
			RecipientDeviceID: nullUUIDString(item.RecipientDeviceID),
			CreatedAt:         item.CreatedAt.Format(time.RFC3339),
		}
		response = append(response, dto)
	}

	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(gin.H{"messages": response}))
}

func nullUUIDString(value uuid.NullUUID) string {
	if value.Valid {
		return value.UUID.String()
	}
	return ""
}

func (h *MessageHandler) GetByID(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, httpdto.NewErrorResponse("message detail not supported for e2e", "NOT_SUPPORTED"))
}

func (h *MessageHandler) Delete(c *gin.Context) {
	messageID, err := parseUUID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid message id", "INVALID_REQUEST"))
		return
	}
	if err := h.service.Delete(c.Request.Context(), messageID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *MessageHandler) Update(c *gin.Context) {
	var req httpdto.UpdateMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid request", "INVALID_REQUEST"))
		return
	}
	c.JSON(http.StatusNotImplemented, httpdto.NewErrorResponse("message updates not supported for e2e", "NOT_SUPPORTED"))
}

func (h *MessageHandler) HardDelete(c *gin.Context) {
	messageID, err := parseUUID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid message id", "INVALID_REQUEST"))
		return
	}
	if err := h.service.HardDelete(c.Request.Context(), messageID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *MessageHandler) MarkRead(c *gin.Context) {
	messageID, err := parseUUID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid message id", "INVALID_REQUEST"))
		return
	}
	userID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}
	if err := h.service.MarkAsRead(c.Request.Context(), messageID, userID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func (h *MessageHandler) MarkDelivered(c *gin.Context) {
	messageID, err := parseUUID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse("invalid message id", "INVALID_REQUEST"))
		return
	}
	userID, ok := services.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, httpdto.NewErrorResponse("unauthorized", "UNAUTHORIZED"))
		return
	}
	if err := h.service.MarkAsDelivered(c.Request.Context(), messageID, userID); err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse[any](nil))
}

func parseUUID(value string) (uuid.UUID, error) {
	return uuid.Parse(value)
}

func parseInt(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

func parseInt64(value string) (int64, error) {
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}
