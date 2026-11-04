package controllers

import (
	"net/http"
	"strconv"

	"lechat-backend/internal/constant"
	"lechat-backend/internal/database/models"
	"lechat-backend/internal/interfaces"
	"lechat-backend/internal/request"
	"lechat-backend/utils"

	"github.com/gin-gonic/gin"
)

type ContactController struct {
	contacts interfaces.IContacts
}

func NewContactController(contacts interfaces.IContacts) *ContactController {
	return &ContactController{contacts: contacts}
}

func (c *ContactController) ListContacts(ctx *gin.Context) {
	claims, ok := authenticatedClaims(ctx)
	if !ok {
		writeChatError(ctx, http.StatusUnauthorized, constant.ErrInvalidToken)
		return
	}
	contacts, err := c.contacts.ListContacts(claims.UserId)
	if err != nil {
		writeChatServiceError(ctx, err)
		return
	}
	utils.SuccessResponse(ctx, http.StatusOK, contacts)
}

func (c *ContactController) ListRequests(ctx *gin.Context) {
	claims, ok := authenticatedClaims(ctx)
	if !ok {
		writeChatError(ctx, http.StatusUnauthorized, constant.ErrInvalidToken)
		return
	}
	requests, err := c.contacts.ListContactRequests(claims.UserId)
	if err != nil {
		writeChatServiceError(ctx, err)
		return
	}
	utils.SuccessResponse(ctx, http.StatusOK, requests)
}

func (c *ContactController) RequestContact(ctx *gin.Context) {
	claims, ok := authenticatedClaims(ctx)
	if !ok {
		writeChatError(ctx, http.StatusUnauthorized, constant.ErrInvalidToken)
		return
	}
	var req request.ContactRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeChatError(ctx, http.StatusBadRequest, constant.ErrChatInvalidRequest)
		return
	}
	targetID, status, err := c.contacts.RequestContactByPhone(claims.UserId, req.Phone)
	if err != nil {
		writeChatServiceError(ctx, err)
		return
	}
	utils.SuccessResponse(ctx, http.StatusCreated, gin.H{"user_id": targetID, "status": status})
}

func (c *ContactController) AcceptContact(ctx *gin.Context) {
	claims, targetID, ok := contactTarget(ctx)
	if !ok {
		return
	}
	if err := c.contacts.AcceptContact(claims.UserId, targetID); err != nil {
		writeChatServiceError(ctx, err)
		return
	}
	utils.SuccessResponse(ctx, http.StatusOK, gin.H{"user_id": targetID, "status": "accepted"})
}

func (c *ContactController) RejectContact(ctx *gin.Context) {
	claims, targetID, ok := contactTarget(ctx)
	if !ok {
		return
	}
	if err := c.contacts.RejectContact(claims.UserId, targetID); err != nil {
		writeChatServiceError(ctx, err)
		return
	}
	utils.SuccessResponse(ctx, http.StatusOK, gin.H{"user_id": targetID, "status": "rejected"})
}

func (c *ContactController) RemoveContact(ctx *gin.Context) {
	claims, targetID, ok := contactTarget(ctx)
	if !ok {
		return
	}
	if err := c.contacts.RemoveContact(claims.UserId, targetID); err != nil {
		writeChatServiceError(ctx, err)
		return
	}
	utils.SuccessResponse(ctx, http.StatusOK, gin.H{"user_id": targetID, "status": "removed"})
}

func contactTarget(ctx *gin.Context) (*models.JwtClaims, uint, bool) {
	claims, ok := authenticatedClaims(ctx)
	if !ok {
		writeChatError(ctx, http.StatusUnauthorized, constant.ErrInvalidToken)
		return nil, 0, false
	}
	targetID, err := strconv.ParseUint(ctx.Param("user_id"), 10, 64)
	if err != nil || targetID == 0 || uint64(uint(targetID)) != targetID {
		writeChatError(ctx, http.StatusBadRequest, constant.ErrChatInvalidRequest)
		return nil, 0, false
	}
	return claims, uint(targetID), true
}
