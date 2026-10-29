package constant

import "errors"

var (
	ErrNotFoundEmail       = errors.New("email not found!")
	ErrFailedCreateToken   = errors.New("failed create token!")
	ErrIncorectPassword    = errors.New("password incorrect!")
	ErrNotFoundData        = errors.New("not found data")
	ErrInvalidHeadersHttp  = errors.New("any invalid headers http")
	ErrDeviceBlocked       = errors.New("device blocked!")
	ErrToManyRequest       = errors.New("too many attempts, try again later")
	ErrInternalServer      = errors.New("internal server error")
	ErrCreateMessage       = errors.New("error creating messages")
	ErrInvalidToken        = errors.New("invalid or expired token")
	ErrTokenRevoked        = errors.New("token has been revoked")
	ErrTokenRevocationDB   = errors.New("failed to check token revocation")
	ErrInvalidRefreshToken = errors.New("invalid or expired refresh token")
)
