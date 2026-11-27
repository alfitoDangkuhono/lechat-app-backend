package repository

import (
	"errors"
	"math"

	"encoding/base64"
	"lechat-backend/internal/constant"
	"lechat-backend/internal/database/dto"
	"lechat-backend/internal/database/models"
	"lechat-backend/internal/request"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

type ChatRepository struct {
	db *gorm.DB
}

func messageDTOQuery(db *gorm.DB, receiptUserID uint) *gorm.DB {
	return db.Table("messages").
		Select(`
			messages.id, messages.conversation_id, messages.sender_id, messages.client_message_id,
			messages.reply_to_message_id, messages.type, messages.body, messages.ciphertext,
			messages.iv, messages.encryption_version, messages.key_version, messages.edited_at,
			messages.deleted_at, messages.created_at, messages.updated_at,
			sender.username AS from_username,
			CASE WHEN conversation.type = 'direct' THEN peer.username ELSE '' END AS to_username,
			reply.sender_id AS reply_to_sender_id,
			reply_sender.username AS reply_to_username,
			CASE WHEN reply.deleted_at IS NULL THEN COALESCE(reply.body, '') ELSE '' END AS reply_to_body,
			reply.type AS reply_to_type,
			(reply.deleted_at IS NOT NULL) AS reply_to_is_deleted,
			(SELECT COUNT(*) FROM messages_receipts WHERE message_id = messages.id AND delivered_at IS NOT NULL) AS delivered_count,
			(SELECT COUNT(*) FROM messages_receipts WHERE message_id = messages.id AND read_at IS NOT NULL) AS read_count,
			CASE WHEN messages.sender_id = ? THEN
				CASE
					WHEN EXISTS (
						SELECT 1 FROM messages_receipts
						WHERE message_id = messages.id
						  AND user_id <> messages.sender_id
						  AND read_at IS NOT NULL
					) THEN 'read'
					WHEN EXISTS (
						SELECT 1 FROM messages_receipts
						WHERE message_id = messages.id
						  AND user_id <> messages.sender_id
						  AND delivered_at IS NOT NULL
					) THEN 'delivered'
					ELSE 'sent'
				END
				ELSE ''
			END AS receipt_status
		`, receiptUserID).
		Joins("JOIN users AS sender ON sender.id = messages.sender_id").
		Joins("JOIN conversations AS conversation ON conversation.id = messages.conversation_id").
		Joins("LEFT JOIN conversations_members AS peer_member ON peer_member.conversation_id = messages.conversation_id AND peer_member.user_id <> messages.sender_id AND peer_member.left_at IS NULL AND conversation.type = 'direct'").
		Joins("LEFT JOIN users AS peer ON peer.id = peer_member.user_id").
		Joins("LEFT JOIN messages AS reply ON reply.id = messages.reply_to_message_id").
		Joins("LEFT JOIN users AS reply_sender ON reply_sender.id = reply.sender_id")
}

func finalizeMessage(message *dto.MessageDto) {
	if len(message.IVBytes) > 0 {
		encoded := base64.StdEncoding.EncodeToString(message.IVBytes)
		message.IV = &encoded
	}
	message.MessageType = message.Type
	setMessageReply(message)
}

func setMessageReply(message *dto.MessageDto) {
	if message.ReplyToMessageID == nil || message.ReplyToSenderID == nil ||
		message.ReplyToUsername == nil || message.ReplyToBody == nil ||
		message.ReplyToType == nil || message.ReplyToIsDeleted == nil {
		return
	}
	message.ReplyTo = &dto.MessageReplyDto{
		ID:        *message.ReplyToMessageID,
		SenderID:  *message.ReplyToSenderID,
		Username:  *message.ReplyToUsername,
		Body:      *message.ReplyToBody,
		Type:      *message.ReplyToType,
		IsDeleted: *message.ReplyToIsDeleted,
	}
}

func NewChatRepository(db *gorm.DB) *ChatRepository {
	return &ChatRepository{db: db}
}

func (r *ChatRepository) CreateConversation(userID uint, req request.CreateConversationRequest, memberIDs []uint) (string, error) {
	var conversationID string
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var existingUsers int64
		if err := tx.Model(&models.Users{}).Where("id IN ?", memberIDs).Count(&existingUsers).Error; err != nil {
			return err
		}
		if existingUsers != int64(len(memberIDs)) {
			return constant.ErrChatInvalidRequest
		}

		if req.Type == "group" {
			contactIDs := make([]uint, 0, len(memberIDs)-1)
			for _, memberID := range memberIDs {
				if memberID != userID {
					contactIDs = append(contactIDs, memberID)
				}
			}

			var acceptedContacts int64
			if err := tx.Raw(`
				SELECT COUNT(*)
				FROM contacts
				WHERE status = 'accepted'
				  AND ((user_low = ? AND user_high IN ?) OR (user_high = ? AND user_low IN ?))
			`, userID, contactIDs, userID, contactIDs).Scan(&acceptedContacts).Error; err != nil {
				return err
			}
			if acceptedContacts != int64(len(contactIDs)) {
				return constant.ErrChatNotContact
			}
		}

		if req.Type == "direct" {
			low, high := memberIDs[0], memberIDs[1]
			var lockAcquired bool
			if err := tx.Raw(`
				SELECT pg_advisory_xact_lock(
					hashtextextended(CONCAT(?::bigint::text, ':', ?::bigint::text), 0)
				) IS NULL
			`, int64(low), int64(high)).Scan(&lockAcquired).Error; err != nil {
				return err
			}
			var areContacts bool
			if err := tx.Raw(`
				SELECT EXISTS (
					SELECT 1 FROM contacts
					WHERE user_low = ? AND user_high = ? AND status = 'accepted'
				)
			`, low, high).Scan(&areContacts).Error; err != nil {
				return err
			}
			if !areContacts {
				return constant.ErrChatNotContact
			}

			err := tx.Raw(`
				SELECT id
				FROM conversations
				WHERE type = 'direct'
				  AND direct_user_low = ?
				  AND direct_user_high = ?
				LIMIT 1
			`, low, high).Scan(&conversationID).Error
			if err != nil {
				return err
			}
			if conversationID != "" {
				for _, memberID := range memberIDs {
					role := "member"
					if memberID == userID {
						role = "owner"
					}
					if err := tx.Exec(`
						INSERT INTO conversations_members (conversation_id, user_id, role)
						VALUES (?, ?, ?)
						ON CONFLICT (conversation_id, user_id)
						DO UPDATE SET joined_at = CURRENT_TIMESTAMP, left_at = NULL
					`, conversationID, memberID, role).Error; err != nil {
						return err
					}
				}
				return nil
			}
		}

		err := tx.Raw(`
			INSERT INTO conversations (type, title, created_by, direct_user_low, direct_user_high)
			VALUES (?, ?, ?, ?, ?)
			RETURNING id
		`, req.Type, req.Title, userID, directUserID(req.Type, memberIDs, 0), directUserID(req.Type, memberIDs, 1)).
			Scan(&conversationID).Error
		if err != nil {
			return err
		}

		members := make([]models.ConversationMember, 0, len(memberIDs))
		for _, memberID := range memberIDs {
			role := "member"
			if memberID == userID {
				role = "owner"
			}
			members = append(members, models.ConversationMember{
				ConversationID: conversationID,
				UserID:         int64(memberID),
				Role:           role,
			})
		}
		return tx.Table("conversations_members").Create(&members).Error
	})
	return conversationID, err
}

