package controllers_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lechat-backend/internal/api/controllers"
	"lechat-backend/internal/api/routes"
	"lechat-backend/internal/constant"
	"lechat-backend/internal/database/dto"
	"lechat-backend/internal/database/models"
	"lechat-backend/internal/interfaces"
	"lechat-backend/internal/services"

	"github.com/gin-gonic/gin"
)

type contactStub struct {
	// request contact by phone
	requestContactId    uint
	requestContactName  string
	requestContactErr   error
	requestContactCalls int
	requestUserId       uint
	requestPhon         string

	// accept contact /reject contact/ remove contact
	acceptErr, rejectErr, removeErr       error
	acceptCalls, rejectCalls, removeCalls int
	lastUserId, lastTargetId              uint

	// list contacts /list contact requests
	contacts         []dto.ContactDto
	contactsErr      error
	contactRequest   []dto.ContactDto
	contactReqErr    error
	listCalls        int
	listRequestCalss int

	// find contact public keys
	findContactPublicKeysCalls int
	targetUserId               uint
	publicKeysErr              error
	publicKeyDto               *dto.PublicKeyDto
}

var _ interfaces.IContacts = (*contactStub)(nil)

func (s *contactStub) RequestContactByPhone(userId uint, phone string) (uint, string, error) {
	s.requestContactCalls++
	s.requestUserId, s.requestPhon = userId, phone
	return s.requestContactId, s.requestContactName, s.requestContactErr
}

func (s *contactStub) AcceptContact(userId, targetId uint) error {
	s.acceptCalls++
	s.lastUserId, s.lastTargetId = userId, targetId
	return s.acceptErr
}

func (s *contactStub) RejectContact(userId, targetId uint) error {
	s.rejectCalls++
	s.lastUserId, s.lastTargetId = userId, targetId
	return s.rejectErr
}

func (s *contactStub) RemoveContact(userId, targetId uint) error {
	s.removeCalls++
	s.lastUserId, s.lastTargetId = userId, targetId
	return s.removeErr
}
func (s *contactStub) ListContacts(userId uint) ([]dto.ContactDto, error) {
	s.listRequestCalss++
	s.lastUserId = userId
	return s.contacts, s.contactsErr
}
func (s *contactStub) ListContactRequests(userId uint) ([]dto.ContactDto, error) {
	s.listRequestCalss++
	s.lastUserId = userId
	return s.contactRequest, s.contactReqErr
}
func (s *contactStub) FindContactPublicKeys(targetId uint) (*dto.PublicKeyDto, error) {
	s.findContactPublicKeysCalls++
	s.targetUserId = targetId
	return s.publicKeyDto, s.publicKeysErr
}

// Helper
func getPublicKeyContacts(contact *contactStub, jwtService *jwtStub, userId, authorization string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	routes.ContactRoutes(router, jwtService, controllers.NewContactController(contact))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/contacts/"+userId+"/keys", nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// unit test
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

func TestFindContactPublicKeys(t *testing.T) {

	tests := []struct {
		name         string
		userId       string
		stubDto      *dto.PublicKeyDto
		stubErr      error
		wantStatus   int
		wantCalls    int
		wantTargetId uint
		wantBody     string
	}{
		{
			name:         "success",
			userId:       "7",
			stubDto:      &dto.PublicKeyDto{PublicKey: "abs"},
			wantStatus:   http.StatusOK,
			wantCalls:    1,
			wantTargetId: 7,
			wantBody:     "abs",
		},
		{
			name:         "service error return 404",
			userId:       "7",
			stubErr:      constant.ErrNotFoundData,
			wantStatus:   http.StatusNotFound,
			wantCalls:    1,
			wantTargetId: 7,
		},
		{
			name:         "any other service error alse return 404 (perilaku controller saat ini)",
			userId:       "7",
			stubErr:      errors.New("db down"),
			wantStatus:   http.StatusNotFound,
			wantCalls:    1,
			wantTargetId: 7,
		},
		{
			name:       "id not a number",
			userId:     "abc",
			wantStatus: http.StatusBadRequest,
			wantCalls:  0,
		},
		{
			name:       "id zero",
			userId:     "0",
			wantStatus: http.StatusBadRequest,
			wantCalls:  0,
		},
		{
			name:       "id negative",
			userId:     "-1",
			wantStatus: http.StatusBadRequest,
			wantCalls:  0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contact := &contactStub{publicKeyDto: tt.stubDto, publicKeysErr: tt.stubErr}
			jwtService := &jwtStub{claims: &models.JwtClaims{UserId: 42}}

			rec := getPublicKeyContacts(contact, jwtService, tt.userId, "Bearer token-valid")

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, rec.Code, rec.Body.String())
			}
			if contact.findContactPublicKeysCalls != tt.wantCalls {
				t.Fatalf("expected %d service calls, got %d", tt.wantCalls, contact.targetUserId)
			}
			if tt.wantCalls > 0 && contact.targetUserId != tt.wantTargetId {
				t.Fatalf("expected target id %d, got %s", tt.wantTargetId, rec.Body.String())
			}
			if tt.wantBody != "" && !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Fatalf("expected body to contains %q, got %s", tt.wantBody, rec.Body.String())
			}
		})
	}
}
