package models

import "time"

type ConversationParticipant struct {
	ConversationID string `gorm:"column:conversation_id"`
	UserID         int64  `gorm:"column:user_id"`
	Username       string `gorm:"column:username"`
	FirstName      string `gorm:"column:first_name"`
	LastName       string `gorm:"column:last_name"`
	Role           string `gorm:"column:role"`
}

type GroupInvitationReference struct {
	ID             string `gorm:"column:id"`
	ConversationID string `gorm:"column:conversation_id"`
	InviterID      uint   `gorm:"column:inviter_id"`
	InviteeID      uint   `gorm:"column:invitee_id"`
}

type ConversationMemberRole struct {
	Role string `gorm:"column:role"`
}

type ConversationRole struct {
	Type string `gorm:"column:type"`
	Role string `gorm:"column:role"`
}

type MessageMutationState struct {
	SenderID  int64      `gorm:"column:sender_id"`
	DeletedAt *time.Time `gorm:"column:deleted_at"`
}

type MessageConversationReference struct {
	ConversationID string `gorm:"column:conversation_id"`
}

type ConversationAccess struct {
	Type           string `gorm:"column:type"`
	ContactIsValid bool   `gorm:"column:contact_is_valid"`
}

type ContactRequestOutcome struct {
	Status      string `gorm:"column:status"`
	RequestedBy uint   `gorm:"column:requested_by"`
}