func directUserID(conversationType string, memberIDs []uint, index int) *uint {
	if conversationType != "direct" {
		return nil
	}
	return &memberIDs[index]
}

func (r *ChatRepository) ListConversations(userID uint) ([]dto.ConversationDto, error) {
	var conversations []dto.ConversationDto
	err := r.db.Raw(`
		SELECT
			c.id,
			c.type,
			c.title,
			c.created_at,
			c.updated_at,
			latest.id AS last_message_id,
			latest.body AS last_message_body,
			latest.type AS last_message_type,
			latest.last_message_encrypted AS last_message_encrypted,
			latest.created_at AS last_message_at,
			(
				SELECT COUNT(*)
				FROM messages AS unread
				LEFT JOIN messages_receipts AS receipt
				  ON receipt.message_id = unread.id
				 AND receipt.user_id = ?
				WHERE unread.conversation_id = c.id
				  AND unread.sender_id <> ?
				  AND unread.deleted_at IS NULL
				  AND receipt.read_at IS NULL
			) AS unread_message_count
		FROM conversations AS c
		JOIN conversations_members AS member
		  ON member.conversation_id = c.id
		 AND member.user_id = ?
		 AND member.left_at IS NULL
		 AND (
			c.type = 'group'
			OR EXISTS (
				SELECT 1 FROM contacts
				WHERE user_low = c.direct_user_low
				  AND user_high = c.direct_user_high
				  AND status = 'accepted'
			)
		 )
		LEFT JOIN LATERAL (
			SELECT id, CASE WHEN encryption_version IS NOT NULL THEN NULL ELSE body END AS body, type, created_at, (encryption_version IS NOT NULL) AS last_message_encrypted
			FROM messages
			WHERE conversation_id = c.id
			ORDER BY created_at DESC, id DESC
			LIMIT 1
		) AS latest ON TRUE
		ORDER BY COALESCE(latest.created_at, c.created_at) DESC, c.id DESC
	`, userID, userID, userID).Scan(&conversations).Error
	if err != nil || len(conversations) == 0 {
		return conversations, err
	}

	conversationIDs := make([]string, 0, len(conversations))
	for i := range conversations {
		conversations[i].Members = make([]dto.ConversationMemberDto, 0)
		conversationIDs = append(conversationIDs, conversations[i].ID)
	}

	var participants []models.ConversationParticipant
	err = r.db.Table("conversations_members AS member").
		Select("member.conversation_id, users.id AS user_id, users.username, COALESCE(users.first_name, '') AS first_name, COALESCE(users.last_name, '') AS last_name, member.role").
		Joins("JOIN users ON users.id = member.user_id").
		Where("member.conversation_id IN ? AND member.left_at IS NULL", conversationIDs).
		Order("member.joined_at ASC, users.id ASC").
		Find(&participants).Error
	if err != nil {
		return nil, err
	}
	byConversation := make(map[string]int, len(conversations))
	for i := range conversations {
		byConversation[conversations[i].ID] = i
	}
	for _, participant := range participants {
		index := byConversation[participant.ConversationID]
		conversations[index].Members = append(conversations[index].Members, dto.ConversationMemberDto{
			UserID:    participant.UserID,
			Username:  participant.Username,
			FirstName: participant.FirstName,
			LastName:  participant.LastName,
			Role:      participant.Role,
		})
	}
	return conversations, err
}

