package interfaces

import (
	"lechat-backend/internal/database/dto"
	"lechat-backend/internal/request"
)

type IChat interface {
	CreateConversation(userID uint, req request.CreateConversationRequest) (dto.ConversationCreationDto, error)
	ListConversations(userID uint) ([]dto.ConversationDto, error)
	ListGroupMembers(userID uint, conversationID string) ([]dto.ConversationMemberDto, error)
	ListMessages(userID uint, conversationID, beforeID string, limit int) ([]dto.MessageDto, error)
	SendMessage(userID uint, conversationID string, req request.SendMessageRequest) (dto.MessageDto, error)
	MarkDelivered(userID uint, conversationID string, req request.MarkReadRequest) (dto.ReadReceiptDto, error)
	MarkRead(userID uint, conversationID string, req request.MarkReadRequest) (dto.ReadReceiptDto, error)
	EditMessage(userID uint, messageID string, req request.EditMessageRequest) (dto.MessageDto, error)
	DeleteMessage(userID uint, messageID string) (dto.MessageDto, error)
	RemoveMember(userID uint, conversationID string, memberID uint) (dto.MembershipDto, error)
	InviteGroupMember(userID uint, conversationID string, req request.InviteGroupMemberRequest) (dto.GroupInvitationDto, error)
	ListGroupInvitations(userID uint) ([]dto.GroupInvitationDto, error)
	RespondToGroupInvitation(userID uint, invitationID string, accept bool) (dto.MembershipDto, error)
	ActiveMemberIDs(conversationID string) ([]uint, error)
}
