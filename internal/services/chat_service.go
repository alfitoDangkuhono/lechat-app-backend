package services

import (
	"math"
	"regexp"
	"sort"
	"strings"

	"lechat-backend/internal/constant"
	"lechat-backend/internal/database/dto"
	"lechat-backend/internal/interfaces"
	"lechat-backend/internal/repository"
	"lechat-backend/internal/request"
)

var groupInvitationUUIDPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type ChatService struct {
	chatRepository *repository.ChatRepository
}

func NewChatService(chatRepository *repository.ChatRepository) interfaces.IChat {
	return &ChatService{chatRepository: chatRepository}
}

func (s *ChatService) CreateConversation(userID uint, req request.CreateConversationRequest) (dto.ConversationCreationDto, error) {
	if req.Type != "direct" && req.Type != "group" {
		return dto.ConversationCreationDto{}, constant.ErrChatInvalidRequest
	}

	memberSet := map[uint]struct{}{userID: {}}
	for _, memberID := range req.MemberIDs {
		if memberID == 0 || uint64(memberID) > math.MaxInt64 {
			return dto.ConversationCreationDto{}, constant.ErrChatInvalidRequest
		}
		if req.Type == "group" && memberID == userID {
			return dto.ConversationCreationDto{}, constant.ErrChatInvalidRequest
		}
		memberSet[memberID] = struct{}{}
	}

	memberIDs := make([]uint, 0, len(memberSet))
	for memberID := range memberSet {
		memberIDs = append(memberIDs, memberID)
	}
	sort.Slice(memberIDs, func(i, j int) bool { return memberIDs[i] < memberIDs[j] })

	if (req.Type == "direct" && len(memberIDs) != 2) ||
		(req.Type == "group" && len(memberIDs) < 2) {
		return dto.ConversationCreationDto{}, constant.ErrChatInvalidRequest
	}
	if req.Type == "direct" {
		req.Title = nil
	} else if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			req.Title = nil
		} else {
			req.Title = &title
		}
	}
	if req.Type == "group" && req.Title == nil {
		title := "Grup baru"
		req.Title = &title
	}

	conversationID, err := s.chatRepository.CreateConversation(userID, req, memberIDs)
	if err != nil {
		return dto.ConversationCreationDto{}, err
	}
	return dto.ConversationCreationDto{
		ID:          conversationID,
		Type:        req.Type,
		Title:       req.Title,
		MemberCount: int64(len(memberIDs)),
	}, nil
}

func (s *ChatService) ListConversations(userID uint) ([]dto.ConversationDto, error) {
	return s.chatRepository.ListConversations(userID)
}

func (s *ChatService) ListGroupMembers(userID uint, conversationID string) ([]dto.ConversationMemberDto, error) {
	return s.chatRepository.ListGroupMembers(userID, conversationID)
}

func (s *ChatService) ListMessages(userID uint, conversationID, beforeID string, limit int) ([]dto.MessageDto, error) {
	if limit < 1 || limit > 100 {
		return nil, constant.ErrChatInvalidRequest
	}
	return s.chatRepository.ListMessages(userID, conversationID, beforeID, limit)
}

func (s *ChatService) SendMessage(userID uint, conversationID string, req request.SendMessageRequest) (dto.MessageDto, error) {
	if req.Type != "text" {
		return dto.MessageDto{}, constant.ErrChatInvalidRequest
	}
	if req.Type == "text" && strings.TrimSpace(req.Body) == "" {
		return dto.MessageDto{}, constant.ErrChatInvalidRequest
	}
	if req.ReplyToMessageID != nil && !groupInvitationUUIDPattern.MatchString(*req.ReplyToMessageID) {
		return dto.MessageDto{}, constant.ErrChatInvalidReply
	}
	return s.chatRepository.SendMessage(userID, conversationID, req)
}

func (s *ChatService) MarkRead(userID uint, conversationID string, req request.MarkReadRequest) (dto.ReadReceiptDto, error) {
	return s.chatRepository.MarkRead(userID, conversationID, req)
}

func (s *ChatService) MarkDelivered(userID uint, conversationID string, req request.MarkReadRequest) (dto.ReadReceiptDto, error) {
	return s.chatRepository.MarkDelivered(userID, conversationID, req)
}

func (s *ChatService) EditMessage(userID uint, messageID string, req request.EditMessageRequest) (dto.MessageDto, error) {
	if strings.TrimSpace(req.Body) == "" {
		return dto.MessageDto{}, constant.ErrChatInvalidRequest
	}
	return s.chatRepository.EditMessage(userID, messageID, req)
}

func (s *ChatService) DeleteMessage(userID uint, messageID string) (dto.MessageDto, error) {
	return s.chatRepository.DeleteMessage(userID, messageID)
}

func (s *ChatService) InviteGroupMember(userID uint, conversationID string, req request.InviteGroupMemberRequest) (dto.GroupInvitationDto, error) {
	if req.UserID == 0 || uint64(req.UserID) > math.MaxInt64 {
		return dto.GroupInvitationDto{}, constant.ErrChatInvalidRequest
	}
	return s.chatRepository.InviteGroupMember(userID, conversationID, req)
}

func (s *ChatService) ListGroupInvitations(userID uint) ([]dto.GroupInvitationDto, error) {
	return s.chatRepository.ListGroupInvitations(userID)
}

func (s *ChatService) RespondToGroupInvitation(userID uint, invitationID string, accept bool) (dto.MembershipDto, error) {
	if !groupInvitationUUIDPattern.MatchString(invitationID) {
		return dto.MembershipDto{}, constant.ErrChatInvalidRequest
	}
	return s.chatRepository.RespondToGroupInvitation(userID, invitationID, accept)
}

func (s *ChatService) RemoveMember(userID uint, conversationID string, memberID uint) (dto.MembershipDto, error) {
	if memberID == 0 || uint64(memberID) > math.MaxInt64 {
		return dto.MembershipDto{}, constant.ErrChatInvalidRequest
	}
	return s.chatRepository.RemoveMember(userID, conversationID, memberID)
}

func (s *ChatService) ActiveMemberIDs(conversationID string) ([]uint, error) {
	return s.chatRepository.ActiveMemberIDs(conversationID)
}
