package dto

import "time"

type ConversationDto struct {
	ID                 string                  `json:"id"`
	Type               string                  `json:"type"`
	Title              *string                 `json:"title"`
	MemberCount        int64                   `json:"member_count,omitempty"`
	Members            []ConversationMemberDto `json:"members" gorm:"-"`
	CreatedAt          time.Time               `json:"created_at"`
	UpdatedAt          time.Time               `json:"updated_at"`
	LastMessageID      *string                 `json:"last_message_id"`
	LastMessageBody    *string                 `json:"last_message_body"`
	LastMessageType    *string                 `json:"last_message_type"`
	LastMessageAt      *time.Time              `json:"last_message_at"`
	UnreadMessageCount int64                   `json:"unread_message_count"`
}

type ConversationCreationDto struct {
	ID          string  `json:"id"`
	Type        string  `json:"type"`
	Title       *string `json:"title"`
	MemberCount int64   `json:"member_count"`
}

type ConversationMemberDto struct {
	UserID    int64  `json:"user_id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"`
}

type MessageDto struct {
	ID               string           `json:"id"`
	ConversationID   string           `json:"conversation_id"`
	SenderID         int64            `json:"sender_id"`
	ClientMessageID  string           `json:"client_message_id"`
	ReplyToMessageID *string          `json:"reply_to_message_id,omitempty" gorm:"column:reply_to_message_id"`
	ReplyTo          *MessageReplyDto `json:"reply_to,omitempty" gorm:"-"`
	ReplyToSenderID  *int64           `json:"-" gorm:"column:reply_to_sender_id"`
	ReplyToUsername  *string          `json:"-" gorm:"column:reply_to_username"`
	ReplyToBody      *string          `json:"-" gorm:"column:reply_to_body"`
	ReplyToType      *string          `json:"-" gorm:"column:reply_to_type"`
	ReplyToIsDeleted *bool            `json:"-" gorm:"column:reply_to_is_deleted"`
	Type             string           `json:"type"`
	Body             string           `json:"body"`
	EditedAt         *time.Time       `json:"edited_at"`
	DeletedAt        *time.Time       `json:"deleted_at"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
	DeliveredCount   int64            `json:"delivered_count"`
	ReadCount        int64            `json:"read_count"`
	ReceiptStatus    string           `json:"receipt_status,omitempty"`
}

type MessageReplyDto struct {
	ID        string `json:"id"`
	SenderID  int64  `json:"sender_id"`
	Username  string `json:"username"`
	Body      string `json:"body"`
	Type      string `json:"type"`
	IsDeleted bool   `json:"is_deleted"`
}

type ReadReceiptDto struct {
	ConversationID   string `json:"conversation_id"`
	ThroughMessageID string `json:"through_message_id"`
	MarkedCount      int64  `json:"marked_count"`
	SenderIDs        []uint `json:"-"`
}

type MembershipDto struct {
	ConversationID string `json:"conversation_id"`
	UserID         uint   `json:"user_id"`
	Status         string `json:"status"`
}

type GroupInvitationDto struct {
	ID                string    `json:"id"`
	ConversationID    string    `json:"conversation_id"`
	ConversationTitle *string   `json:"conversation_title"`
	InviterID         int64     `json:"inviter_id"`
	InviterUsername   string    `json:"inviter_username"`
	InviteeID         int64     `json:"invitee_id"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"created_at"`
}
