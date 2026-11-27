package dto

import "time"

type PublicKeyDto struct {
	PublicKey string    `json:"public_key"`
	UpdatedAt time.Time `json:"updated_at"`
}
