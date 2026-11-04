package controllers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"lechat-backend/internal/constant"
	"lechat-backend/internal/database/models"
	"lechat-backend/internal/interfaces"
	"lechat-backend/internal/request"
	"lechat-backend/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

const (
	realtimeWriteWait  = 10 * time.Second
	realtimePongWait   = 60 * time.Second
	realtimePingPeriod = (realtimePongWait * 9) / 10
	realtimeAuthPeriod = 15 * time.Second
	realtimeAuthWait   = 8 * time.Second
	realtimeMaxMessage = 64 * 1024
)

type RealtimeController struct {
	chat     interfaces.IChat
	jwt      interfaces.IJwtInterface
	hub      *services.RealtimeHub
	origins  map[string]struct{}
	upgrader websocket.Upgrader
}

type realtimeFrame struct {
	Type             string `json:"type"`
	Token            string `json:"token,omitempty"`
	ConversationID   string `json:"conversation_id,omitempty"`
	BeforeID         string `json:"before_id,omitempty"`
	Limit            int    `json:"limit,omitempty"`
	ClientMessageID  string `json:"client_message_id,omitempty"`
	MessageType      string `json:"message_type,omitempty"`
	Body             string `json:"body,omitempty"`
	MessageID        string `json:"message_id,omitempty"`
	ThroughMessageID string `json:"through_message_id,omitempty"`
}

type realtimeEvent struct {
	Type           string      `json:"type"`
	ConversationID string      `json:"conversation_id,omitempty"`
	Data           interface{} `json:"data,omitempty"`
	Error          string      `json:"error,omitempty"`
}

func NewRealtimeController(chat interfaces.IChat, jwt interfaces.IJwtInterface, hub *services.RealtimeHub, allowedOrigins []string) *RealtimeController {
	origins := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			origins[origin] = struct{}{}
		}
	}

	controller := &RealtimeController{
		chat:    chat,
		jwt:     jwt,
		hub:     hub,
		origins: origins,
	}
	controller.upgrader = websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin:     controller.checkOrigin,
	}
	return controller
}

func (c *RealtimeController) Serve(ctx *gin.Context) {
	authHeader := ctx.GetHeader("Authorization")
	token := bearerToken(authHeader)
	if authHeader != "" && token == "" {
		ctx.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": constant.ErrInvalidToken.Error()})
		return
	}
	var claims *models.JwtClaims
	if token != "" {
		var err error
		claims, err = c.validateAccessToken(token)
		if err != nil {
			ctx.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": constant.ErrInvalidToken.Error()})
			return
		}
	}

	conn, err := c.upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return
	}
	conn.SetReadLimit(realtimeMaxMessage)
	if claims == nil {
		_ = conn.SetReadDeadline(time.Now().Add(realtimeAuthWait))
		var frame realtimeFrame
		if err := conn.ReadJSON(&frame); err != nil || frame.Type != "auth" || frame.Token == "" {
			_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "authentication required"), time.Now().Add(realtimeWriteWait))
			_ = conn.Close()
			return
		}
		claims, err = c.validateAccessToken(frame.Token)
		if err != nil {
			_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "invalid access token"), time.Now().Add(realtimeWriteWait))
			_ = conn.Close()
			return
		}
		token = frame.Token
	}

	_ = conn.SetReadDeadline(time.Now().Add(realtimePongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(realtimePongWait))
	})

	client := c.hub.Register(claims.UserId)
	defer func() {
		c.hub.Unregister(client)
		_ = conn.Close()
	}()
	if !c.hub.Send(client, realtimeEvent{Type: "auth.ok", Data: gin.H{"user_id": claims.UserId}}) {
		return
	}

	go c.writePump(conn, client, token)
	c.readPump(conn, client, claims.UserId)
}

func (c *RealtimeController) checkOrigin(req *http.Request) bool {
	origin := req.Header.Get("Origin")
	if origin == "" {
		return true
	}
	_, allowed := c.origins[origin]
	return allowed
}

