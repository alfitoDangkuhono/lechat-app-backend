package request

type UpdatePublicKeysRequest struct {
	PublicKey string `json:"public_key" binding:"required"`
}
