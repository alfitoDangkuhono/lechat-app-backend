package request

type ProfileRequest struct {
	FirstName string `json:"first_name" binding:"required"`
	LastName string `json:"last_name" binding:"required"`
	Phone string `json:"phone" binding:"required"`
	About string `json:"about" binding:"required"`
}