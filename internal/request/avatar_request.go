package request

type AvatarUploadURLRequest struct {
	ContentType string `json:"content_type" binding:"required"`
}
