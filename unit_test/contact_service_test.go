package controllers_test

import (
	"testing"

	"lechat-backend/internal/constant"
	"lechat-backend/internal/services"
)

func TestContactServiceRejectsInvalidPhoneRequests(t *testing.T) {
	service := services.NewContactService(nil)
	for _, phone := range []string{"", "123456789"} {
		if _, _, err := service.RequestContactByPhone(1, phone); err != constant.ErrChatInvalidRequest {
			t.Errorf("RequestContactByPhone(%q) error = %v, want %v", phone, err, constant.ErrChatInvalidRequest)
		}
	}
	if _, _, err := service.RequestContactByPhone(0, "1234567890"); err != constant.ErrChatInvalidRequest {
		t.Errorf("RequestContactByPhone with zero user ID error = %v, want %v", err, constant.ErrChatInvalidRequest)
	}
}