func (r *ChatRepository) ListGroupMembers(userID uint, conversationID string) ([]dto.ConversationMemberDto, error) {
	if _, err := activeGroupRole(r.db, userID, conversationID); err != nil {
		return nil, err
	}

	members := make([]dto.ConversationMemberDto, 0)
	err := r.db.Table("conversations_members AS member").
		Select("users.id AS user_id, users.username, COALESCE(users.first_name, '') AS first_name, COALESCE(users.last_name, '') AS last_name, member.role").
		Joins("JOIN users ON users.id = member.user_id").
		Where("member.conversation_id = ? AND member.left_at IS NULL", conversationID).
		Order("member.joined_at ASC, users.id ASC").
		Find(&members).Error
	return members, err
}

func (r *ChatRepository) ListMessages(userID uint, conversationID, beforeID string, limit int) ([]dto.MessageDto, error) {
	if _, err := r.requireActiveMember(userID, conversationID); err != nil {
		return nil, err
	}

	query := messageDTOQuery(r.db, userID).
		Where("messages.conversation_id = ?", conversationID).
		Order("messages.created_at DESC, messages.id DESC").
		Limit(limit)
	if beforeID != "" {
		var cursorExists bool
		err := r.db.Raw(`
			SELECT EXISTS (
				SELECT 1 FROM messages WHERE id = ? AND conversation_id = ?
			)
		`, beforeID, conversationID).Scan(&cursorExists).Error
		if err != nil {
			return nil, err
		}
		if !cursorExists {
			return nil, gorm.ErrRecordNotFound
		}
		query = query.Where(
			"(messages.created_at, messages.id) < (SELECT created_at, id FROM messages WHERE id = ? AND conversation_id = ?)",
			beforeID,
			conversationID,
		)
	}

	var messages []dto.MessageDto
	err := query.Find(&messages).Error
	for i := range messages {
		finalizeMessage(&messages[i])
	}
	return messages, err
}

func (r *ChatRepository) SendMessage(userID uint, conversationID string, req request.SendMessageRequest) (dto.MessageDto, error) {
	if _, err := r.requireActiveMember(userID, conversationID); err != nil {
		return dto.MessageDto{}, err
	}
	if req.ReplyToMessageID != nil {
		var replyTargetExists bool
		if err := r.db.Raw(`
			SELECT EXISTS (
				SELECT 1
				FROM messages
				WHERE id = ? AND conversation_id = ?
			)
		`, *req.ReplyToMessageID, conversationID).Scan(&replyTargetExists).Error; err != nil {
			return dto.MessageDto{}, err
		}
		if !replyTargetExists {
			return dto.MessageDto{}, constant.ErrChatInvalidReply
		}
	}

	// The server decides the payload shape from the stored conversation type:
	// direct messages must be encrypted payloads, group messages stay legacy plaintext.
	encrypted := req.EncryptionVersion != nil && req.Ciphertext != nil && req.IV != nil

	message := models.Message{
		ConversationID:    conversationID,
		SenderID:          int64(userID),
		ClientMessageID:   req.ClientMessageID,
		ReplyToMessageID:  req.ReplyToMessageID,
		Type:              req.Type,
		EncryptionVersion: req.EncryptionVersion,
	}
	if encrypted {
		decodedIV, err := base64.StdEncoding.Strict().DecodeString(*req.IV)
		if err != nil || len(decodedIV) != 12 {
			return dto.MessageDto{}, constant.ErrChatInvalidRequest
		}
		iv := decodedIV
		message.IV = &iv
		message.Ciphertext = req.Ciphertext
		message.KeyVersion = req.KeyVersion
	} else {
		body := req.Body
		message.Body = &body
	}

	if err := r.db.Table("messages").Create(&message).Error; err != nil {
		var existing dto.MessageDto
		findErr := messageDTOQuery(r.db, userID).
			Where("messages.sender_id = ? AND messages.client_message_id = ?", userID, req.ClientMessageID).
			Take(&existing).Error
		if findErr == nil {
			finalizeMessage(&existing)
			if existing.ConversationID == conversationID && existing.Type == req.Type &&
				sameMessagePayload(&existing, req) {
				return existing, nil
			}
			return dto.MessageDto{}, constant.ErrChatConflict
		}
		return dto.MessageDto{}, err
	}

	var created dto.MessageDto
	if err := messageDTOQuery(r.db, userID).Where("messages.id = ?", message.ID).Take(&created).Error; err != nil {
		return dto.MessageDto{}, err
	}
	finalizeMessage(&created)
	return created, nil
}

