package controllers

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"

	"lechat-backend/internal/api/middleware"
	"lechat-backend/internal/constant"
	"lechat-backend/internal/database/models"
	"lechat-backend/internal/interfaces"
	"lechat-backend/internal/request"
	"lechat-backend/internal/services"
	"lechat-backend/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type ChatController struct {
	chat interfaces.IChat
	hub  *services.RealtimeHub
}

func NewChatController(chat interfaces.IChat, hub *services.RealtimeHub) *ChatController {
	return &ChatController{chat: chat, hub: hub}
}

func (c *ChatController) CreateConversation(ctx *gin.Context) {
	claims, ok := authenticatedClaims(ctx)
	if !ok {
		writeChatError(ctx, http.StatusUnauthorized, constant.ErrInvalidToken)
		return
	}

	var req request.CreateConversationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeChatError(ctx, http.StatusBadRequest, constant.ErrChatInvalidRequest)
		return
	}
	conversation, err := c.chat.CreateConversation(claims.UserId, req)
	if err != nil {
		writeChatServiceError(ctx, err)
		return
	}
	utils.SuccessResponse(ctx, http.StatusCreated, gin.H{
		"id":           conversation.ID,
		"conversation": conversation,
	})
}

func (c *ChatController) ListConversations(ctx *gin.Context) {
	claims, ok := authenticatedClaims(ctx)
	if !ok {
		writeChatError(ctx, http.StatusUnauthorized, constant.ErrInvalidToken)
		return
	}
	conversations, err := c.chat.ListConversations(claims.UserId)
	if err != nil {
		writeChatServiceError(ctx, err)
		return
	}
	utils.SuccessResponse(ctx, http.StatusOK, conversations)
}

func (c *ChatController) ListGroupMembers(ctx *gin.Context) {
	claims, ok := authenticatedClaims(ctx)
	if !ok {
		writeChatError(ctx, http.StatusUnauthorized, constant.ErrInvalidToken)
		return
	}
	conversationID := ctx.Param("conversation_id")
	if !uuidPattern.MatchString(conversationID) {
		writeChatError(ctx, http.StatusBadRequest, constant.ErrChatInvalidRequest)
		return
	}

	members, err := c.chat.ListGroupMembers(claims.UserId, conversationID)
	if err != nil {
		writeChatServiceError(ctx, err)
		return
	}
	utils.SuccessResponse(ctx, http.StatusOK, members)
}

func (c *ChatController) ListMessages(ctx *gin.Context) {
	claims, ok := authenticatedClaims(ctx)
	if !ok {
		writeChatError(ctx, http.StatusUnauthorized, constant.ErrInvalidToken)
		return
	}
	conversationID := ctx.Param("conversation_id")
	if !uuidPattern.MatchString(conversationID) {
		writeChatError(ctx, http.StatusBadRequest, constant.ErrChatInvalidRequest)
		return
	}

	limit := 50
	if value := ctx.Query("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 100 {
			writeChatError(ctx, http.StatusBadRequest, constant.ErrChatInvalidRequest)
			return
		}
		limit = parsed
	}
	beforeID := ctx.Query("before_id")
	if beforeID != "" && !uuidPattern.MatchString(beforeID) {
		writeChatError(ctx, http.StatusBadRequest, constant.ErrChatInvalidRequest)
		return
	}

	messages, err := c.chat.ListMessages(claims.UserId, conversationID, beforeID, limit)
	if err != nil {
		writeChatServiceError(ctx, err)
		return
	}
	nextBeforeID := ""
	if len(messages) == limit {
		nextBeforeID = messages[len(messages)-1].ID
	}
	utils.SuccessResponse(ctx, http.StatusOK, gin.H{
		"items":          messages,
		"next_before_id": nextBeforeID,
	})
}

func (c *ChatController) SendMessage(ctx *gin.Context) {
	claims, ok := authenticatedClaims(ctx)
	if !ok {
		writeChatError(ctx, http.StatusUnauthorized, constant.ErrInvalidToken)
		return
	}
	conversationID := ctx.Param("conversation_id")
	if !uuidPattern.MatchString(conversationID) {
		writeChatError(ctx, http.StatusBadRequest, constant.ErrChatInvalidRequest)
		return
	}

	var req request.SendMessageRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeChatError(ctx, http.StatusBadRequest, constant.ErrChatInvalidRequest)
		return
	}
	message, err := c.chat.SendMessage(claims.UserId, conversationID, req)
	if err != nil {
		writeChatServiceError(ctx, err)
		return
	}
	memberIDs, err := c.chat.ActiveMemberIDs(conversationID)
	if err != nil {
		writeChatServiceError(ctx, err)
		return
	}
	if err := c.hub.BroadcastUsers(memberIDs, realtimeEvent{
		Type:           "message.created",
		ConversationID: conversationID,
		Data:           message,
	}); err != nil {
		writeChatServiceError(ctx, err)
		return
	}
	utils.SuccessResponse(ctx, http.StatusCreated, message)
}

