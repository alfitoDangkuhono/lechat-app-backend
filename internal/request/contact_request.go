package request

type ContactRequest struct {
	Phone string `json:"phone" binding:"required,min=10"`
}