func sameMessagePayload(existing *dto.MessageDto, req request.SendMessageRequest) bool {
	if !sameOptionalString(existing.ReplyToMessageID, req.ReplyToMessageID) {
		return false
	}
	if req.EncryptionVersion != nil {
		return existing.EncryptionVersion != nil && *existing.EncryptionVersion == *req.EncryptionVersion &&
			existing.Ciphertext != nil && *existing.Ciphertext == *req.Ciphertext &&
			existing.IV != nil && *existing.IV == *req.IV &&
			sameOptionalInt(existing.KeyVersion, req.KeyVersion) &&
			existing.Body == nil
	}
	return existing.Body != nil && *existing.Body == req.Body && existing.Ciphertext == nil
}

func sameOptionalInt(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func (r *ChatRepository) AckMessages(userID uint, conversationID string, req request.AckMessagesRequest) error {
	if _, err := r.requireActiveMember(userID, conversationID); err != nil {
		return err
	}
	result := r.db.Exec(`
		INSERT INTO messages_acks (message_id, user_id)
		SELECT id, ?
		FROM messages
		WHERE conversation_id = ?
		  AND id IN ?
		  AND sender_id <> ?
		ON CONFLICT (message_id, user_id)
		DO NOTHING
	`, userID, conversationID, req.IDs, userID)
	return result.Error
}

func (r *ChatRepository) ConversationType(userID uint, conversationID string) (string, error) {
	access, err := r.requireActiveMember(userID, conversationID)
	if err != nil {
		return "", err
	}
	return access.Type, nil
}

func sameOptionalString(first, second *string) bool {
	if first == nil || second == nil {
		return first == nil && second == nil
	}
	return *first == *second
}

func (r *ChatRepository) MarkRead(userID uint, conversationID string, req request.MarkReadRequest) (dto.ReadReceiptDto, error) {
	if _, err := r.requireActiveMember(userID, conversationID); err != nil {
		return dto.ReadReceiptDto{}, err
	}

	var targetExists bool
	if err := r.db.Raw(`
		SELECT EXISTS (
			SELECT 1
			FROM messages
			WHERE id = ? AND conversation_id = ?
		)
	`, req.ThroughMessageID, conversationID).Scan(&targetExists).Error; err != nil {
		return dto.ReadReceiptDto{}, err
	}
	if !targetExists {
		return dto.ReadReceiptDto{}, gorm.ErrRecordNotFound
	}

	var senderIDs []uint
	if err := r.db.Raw(`
		SELECT DISTINCT sender_id
		FROM messages
		WHERE conversation_id = ?
		  AND sender_id <> ?
		  AND deleted_at IS NULL
		  AND (created_at, id) <= (
			SELECT created_at, id FROM messages WHERE id = ? AND conversation_id = ?
		  )
	`, conversationID, userID, req.ThroughMessageID, conversationID).Scan(&senderIDs).Error; err != nil {
		return dto.ReadReceiptDto{}, err
	}

	result := r.db.Exec(`
		INSERT INTO messages_receipts (message_id, user_id, delivered_at, read_at)
		SELECT id, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
		FROM messages
		WHERE conversation_id = ?
		  AND sender_id <> ?
		  AND deleted_at IS NULL
		  AND (created_at, id) <= (
			SELECT created_at, id FROM messages WHERE id = ? AND conversation_id = ?
		  )
		ON CONFLICT (message_id, user_id)
		DO UPDATE SET
			delivered_at = LEAST(messages_receipts.delivered_at, EXCLUDED.delivered_at),
			read_at = COALESCE(messages_receipts.read_at, EXCLUDED.read_at)
	`, userID, conversationID, userID, req.ThroughMessageID, conversationID)
	if result.Error != nil {
		return dto.ReadReceiptDto{}, result.Error
	}

	return dto.ReadReceiptDto{
		ConversationID:   conversationID,
		ThroughMessageID: req.ThroughMessageID,
		MarkedCount:      result.RowsAffected,
		SenderIDs:        senderIDs,
	}, nil
}

func (r *ChatRepository) MarkDelivered(userID uint, conversationID string, req request.MarkReadRequest) (dto.ReadReceiptDto, error) {
	if _, err := r.requireActiveMember(userID, conversationID); err != nil {
		return dto.ReadReceiptDto{}, err
	}

	var targetExists bool
	if err := r.db.Raw(`
		SELECT EXISTS (
			SELECT 1
			FROM messages
			WHERE id = ? AND conversation_id = ?
		)
	`, req.ThroughMessageID, conversationID).Scan(&targetExists).Error; err != nil {
		return dto.ReadReceiptDto{}, err
	}
	if !targetExists {
		return dto.ReadReceiptDto{}, gorm.ErrRecordNotFound
	}

	result := r.db.Exec(`
		INSERT INTO messages_receipts (message_id, user_id, delivered_at)
		SELECT id, ?, CURRENT_TIMESTAMP
		FROM messages
		WHERE conversation_id = ?
		  AND sender_id <> ?
		  AND deleted_at IS NULL
		  AND (created_at, id) <= (
			SELECT created_at, id FROM messages WHERE id = ? AND conversation_id = ?
		  )
		ON CONFLICT (message_id, user_id)
		DO UPDATE SET delivered_at = LEAST(messages_receipts.delivered_at, EXCLUDED.delivered_at)
	`, userID, conversationID, userID, req.ThroughMessageID, conversationID)
	if result.Error != nil {
		return dto.ReadReceiptDto{}, result.Error
	}
	return dto.ReadReceiptDto{
		ConversationID:   conversationID,
		ThroughMessageID: req.ThroughMessageID,
		MarkedCount:      result.RowsAffected,
	}, nil
}

func (r *ChatRepository) EditMessage(userID uint, messageID string, req request.EditMessageRequest) (dto.MessageDto, error) {
	if err := r.requireMessageAccess(userID, messageID); err != nil {
		return dto.MessageDto{}, err
	}
	if err := r.rejectEncryptedMutation(userID, messageID); err != nil {
		return dto.MessageDto{}, err
	}
	result := r.db.Table("messages").
		Where("id = ? AND sender_id = ? AND deleted_at IS NULL", messageID, userID).
		Updates(map[string]interface{}{
			"body":      req.Body,
			"edited_at": gorm.Expr("CURRENT_TIMESTAMP"),
		})
	if result.Error != nil {
		return dto.MessageDto{}, result.Error
	}
	if result.RowsAffected == 0 {
		return dto.MessageDto{}, r.messageMutationError(userID, messageID)
	}

	var message dto.MessageDto
	err := messageDTOQuery(r.db, userID).Where("messages.id = ?", messageID).Take(&message).Error
	finalizeMessage(&message)
	return message, err
}

func (r *ChatRepository) DeleteMessage(userID uint, messageID string) (dto.MessageDto, error) {
	if err := r.requireMessageAccess(userID, messageID); err != nil {
		return dto.MessageDto{}, err
	}
	result := r.db.Table("messages").
		Where("id = ? AND sender_id = ? AND deleted_at IS NULL", messageID, userID).
		Updates(map[string]interface{}{
			"body":               "",
			"ciphertext":         nil,
			"iv":                 nil,
			"encryption_version": nil,
			"key_version":        nil,
			"deleted_at":         gorm.Expr("CURRENT_TIMESTAMP"),
		})
	if result.Error != nil {
		return dto.MessageDto{}, result.Error
	}
	if result.RowsAffected == 0 {
		return dto.MessageDto{}, r.messageMutationError(userID, messageID)
	}

	var message dto.MessageDto
	err := messageDTOQuery(r.db, userID).Where("messages.id = ?", messageID).Take(&message).Error
	finalizeMessage(&message)
	return message, err
}

func (r *ChatRepository) rejectEncryptedMutation(userID uint, messageID string) error {
	var state models.MessageMutationState
	err := r.db.Table("messages").
		Select("sender_id, encryption_version, deleted_at").
		Where("id = ?", messageID).
		Take(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return gorm.ErrRecordNotFound
	}
	if err != nil {
		return err
	}
	if state.EncryptionVersion != nil {
		return constant.ErrChatForbidden
	}
	return nil
}

func (r *ChatRepository) InviteGroupMember(userID uint, conversationID string, req request.InviteGroupMemberRequest) (dto.GroupInvitationDto, error) {
	var invitation dto.GroupInvitationDto
	err := r.db.Transaction(func(tx *gorm.DB) error {
		role, err := activeGroupRole(tx, userID, conversationID)
		if err != nil {
			return err
		}
		if role != "owner" && role != "admin" {
			return constant.ErrChatForbidden
		}
		if userID == req.UserID || uint64(req.UserID) > math.MaxInt64 {
			return constant.ErrChatInvalidRequest
		}

		low, high := orderedUsers(userID, req.UserID)
		var isContact bool
		if err := tx.Raw(`
			SELECT EXISTS (
				SELECT 1 FROM contacts
				WHERE user_low = ? AND user_high = ? AND status = 'accepted'
			)
		`, low, high).Scan(&isContact).Error; err != nil {
			return err
		}
		if !isContact {
			return constant.ErrChatNotContact
		}

		var activeMember bool
		if err := tx.Raw(`
			SELECT EXISTS (
				SELECT 1 FROM conversations_members
				WHERE conversation_id = ? AND user_id = ? AND left_at IS NULL
			)
		`, conversationID, req.UserID).Scan(&activeMember).Error; err != nil {
			return err
		}
		if activeMember {
			return constant.ErrChatMemberExists
		}

		var invitationID string
		err = tx.Raw(`
			INSERT INTO group_invitations (conversation_id, inviter_id, invitee_id)
			VALUES (?, ?, ?)
			RETURNING id
		`, conversationID, userID, req.UserID).Scan(&invitationID).Error
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "idx_group_invitations_one_pending" {
				return constant.ErrGroupInvitationPending
			}
			return err
		}

		return tx.Table("group_invitations AS invitation").
			Select(`
				invitation.id,
				invitation.conversation_id,
				conversation.title AS conversation_title,
				invitation.inviter_id,
				inviter.username AS inviter_username,
				invitation.invitee_id,
				invitation.status,
				invitation.created_at
			`).
			Joins("JOIN conversations AS conversation ON conversation.id = invitation.conversation_id").
			Joins("JOIN users AS inviter ON inviter.id = invitation.inviter_id").
			Where("invitation.id = ?", invitationID).
			Take(&invitation).Error
	})
	return invitation, err
}