func (c *ChatController) MarkRead(ctx *gin.Context) {
	claims, ok := authenticatedClaims(ctx)
	if !ok {
		writeChatError(ctx, http.StatusUnauthorized, constant.ErrInvalidToken)
		return
	}
	conversationID := ctx.Param("conversation_id")
	if !uuidPattern.MatchString(conversationID) {
		writeChatError(ctx, http.StatusBadRequest, constant.ErrChatInvalidRequest)
		return
	}

	var req request.MarkReadRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeChatError(ctx, http.StatusBadRequest, constant.ErrChatInvalidRequest)
		return
	}
	result, err := c.chat.MarkRead(claims.UserId, conversationID, req)
	if err != nil {
		writeChatServiceError(ctx, err)
		return
	}
	if err := c.hub.BroadcastUsers(result.SenderIDs, realtimeEvent{
		Type:           "receipt.read",
		ConversationID: conversationID,
		Data: gin.H{
			"through_message_id": result.ThroughMessageID,
		},
	}); err != nil {
		writeChatServiceError(ctx, err)
		return
	}
	utils.SuccessResponse(ctx, http.StatusOK, result)
}

func (c *ChatController) MarkDelivered(ctx *gin.Context) {
	claims, ok := authenticatedClaims(ctx)
	if !ok {
		writeChatError(ctx, http.StatusUnauthorized, constant.ErrInvalidToken)
		return
	}
	conversationID := ctx.Param("conversation_id")
	if !uuidPattern.MatchString(conversationID) {
		writeChatError(ctx, http.StatusBadRequest, constant.ErrChatInvalidRequest)
		return
	}

	var req request.MarkReadRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeChatError(ctx, http.StatusBadRequest, constant.ErrChatInvalidRequest)
		return
	}
	result, err := c.chat.MarkDelivered(claims.UserId, conversationID, req)
	if err != nil {
		writeChatServiceError(ctx, err)
		return
	}
	utils.SuccessResponse(ctx, http.StatusOK, result)
}

func (c *ChatController) EditMessage(ctx *gin.Context) {
	claims, ok := authenticatedClaims(ctx)
	if !ok {
		writeChatError(ctx, http.StatusUnauthorized, constant.ErrInvalidToken)
		return
	}
	messageID := ctx.Param("message_id")
	if !uuidPattern.MatchString(messageID) {
		writeChatError(ctx, http.StatusBadRequest, constant.ErrChatInvalidRequest)
		return
	}

	var req request.EditMessageRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeChatError(ctx, http.StatusBadRequest, constant.ErrChatInvalidRequest)
		return
	}
	message, err := c.chat.EditMessage(claims.UserId, messageID, req)
	if err != nil {
		writeChatServiceError(ctx, err)
		return
	}
	utils.SuccessResponse(ctx, http.StatusOK, message)
}

func (c *ChatController) DeleteMessage(ctx *gin.Context) {
	claims, ok := authenticatedClaims(ctx)
	if !ok {
		writeChatError(ctx, http.StatusUnauthorized, constant.ErrInvalidToken)
		return
	}
	messageID := ctx.Param("message_id")
	if !uuidPattern.MatchString(messageID) {
		writeChatError(ctx, http.StatusBadRequest, constant.ErrChatInvalidRequest)
		return
	}

	message, err := c.chat.DeleteMessage(claims.UserId, messageID)
	if err != nil {
		writeChatServiceError(ctx, err)
		return
	}
	utils.SuccessResponse(ctx, http.StatusOK, message)
}

func (c *ChatController) InviteGroupMember(ctx *gin.Context) {
	claims, ok := authenticatedClaims(ctx)
	if !ok {
		writeChatError(ctx, http.StatusUnauthorized, constant.ErrInvalidToken)
		return
	}
	conversationID := ctx.Param("conversation_id")
	if !uuidPattern.MatchString(conversationID) {
		writeChatError(ctx, http.StatusBadRequest, constant.ErrChatInvalidRequest)
		return
	}
	var req request.InviteGroupMemberRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeChatError(ctx, http.StatusBadRequest, constant.ErrChatInvalidRequest)
		return
	}
	result, err := c.chat.InviteGroupMember(claims.UserId, conversationID, req)
	if err != nil {
		writeChatServiceError(ctx, err)
		return
	}
	utils.SuccessResponse(ctx, http.StatusCreated, result)
}