func bearerToken(header string) string {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

func (c *RealtimeController) validateAccessToken(token string) (*models.JwtClaims, error) {
	claims, err := c.jwt.ValidateToken(token)
	if err != nil {
		return nil, err
	}
	if claims == nil || claims.UserId == 0 ||
		(claims.TokenType != "" && claims.TokenType != models.TokenTypeAccess) {
		return nil, constant.ErrInvalidToken
	}
	return claims, nil
}

func (c *RealtimeController) readPump(conn *websocket.Conn, client *services.RealtimeClient, userID uint) {
	for {
		var frame realtimeFrame
		if err := conn.ReadJSON(&frame); err != nil {
			return
		}
		if err := c.handleFrame(client, userID, frame); err != nil {
			c.hub.Send(client, realtimeEvent{Type: "error", Error: realtimeErrorMessage(err)})
		}
	}
}

func (c *RealtimeController) handleFrame(client *services.RealtimeClient, userID uint, frame realtimeFrame) error {
	switch frame.Type {
	case "conversation.join", "conversation.history":
		if !uuidPattern.MatchString(frame.ConversationID) {
			return constant.ErrChatInvalidRequest
		}
		limit := frame.Limit
		if limit == 0 {
			limit = 50
		}
		messages, err := c.chat.ListMessages(userID, frame.ConversationID, frame.BeforeID, limit)
		if err != nil {
			return err
		}
		if frame.Type == "conversation.join" {
			c.hub.Join(client, frame.ConversationID)
		}
		nextBeforeID := ""
		if len(messages) == limit {
			nextBeforeID = messages[len(messages)-1].ID
		}
		c.hub.Send(client, realtimeEvent{
			Type:           "conversation.history",
			ConversationID: frame.ConversationID,
			Data: gin.H{
				"items":          messages,
				"next_before_id": nextBeforeID,
				"joined":         frame.Type == "conversation.join",
			},
		})
		return nil
	case "conversation.leave":
		if !uuidPattern.MatchString(frame.ConversationID) {
			return constant.ErrChatInvalidRequest
		}
		c.hub.Leave(client, frame.ConversationID)
		c.hub.Send(client, realtimeEvent{Type: "conversation.left", ConversationID: frame.ConversationID})
		return nil
	case "message.send":
		if !uuidPattern.MatchString(frame.ConversationID) || !uuidPattern.MatchString(frame.ClientMessageID) {
			return constant.ErrChatInvalidRequest
		}
		if !c.hub.IsJoined(client, frame.ConversationID) {
			return constant.ErrChatForbidden
		}
		message, err := c.chat.SendMessage(userID, frame.ConversationID, request.SendMessageRequest{
			ClientMessageID: frame.ClientMessageID,
			Type:            frame.MessageType,
			Body:            frame.Body,
		})
		if err != nil {
			return err
		}
		return c.broadcastConversation(frame.ConversationID, realtimeEvent{
			Type:           "message.created",
			ConversationID: frame.ConversationID,
			Data:           message,
		})
	case "receipt.delivered", "receipt.read":
		if !uuidPattern.MatchString(frame.ConversationID) || !uuidPattern.MatchString(frame.ThroughMessageID) {
			return constant.ErrChatInvalidRequest
		}
		if !c.hub.IsJoined(client, frame.ConversationID) {
			return constant.ErrChatForbidden
		}
		req := request.MarkReadRequest{ThroughMessageID: frame.ThroughMessageID}
		var receipt interface{}
		var err error
		if frame.Type == "receipt.delivered" {
			receipt, err = c.chat.MarkDelivered(userID, frame.ConversationID, req)
		} else {
			receipt, err = c.chat.MarkRead(userID, frame.ConversationID, req)
		}
		if err != nil {
			return err
		}
		return c.broadcastConversation(frame.ConversationID, realtimeEvent{
			Type:           frame.Type,
			ConversationID: frame.ConversationID,
			Data:           receipt,
		})
	case "message.edit":
		if !uuidPattern.MatchString(frame.MessageID) {
			return constant.ErrChatInvalidRequest
		}
		message, err := c.chat.EditMessage(userID, frame.MessageID, request.EditMessageRequest{Body: frame.Body})
		if err != nil {
			return err
		}
		return c.broadcastConversation(message.ConversationID, realtimeEvent{
			Type:           "message.updated",
			ConversationID: message.ConversationID,
			Data:           message,
		})
	case "message.delete":
		if !uuidPattern.MatchString(frame.MessageID) {
			return constant.ErrChatInvalidRequest
		}
		message, err := c.chat.DeleteMessage(userID, frame.MessageID)
		if err != nil {
			return err
		}
		return c.broadcastConversation(message.ConversationID, realtimeEvent{
			Type:           "message.deleted",
			ConversationID: message.ConversationID,
			Data:           message,
		})
	default:
		return constant.ErrChatInvalidRequest
	}
}

func (c *RealtimeController) broadcastConversation(conversationID string, event realtimeEvent) error {
	memberIDs, err := c.chat.ActiveMemberIDs(conversationID)
	if err != nil {
		return err
	}
	return c.hub.Broadcast(conversationID, memberIDs, event)
}

func (c *RealtimeController) writePump(conn *websocket.Conn, client *services.RealtimeClient, token string) {
	pingTicker := time.NewTicker(realtimePingPeriod)
	authTicker := time.NewTicker(realtimeAuthPeriod)
	defer pingTicker.Stop()
	defer authTicker.Stop()
	defer conn.Close()
	for {
		select {
		case payload := <-client.SendQueue():
			_ = conn.SetWriteDeadline(time.Now().Add(realtimeWriteWait))
			if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
		case <-authTicker.C:
			if _, err := c.validateAccessToken(token); err != nil {
				_ = conn.SetWriteDeadline(time.Now().Add(realtimeWriteWait))
				_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "access token expired or revoked"))
				return
			}
		case <-pingTicker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(realtimeWriteWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-client.Done():
			_ = conn.SetWriteDeadline(time.Now().Add(realtimeWriteWait))
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			return
		}
	}
}

func realtimeErrorMessage(err error) string {
	switch {
	case errors.Is(err, constant.ErrChatInvalidRequest):
		return constant.ErrChatInvalidRequest.Error()
	case errors.Is(err, constant.ErrChatForbidden), errors.Is(err, constant.ErrChatNotContact):
		return constant.ErrChatForbidden.Error()
	case errors.Is(err, gorm.ErrRecordNotFound):
		return constant.ErrNotFoundData.Error()
	case errors.Is(err, constant.ErrChatConflict):
		return constant.ErrChatConflict.Error()
	default:
		return constant.ErrInternalServer.Error()
	}
}
