package dto

import "time"

type ContactDto struct {
	UserID     int64      `json:"user_id"`
	Username   string     `json:"username"`
	FirstName  string     `json:"first_name"`
	LastName   string     `json:"last_name"`
	Status     string     `json:"status"`
	Direction  string     `json:"direction,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	AcceptedAt *time.Time `json:"accepted_at,omitempty"`
}
