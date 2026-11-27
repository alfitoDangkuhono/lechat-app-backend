package request

type CreateConversationRequest struct {
	Type      string  `json:"type" binding:"required,oneof=direct group"`
	Title     *string `json:"title"`
	MemberIDs []uint  `json:"member_ids" binding:"required,min=1"`
}

type SendMessageRequest struct {
	ClientMessageID   string  `json:"client_message_id" binding:"required,uuid"`
	Type              string  `json:"type" binding:"required,oneof=text image video file"`
	Body              string  `json:"body"`
	EncryptionVersion *int    `json:"encryption_version"`
	KeyVersion        *int    `json:"key_version"`
	Ciphertext        *string `json:"ciphertext"`
	IV                *string `json:"iv"`
	ReplyToMessageID  *string `json:"reply_to_message_id"`
}

type UploadGroupKeysRequest struct {
	KeyVersion int                  `json:"key_version" binding:"required,min=1"`
	Keys       []UploadGroupKeyItem `json:"keys" binding:"required,min=1,max=250,dive"`
}

type UploadGroupKeyItem struct {
	UserID       uint   `json:"user_id" binding:"required"`
	EncryptedKey string `json:"encrypted_key" binding:"required"`
	IV           string `json:"iv" binding:"required"`
}

type MarkReadRequest struct {
	ThroughMessageID string `json:"through_message_id" binding:"required,uuid"`
}

type AckMessagesRequest struct {
	IDs []string `json:"ids" binding:"required,min=1,max=500,dive,uuid"`
}

type EditMessageRequest struct {
	Body string `json:"body" binding:"required"`
}

type InviteGroupMemberRequest struct {
	UserID uint `json:"user_id" binding:"required"`
}
