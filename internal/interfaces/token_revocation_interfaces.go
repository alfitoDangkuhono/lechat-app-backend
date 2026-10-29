package interfaces

import "time"

type ITokenRevocationStore interface {
	Revoke(tokenID string, expiresAt time.Time) error
	IsRevoked(tokenID string) (bool, error)
}