func (r *ChatRepository) ListGroupInvitations(userID uint) ([]dto.GroupInvitationDto, error) {
	var invitations []dto.GroupInvitationDto
	err := r.db.Table("group_invitations AS invitation").
		Select(`
			invitation.id,
			invitation.conversation_id,
			conversation.title AS conversation_title,
			invitation.inviter_id,
			inviter.username AS inviter_username,
			invitation.invitee_id,
			invitation.status,
			invitation.created_at
		`).
		Joins("JOIN conversations AS conversation ON conversation.id = invitation.conversation_id").
		Joins("JOIN users AS inviter ON inviter.id = invitation.inviter_id").
		Where("invitation.invitee_id = ? AND invitation.status = 'pending'", userID).
		Order("invitation.created_at DESC").
		Find(&invitations).Error
	return invitations, err
}

func (r *ChatRepository) RespondToGroupInvitation(userID uint, invitationID string, accept bool) (dto.MembershipDto, error) {
	var result dto.MembershipDto
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var invitation models.GroupInvitationReference
		err := tx.Raw(`
			SELECT id, conversation_id, inviter_id, invitee_id
			FROM group_invitations
			WHERE id = ? AND invitee_id = ? AND status = 'pending'
			FOR UPDATE
		`, invitationID, userID).Scan(&invitation).Error
		if err != nil {
			return err
		}
		if invitation.ID == "" {
			return constant.ErrGroupInvitationInvalid
		}

		role, err := activeGroupRole(tx, invitation.InviterID, invitation.ConversationID)
		if err != nil || (role != "owner" && role != "admin") {
			if err != nil {
				return err
			}
			return constant.ErrGroupInvitationInvalid
		}

		if accept {
			low, high := orderedUsers(userID, invitation.InviterID)
			var isContact bool
			if err := tx.Raw(`
				SELECT EXISTS (
					SELECT 1 FROM contacts
					WHERE user_low = ? AND user_high = ? AND status = 'accepted'
				)
			`, low, high).Scan(&isContact).Error; err != nil {
				return err
			}
			if !isContact {
				return constant.ErrChatNotContact
			}

			var activeMember bool
			if err := tx.Raw(`
				SELECT EXISTS (
					SELECT 1 FROM conversations_members
					WHERE conversation_id = ? AND user_id = ? AND left_at IS NULL
				)
			`, invitation.ConversationID, userID).Scan(&activeMember).Error; err != nil {
				return err
			}
			if activeMember {
				return constant.ErrChatMemberExists
			}

			if err := tx.Exec(`
				INSERT INTO conversations_members (conversation_id, user_id, role)
				VALUES (?, ?, 'member')
				ON CONFLICT (conversation_id, user_id)
				DO UPDATE SET role = 'member', joined_at = CURRENT_TIMESTAMP, left_at = NULL
			`, invitation.ConversationID, userID).Error; err != nil {
				return err
			}
		}

		status := "declined"
		if accept {
			status = "accepted"
		}
		if err := tx.Exec(`
			UPDATE group_invitations
			SET status = ?, responded_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, status, invitationID).Error; err != nil {
			return err
		}
		result = dto.MembershipDto{
			ConversationID: invitation.ConversationID,
			UserID:         userID,
			Status:         status,
		}
		return nil
	})
	if err != nil {
		return dto.MembershipDto{}, err
	}
	return result, nil
}

func (r *ChatRepository) RemoveMember(userID uint, conversationID string, memberID uint) (dto.MembershipDto, error) {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		actorRole, err := activeGroupRole(tx, userID, conversationID)
		if err != nil {
			return err
		}

		var target models.ConversationMemberRole
		err = tx.Table("conversations_members").
			Select("role").
			Where("conversation_id = ? AND user_id = ? AND left_at IS NULL", conversationID, memberID).
			Take(&target).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return gorm.ErrRecordNotFound
		}
		if err != nil {
			return err
		}
		if target.Role == "owner" {
			return constant.ErrChatOwnerCannotLeave
		}
		if memberID != userID {
			if actorRole != "owner" && actorRole != "admin" {
				return constant.ErrChatForbidden
			}
			if actorRole == "admin" && target.Role != "member" {
				return constant.ErrChatForbidden
			}
		}

		return tx.Exec(`
			UPDATE conversations_members
			SET left_at = CURRENT_TIMESTAMP
			WHERE conversation_id = ? AND user_id = ? AND left_at IS NULL
		`, conversationID, memberID).Error
	})
	if err != nil {
		return dto.MembershipDto{}, err
	}
	return dto.MembershipDto{ConversationID: conversationID, UserID: memberID, Status: "left"}, nil
}

func activeGroupRole(tx *gorm.DB, userID uint, conversationID string) (string, error) {
	var result models.ConversationRole
	err := tx.Table("conversations_members AS member").
		Select("conversation.type, member.role").
		Joins("JOIN conversations AS conversation ON conversation.id = member.conversation_id").
		Where("member.conversation_id = ? AND member.user_id = ? AND member.left_at IS NULL", conversationID, userID).
		Take(&result).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", constant.ErrChatForbidden
	}
	if err != nil {
		return "", err
	}
	if result.Type != "group" {
		return "", constant.ErrChatInvalidRequest
	}
	return result.Role, nil
}

func (r *ChatRepository) messageMutationError(userID uint, messageID string) error {
	var message models.MessageMutationState
	err := r.db.Table("messages").
		Select("sender_id, deleted_at").
		Where("id = ?", messageID).
		Take(&message).Error
	if err != nil {
		return err
	}
	if message.SenderID != int64(userID) {
		return constant.ErrChatForbidden
	}
	if message.DeletedAt != nil {
		return constant.ErrChatMessageDeleted
	}
	return gorm.ErrRecordNotFound
}

func (r *ChatRepository) requireMessageAccess(userID uint, messageID string) error {
	var message models.MessageConversationReference
	err := r.db.Table("messages").
		Select("conversation_id").
		Where("id = ?", messageID).
		Take(&message).Error
	if err != nil {
		return err
	}
	_, err = r.requireActiveMember(userID, message.ConversationID)
	return err
}

func (r *ChatRepository) requireActiveMember(userID uint, conversationID string) (*models.ConversationAccess, error) {
	var membership models.ConversationAccess
	err := r.db.Table("conversations_members AS member").
		Select(`
			conversation.type,
			CASE
				WHEN conversation.type = 'group' THEN TRUE
				ELSE EXISTS (
					SELECT 1
					FROM contacts
					WHERE user_low = conversation.direct_user_low
					  AND user_high = conversation.direct_user_high
					  AND status = 'accepted'
				)
			END AS contact_is_valid
		`).
		Joins("JOIN conversations AS conversation ON conversation.id = member.conversation_id").
		Where("member.conversation_id = ? AND member.user_id = ? AND member.left_at IS NULL", conversationID, userID).
		Take(&membership).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, constant.ErrChatForbidden
	}
	if err == nil && membership.Type == "direct" && !membership.ContactIsValid {
		return nil, constant.ErrChatNotContact
	}
	if err != nil {
		return nil, err
	}
	return &membership, nil
}

func (r *ChatRepository) ActiveMemberIDs(conversationID string) ([]uint, error) {
	var memberIDs []uint
	err := r.db.Table("conversations_members AS member").
		Select("member.user_id").
		Joins("JOIN conversations AS conversation ON conversation.id = member.conversation_id").
		Where(`
			member.conversation_id = ?
			AND member.left_at IS NULL
			AND (
				conversation.type = 'group'
				OR EXISTS (
					SELECT 1 FROM contacts
					WHERE user_low = conversation.direct_user_low
					  AND user_high = conversation.direct_user_high
					  AND status = 'accepted'
				)
			)
		`, conversationID).
		Find(&memberIDs).Error
	return memberIDs, err
}

func (r *ChatRepository) UploadGroupKeys(userID uint, conversationID string, req request.UploadGroupKeysRequest) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var conv models.ConversationRole
		err := tx.Table("conversations").Select("type").Where("id = ?", conversationID).Take(&conv).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return gorm.ErrRecordNotFound
		}
		if err != nil {
			return err
		}
		if conv.Type != "group" {
			return constant.ErrChatInvalidRequest
		}

		var memberRole string
		err = tx.Table("conversations_members").
			Select("role").
			Where("conversation_id = ? AND user_id = ? AND left_at IS NULL", conversationID, userID).
			Take(&memberRole).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return constant.ErrChatForbidden
		}
		if err != nil {
			return err
		}

		// Rotation may only advance the epoch by one so a member cannot poison
		// the group with an arbitrary future key_version (GET /keys/me serves
		// the highest version). Overwriting the current version stays allowed
		// because re-wrapping for a new member reuses the same epoch.
		var currentMaxVersion int
		err = tx.Table("conversation_group_keys").
			Select("COALESCE(MAX(key_version), 0)").
			Where("conversation_id = ?", conversationID).
			Scan(&currentMaxVersion).Error
		if err != nil {
			return err
		}
		if req.KeyVersion > currentMaxVersion+1 {
			return constant.ErrChatKeyVersionConflict
		}

		userIDs := make([]uint, len(req.Keys))
		for i, key := range req.Keys {
			userIDs[i] = key.UserID
		}

		var activeCount int64
		err = tx.Table("conversations_members").
			Where("conversation_id = ? AND user_id IN ? AND left_at IS NULL", conversationID, userIDs).
			Count(&activeCount).Error
		if err != nil {
			return err
		}
		if activeCount != int64(len(userIDs)) {
			return constant.ErrChatInvalidRequest
		}

		for _, item := range req.Keys {
			err := tx.Exec(`
				INSERT INTO conversation_group_keys (
					conversation_id, key_version, user_id, sender_id, encrypted_key, iv, created_at
				) VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
				ON CONFLICT (conversation_id, key_version, user_id)
				DO UPDATE SET
					sender_id = EXCLUDED.sender_id,
					encrypted_key = EXCLUDED.encrypted_key,
					iv = EXCLUDED.iv,
					created_at = CURRENT_TIMESTAMP
			`, conversationID, req.KeyVersion, item.UserID, userID, item.EncryptedKey, item.IV).Error
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *ChatRepository) GetMyGroupKey(userID uint, conversationID string, keyVersion *int) (dto.ConversationGroupKeyDto, error) {
	var conv models.ConversationRole
	err := r.db.Table("conversations").Select("type").Where("id = ?", conversationID).Take(&conv).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return dto.ConversationGroupKeyDto{}, gorm.ErrRecordNotFound
	}
	if err != nil {
		return dto.ConversationGroupKeyDto{}, err
	}
	if conv.Type != "group" {
		return dto.ConversationGroupKeyDto{}, constant.ErrChatInvalidRequest
	}

	var memberRole string
	err = r.db.Table("conversations_members").
		Select("role").
		Where("conversation_id = ? AND user_id = ? AND left_at IS NULL", conversationID, userID).
		Take(&memberRole).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return dto.ConversationGroupKeyDto{}, constant.ErrChatForbidden
	}
	if err != nil {
		return dto.ConversationGroupKeyDto{}, err
	}

	query := r.db.Table("conversation_group_keys AS k").
		Select(`
			k.conversation_id,
			k.key_version,
			k.sender_id,
			COALESCE(u.username, '') AS sender_username,
			k.encrypted_key,
			k.iv,
			k.created_at
		`).
		Joins("LEFT JOIN users AS u ON u.id = k.sender_id").
		Where("k.conversation_id = ? AND k.user_id = ?", conversationID, userID)

	if keyVersion != nil {
		query = query.Where("k.key_version = ?", *keyVersion)
	}
	query = query.Order("k.key_version DESC").Limit(1)

	var result dto.ConversationGroupKeyDto
	err = query.Take(&result).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return dto.ConversationGroupKeyDto{}, gorm.ErrRecordNotFound
	}
	if err != nil {
		return dto.ConversationGroupKeyDto{}, err
	}
	return result, nil
}