func (c *ChatController) ListGroupInvitations(ctx *gin.Context) {
	claims, ok := authenticatedClaims(ctx)
	if !ok {
		writeChatError(ctx, http.StatusUnauthorized, constant.ErrInvalidToken)
		return
	}
	result, err := c.chat.ListGroupInvitations(claims.UserId)
	if err != nil {
		writeChatServiceError(ctx, err)
		return
	}
	utils.SuccessResponse(ctx, http.StatusOK, result)
}

func (c *ChatController) RespondToGroupInvitation(ctx *gin.Context) {
	claims, ok := authenticatedClaims(ctx)
	if !ok {
		writeChatError(ctx, http.StatusUnauthorized, constant.ErrInvalidToken)
		return
	}
	invitationID := ctx.Param("invitation_id")
	if !uuidPattern.MatchString(invitationID) {
		writeChatError(ctx, http.StatusBadRequest, constant.ErrChatInvalidRequest)
		return
	}

	accept := ctx.Request.Method == http.MethodPost
	result, err := c.chat.RespondToGroupInvitation(claims.UserId, invitationID, accept)
	if err != nil {
		writeChatServiceError(ctx, err)
		return
	}
	utils.SuccessResponse(ctx, http.StatusOK, result)
}

func (c *ChatController) RemoveMember(ctx *gin.Context) {
	claims, ok := authenticatedClaims(ctx)
	if !ok {
		writeChatError(ctx, http.StatusUnauthorized, constant.ErrInvalidToken)
		return
	}
	conversationID := ctx.Param("conversation_id")
	memberID, err := strconv.ParseUint(ctx.Param("member_id"), 10, 64)
	if !uuidPattern.MatchString(conversationID) || err != nil || memberID == 0 || uint64(uint(memberID)) != memberID {
		writeChatError(ctx, http.StatusBadRequest, constant.ErrChatInvalidRequest)
		return
	}
	result, err := c.chat.RemoveMember(claims.UserId, conversationID, uint(memberID))
	if err != nil {
		writeChatServiceError(ctx, err)
		return
	}
	utils.SuccessResponse(ctx, http.StatusOK, result)
}

func authenticatedClaims(ctx *gin.Context) (*models.JwtClaims, bool) {
	value, exists := ctx.Get(middleware.AuthenticatedClaimsKey)
	if !exists {
		return nil, false
	}
	claims, ok := value.(*models.JwtClaims)
	return claims, ok && claims != nil
}

func writeChatServiceError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, constant.ErrChatInvalidRequest):
		writeChatError(ctx, http.StatusBadRequest, err)
	case errors.Is(err, constant.ErrChatInvalidReply):
		writeChatError(ctx, http.StatusUnprocessableEntity, err)
	case errors.Is(err, constant.ErrChatForbidden):
		writeChatError(ctx, http.StatusForbidden, err)
	case errors.Is(err, constant.ErrChatNotContact):
		writeChatError(ctx, http.StatusForbidden, err)
	case errors.Is(err, constant.ErrGroupInvitationInvalid):
		writeChatError(ctx, http.StatusConflict, err)
	case errors.Is(err, constant.ErrGroupInvitationPending):
		writeChatError(ctx, http.StatusConflict, err)
	case errors.Is(err, constant.ErrChatConflict):
		writeChatError(ctx, http.StatusConflict, err)
	case errors.Is(err, constant.ErrContactRequestPending):
		writeChatError(ctx, http.StatusConflict, err)
	case errors.Is(err, constant.ErrChatMessageDeleted):
		writeChatError(ctx, http.StatusConflict, err)
	case errors.Is(err, constant.ErrChatMemberExists), errors.Is(err, constant.ErrChatOwnerCannotLeave):
		writeChatError(ctx, http.StatusConflict, err)
	case errors.Is(err, gorm.ErrRecordNotFound):
		writeChatError(ctx, http.StatusNotFound, constant.ErrNotFoundData)
	default:
		writeChatError(ctx, http.StatusInternalServerError, constant.ErrInternalServer)
	}
}

func writeChatError(ctx *gin.Context, status int, err error) {
	utils.ErrorResponse(ctx, status, err, nil)
}
