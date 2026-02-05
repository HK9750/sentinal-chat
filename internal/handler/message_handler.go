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

	result, err := h.service.HandleSendMessage(c.Request.Context(), commands.SendMessageCommand{
		ConversationID:      conversationID,
		SenderID:            userID,
		Content:             req.Content,
		ClientMsgID:         req.ClientMsgID,
		IdempotencyKeyValue: req.IdempotencyKey,
	})
	if err != nil {
		if err == commands.ErrDuplicateCommand {
			c.JSON(http.StatusConflict, httpdto.NewErrorResponse(err.Error(), "DUPLICATE_COMMAND"))
			return
		}
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}

	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(result))
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

	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(gin.H{"messages": items}))
}

func (h *MessageHandler) GetByID(c *gin.Context) {
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
	msg, err := h.service.GetByID(c.Request.Context(), messageID, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpdto.NewErrorResponse(err.Error(), "REQUEST_FAILED"))
		return
	}
	c.JSON(http.StatusOK, httpdto.NewSuccessResponse(msg))
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
