package request

type CreateConversationRequest struct {
	Type      string  `json:"type" binding:"required,oneof=direct group"`
	Title     *string `json:"title"`
	MemberIDs []uint  `json:"member_ids" binding:"required,min=1"`
}

type SendMessageRequest struct {
	ClientMessageID  string  `json:"client_message_id" binding:"required,uuid"`
	Type             string  `json:"type" binding:"required,oneof=text image video file"`
	Body             string  `json:"body"`
	ReplyToMessageID *string `json:"reply_to_message_id"`
}

type MarkReadRequest struct {
	ThroughMessageID string `json:"through_message_id" binding:"required,uuid"`
}

type EditMessageRequest struct {
	Body string `json:"body" binding:"required"`
}

type InviteGroupMemberRequest struct {
	UserID uint `json:"user_id" binding:"required"`
}
