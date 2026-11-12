package models

import "time"

type Users struct {
	Id        int       `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Password  string    `json:"password"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Phone     string    `json:"phone"`
	About     string    `json:"about"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ConversationMember struct {
	ConversationID string     `gorm:"column:conversation_id;primaryKey"`
	UserID         int64      `gorm:"column:user_id;primaryKey"`
	Role           string     `gorm:"column:role"`
	JoinedAt       time.Time  `gorm:"column:joined_at;default:CURRENT_TIMESTAMP"`
	LeftAt         *time.Time `gorm:"column:left_at"`
}

type Message struct {
	ID               string     `gorm:"column:id;primaryKey;default:gen_random_uuid()"`
	ConversationID   string     `gorm:"column:conversation_id"`
	SenderID         int64      `gorm:"column:sender_id"`
	ClientMessageID  string     `gorm:"column:client_message_id"`
	ReplyToMessageID *string    `gorm:"column:reply_to_message_id"`
	Type             string     `gorm:"column:type"`
	Body             string     `gorm:"column:body"`
	EditedAt         *time.Time `gorm:"column:edited_at"`
	DeletedAt        *time.Time `gorm:"column:deleted_at"`
	CreatedAt        time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt        time.Time  `gorm:"column:updated_at;autoCreateTime;autoUpdateTime"`
}
