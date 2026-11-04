package controllers_test

import (
	"testing"

	"lechat-backend/internal/constant"
	"lechat-backend/internal/services"
)

func TestContactServiceRejectsInvalidUserPairs(t *testing.T) {
	service := services.NewContactService(nil)
	for _, pair := range [][2]uint{{0, 1}, {1, 0}, {1, 1}} {
		if _, err := service.RequestContact(pair[0], pair[1]); err != constant.ErrChatInvalidRequest {
			t.Errorf("RequestContact(%d, %d) error = %v, want %v", pair[0], pair[1], err, constant.ErrChatInvalidRequest)
		}
	}
}
