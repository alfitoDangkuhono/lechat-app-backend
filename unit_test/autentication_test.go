package controllers_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"lechat-backend/config"
	"lechat-backend/internal/api/controllers"
	"lechat-backend/internal/api/routes"
	"lechat-backend/internal/constant"
	"lechat-backend/internal/database/dto"
	"lechat-backend/internal/database/models"
	"lechat-backend/internal/helpers"
	"lechat-backend/internal/request"
	"lechat-backend/internal/services"
	"lechat-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

type authenticationStub struct {
	user          models.Users
	err           error
	calls         int
	req           request.AddUserRequest
	login         dto.LoginDto
	loginErr      error
	loginCalls    int
	loginEmail    string
	loginPassword string
	profile       dto.ProfileDto
	profileErr    error
	profileCalls  int
	profileUserID uint
}

func (s *authenticationStub) Registration(req request.AddUserRequest) (models.Users, error) {
	s.calls++
	s.req = req
	return s.user, s.err
}

func (s *authenticationStub) CredentialLogin(email string, password string) (dto.LoginDto, error) {
	s.loginCalls++
	s.loginEmail = email
	s.loginPassword = password
	return s.login, s.loginErr
}

func (s *authenticationStub) GetProfile(userID uint) (dto.ProfileDto, error) {
	s.profileCalls++
	s.profileUserID = userID
	return s.profile, s.profileErr
}

type jwtStub struct {
	token        string
	err          error
	calls        int
	revokeCalls  int
	userID       uint
	username     string
	email        string
	refreshFlag  bool
	claims       *models.JwtClaims
	validateErr  error
	validateCall int
	validated    string
	revokeErr    error
	revokedID    string
	revokedAt    time.Time
	revoked      bool
}

type tokenRevocationStoreStub struct {
	revoked map[string]bool
	err     error
}

func (s *tokenRevocationStoreStub) Revoke(tokenID string, _ time.Time) error {
	if s.err != nil {
		return s.err
	}
	s.revoked[tokenID] = true
	return nil
}

func (s *tokenRevocationStoreStub) IsRevoked(tokenID string) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.revoked[tokenID], nil
}

func (s *jwtStub) GenerateToken(userID uint, username string, email string, isRefreshToken bool) (string, error) {
	s.calls++
	s.userID = userID
	s.username = username
	s.email = email
	s.refreshFlag = isRefreshToken
	if s.err != nil {
		return "", s.err
	}
	if s.token != "" {
		return s.token, nil
	}
	if isRefreshToken {
		return "refresh-token", nil
	}
	return "signed-token", nil
}

func (s *jwtStub) ValidateToken(token string) (*models.JwtClaims, error) {
	s.validateCall++
	s.validated = token
	if s.validateErr != nil {
		return nil, s.validateErr
	}
	if s.revoked {
		return nil, errors.New("token revoked")
	}
	return s.claims, nil
}

func (s *jwtStub) RevokeToken(tokenID string, expiresAt time.Time) error {
	s.revokeCalls++
	s.revokedID = tokenID
	s.revokedAt = expiresAt
	if s.revokeErr != nil {
		return s.revokeErr
	}
	s.revoked = true
	return nil
}

func registerRequest(body string, auth *authenticationStub, jwt *jwtStub) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	routes.Routes(router, controllers.NewAuthenticationController(auth, jwt), jwt)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/registers", bytes.NewBufferString(body)))
	return recorder
}

func loginRequest(body string, auth *authenticationStub, jwt *jwtStub) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	routes.Routes(router, controllers.NewAuthenticationController(auth, jwt), jwt)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBufferString(body)))
	return recorder
}

func refreshRequest(auth *authenticationStub, jwt *jwtStub, authorization string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	routes.Routes(router, controllers.NewAuthenticationController(auth, jwt), jwt)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/refresh", nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	router.ServeHTTP(recorder, req)
	return recorder
}

func logoutRequest(auth *authenticationStub, jwt *jwtStub, authorization string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	routes.Routes(router, controllers.NewAuthenticationController(auth, jwt), jwt)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions", nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	router.ServeHTTP(recorder, req)
	return recorder
}

func profileRequest(auth *authenticationStub, jwt *jwtStub, authorization string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	routes.Routes(router, controllers.NewAuthenticationController(auth, jwt), jwt)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	router.ServeHTTP(recorder, req)
	return recorder
}

func checkSessionRequest(jwt *jwtStub, authorization string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	routes.Routes(router, controllers.NewAuthenticationController(&authenticationStub{}, jwt), jwt)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/check-session", nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestRegisterSuccess(t *testing.T) {
	auth := &authenticationStub{
		user: models.Users{
			Id:       42,
			Username: "jane",
			Email:    "jane@example.com",
		},
	}
	jwt := &jwtStub{}

	recorder := registerRequest(
		`{"username":"jane","email":"jane@example.com","password":"secret1","phone":"08123456789"}`,
		auth,
		jwt,
	)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, recorder.Code, recorder.Body.String())
	}

	var response utils.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected response data object, got %#v", response.Data)
	}
	if response.Code != http.StatusCreated || response.Message != "success" || data["token"] != "signed-token" || data["refresh_token"] != "refresh-token" {
		t.Errorf("unexpected response: %#v", response)
	}

	if auth.calls != 1 {
		t.Errorf("expected registration to be called once, got %d", auth.calls)
	}
	if auth.req != (request.AddUserRequest{
		Username: "jane",
		Email:    "jane@example.com",
		Password: "secret1",
		Phone:    "08123456789",
	}) {
		t.Errorf("unexpected registration request: %#v", auth.req)
	}
	if jwt.calls != 2 || jwt.userID != 42 || jwt.username != "jane" || jwt.email != "jane@example.com" || !jwt.refreshFlag {
		t.Errorf("unexpected token generation arguments: %#v", jwt)
	}
}

func TestRegisterRejectsInvalidRequest(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "invalid JSON",
			body: `{"username":`,
		},
		{
			name: "missing required fields",
			body: `{"username":"jane"}`,
		},
		{
			name: "invalid email",
			body: `{"username":"jane","email":"not-an-email","password":"secret1","phone":"08123456789"}`,
		},
		{
			name: "password too short",
			body: `{"username":"jane","email":"jane@example.com","password":"123","phone":"08123456789"}`,
		},
		{
			name: "phone too short",
			body: `{"username":"jane","email":"jane@example.com","password":"secret1","phone":"123"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := &authenticationStub{}
			jwt := &jwtStub{}

			recorder := registerRequest(tt.body, auth, jwt)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
			}
			if auth.calls != 0 {
				t.Errorf("registration should not be called for invalid input, got %d calls", auth.calls)
			}
			if jwt.calls != 0 {
				t.Errorf("token generation should not be called for invalid input, got %d calls", jwt.calls)
			}
		})
	}
}

func TestRegisterReturnsErrorWhenRegistrationFails(t *testing.T) {
	auth := &authenticationStub{err: errors.New("database unavailable")}
	jwt := &jwtStub{}

	recorder := registerRequest(
		`{"username":"jane","email":"jane@example.com","password":"secret1","phone":"08123456789"}`,
		auth,
		jwt,
	)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d: %s", http.StatusInternalServerError, recorder.Code, recorder.Body.String())
	}
	if auth.calls != 1 {
		t.Errorf("expected registration to be called once, got %d", auth.calls)
	}
	if jwt.calls != 0 {
		t.Errorf("token generation should not be called after registration fails, got %d calls", jwt.calls)
	}
}

func TestRegisterReturnsErrorWhenTokenGenerationFails(t *testing.T) {
	auth := &authenticationStub{
		user: models.Users{
			Id:       42,
			Username: "jane",
			Email:    "jane@example.com",
		},
	}
	jwt := &jwtStub{err: errors.New("signing failed")}

	recorder := registerRequest(
		`{"username":"jane","email":"jane@example.com","password":"secret1","phone":"08123456789"}`,
		auth,
		jwt,
	)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d: %s", http.StatusInternalServerError, recorder.Code, recorder.Body.String())
	}
	if auth.calls != 1 || jwt.calls != 1 {
		t.Errorf("expected registration and token generation once each, got %d and %d", auth.calls, jwt.calls)
	}
	if bytes.Contains(recorder.Body.Bytes(), []byte(`"message":"success"`)) {
		t.Errorf("error response must not be followed by a success response: %s", recorder.Body.String())
	}
}

func TestLoginSuccess(t *testing.T) {
	auth := &authenticationStub{
		login: dto.LoginDto{
			Id:       42,
			Username: "jane",
			Email:    "jane@example.com",
		},
	}
	jwt := &jwtStub{}

	recorder := loginRequest(
		`{"email":"jane@example.com","password":"secret1"}`,
		auth,
		jwt,
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var response utils.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected response data object, got %#v", response.Data)
	}
	if response.Code != http.StatusOK || response.Message != "success" || data["token"] != "signed-token" || data["refresh_token"] != "refresh-token" {
		t.Errorf("unexpected response: %#v", response)
	}
	if auth.loginCalls != 1 || auth.loginEmail != "jane@example.com" || auth.loginPassword != "secret1" {
		t.Errorf("unexpected credential login call: %#v", auth)
	}
	if jwt.calls != 2 || jwt.userID != 42 || jwt.username != "jane" || jwt.email != "jane@example.com" || !jwt.refreshFlag {
		t.Errorf("unexpected token generation arguments: %#v", jwt)
	}
}

func TestLoginRejectsInvalidRequest(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "invalid JSON",
			body: `{"email":`,
		},
		{
			name: "missing fields",
			body: `{}`,
		},
		{
			name: "invalid email",
			body: `{"email":"not-an-email","password":"secret1"}`,
		},
		{
			name: "password too short",
			body: `{"email":"jane@example.com","password":"123"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := &authenticationStub{}
			jwt := &jwtStub{}

			recorder := loginRequest(tt.body, auth, jwt)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
			}
			if auth.loginCalls != 0 {
				t.Errorf("credential login should not be called for invalid input, got %d calls", auth.loginCalls)
			}
			if jwt.calls != 0 {
				t.Errorf("token generation should not be called for invalid input, got %d calls", jwt.calls)
			}
		})
	}
}

func TestLoginReturnsNotFoundWhenEmailDoesNotExist(t *testing.T) {
	auth := &authenticationStub{loginErr: errors.New("email_incorrect")}
	jwt := &jwtStub{}

	recorder := loginRequest(`{"email":"jane@example.com","password":"secret1"}`, auth, jwt)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, recorder.Code, recorder.Body.String())
	}
	if auth.loginCalls != 1 {
		t.Errorf("expected credential login to be called once, got %d", auth.loginCalls)
	}
	if jwt.calls != 0 {
		t.Errorf("token generation should not be called after credential login fails, got %d calls", jwt.calls)
	}
	if bytes.Contains(recorder.Body.Bytes(), []byte(`"message":"success"`)) {
		t.Errorf("error response must not be followed by a success response: %s", recorder.Body.String())
	}
}

func TestLoginReturnsErrorWhenPasswordIsIncorrect(t *testing.T) {
	auth := &authenticationStub{loginErr: errors.New("password_inccorrect")}
	jwt := &jwtStub{}

	recorder := loginRequest(`{"email":"jane@example.com","password":"wrongpass"}`, auth, jwt)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d: %s", http.StatusInternalServerError, recorder.Code, recorder.Body.String())
	}
	if auth.loginCalls != 1 {
		t.Errorf("expected credential login to be called once, got %d", auth.loginCalls)
	}
	if jwt.calls != 0 {
		t.Errorf("token generation should not be called after credential login fails, got %d calls", jwt.calls)
	}
}

func TestLoginReturnsErrorWhenTokenGenerationFails(t *testing.T) {
	auth := &authenticationStub{
		login: dto.LoginDto{
			Id:       42,
			Username: "jane",
			Email:    "jane@example.com",
		},
	}
	jwt := &jwtStub{err: errors.New("signing failed")}

	recorder := loginRequest(`{"email":"jane@example.com","password":"secret1"}`, auth, jwt)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d: %s", http.StatusInternalServerError, recorder.Code, recorder.Body.String())
	}
	if auth.loginCalls != 1 || jwt.calls != 1 {
		t.Errorf("expected credential login and token generation once each, got %d and %d", auth.loginCalls, jwt.calls)
	}
	if bytes.Contains(recorder.Body.Bytes(), []byte(`"message":"success"`)) {
		t.Errorf("error response must not be followed by a success response: %s", recorder.Body.String())
	}
}

func TestRefreshReturnsNewTokenPairAndRevokesOldRefreshToken(t *testing.T) {
	expiresAt := jwt.NewNumericDate(time.Now().Add(time.Hour)).Time
	auth := &authenticationStub{}
	jwt := &jwtStub{
		claims: &models.JwtClaims{
			UserId:    42,
			Username:  "jane",
			Email:     "jane@example.com",
			TokenType: models.TokenTypeRefresh,
			RegisteredClaims: jwt.RegisteredClaims{
				ID:        "refresh-id",
				ExpiresAt: jwt.NewNumericDate(expiresAt),
			},
		},
	}

	recorder := refreshRequest(auth, jwt, "Bearer old-refresh-token")

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	var response utils.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected token object, got %#v", response.Data)
	}
	if response.Message != "success" || data["token"] != "signed-token" || data["refresh_token"] != "refresh-token" {
		t.Errorf("unexpected refresh response: %#v", response)
	}
	if jwt.validated != "old-refresh-token" || jwt.calls != 2 || jwt.userID != 42 || jwt.username != "jane" || jwt.email != "jane@example.com" {
		t.Errorf("unexpected refresh token processing: %#v", jwt)
	}
	if jwt.revokeCalls != 1 || jwt.revokedID != "refresh-id" || jwt.revokedAt.Unix() != expiresAt.Unix() {
		t.Errorf("expected old refresh token to be revoked, got calls=%d id=%q expiry=%v", jwt.revokeCalls, jwt.revokedID, jwt.revokedAt)
	}
}

func TestRefreshRejectsAccessToken(t *testing.T) {
	auth := &authenticationStub{}
	jwt := &jwtStub{
		claims: &models.JwtClaims{
			TokenType: models.TokenTypeAccess,
			RegisteredClaims: jwt.RegisteredClaims{
				ID:        "access-id",
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
		},
	}

	recorder := refreshRequest(auth, jwt, "Bearer access-token")

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d: %s", http.StatusUnauthorized, recorder.Code, recorder.Body.String())
	}
	if jwt.calls != 0 || jwt.revokeCalls != 0 {
		t.Errorf("access token must not be used or revoked for refresh: %#v", jwt)
	}
}

func TestRefreshRejectsInvalidOrRevokedToken(t *testing.T) {
	auth := &authenticationStub{}
	jwt := &jwtStub{validateErr: constant.ErrTokenRevoked}

	recorder := refreshRequest(auth, jwt, "Bearer revoked-token")

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d: %s", http.StatusUnauthorized, recorder.Code, recorder.Body.String())
	}
	if jwt.calls != 0 || jwt.revokeCalls != 0 {
		t.Errorf("invalid refresh token must not issue or revoke tokens: %#v", jwt)
	}
}

func TestRefreshRequiresBearerToken(t *testing.T) {
	auth := &authenticationStub{}
	jwt := &jwtStub{}

	recorder := refreshRequest(auth, jwt, "")

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d: %s", http.StatusUnauthorized, recorder.Code, recorder.Body.String())
	}
	if jwt.validateCall != 0 || jwt.calls != 0 || jwt.revokeCalls != 0 {
		t.Errorf("missing refresh token should stop before token operations: %#v", jwt)
	}
}

func TestRefreshRejectsExpiredOrInvalidToken(t *testing.T) {
	auth := &authenticationStub{}
	jwt := &jwtStub{validateErr: errors.New("token expired")}

	recorder := refreshRequest(auth, jwt, "Bearer expired-token")

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d: %s", http.StatusUnauthorized, recorder.Code, recorder.Body.String())
	}
	if jwt.calls != 0 || jwt.revokeCalls != 0 {
		t.Errorf("invalid token must not issue or revoke tokens: %#v", jwt)
	}
}

func TestRefreshReturnsErrorWhenTokenRotationFails(t *testing.T) {
	auth := &authenticationStub{}
	jwt := &jwtStub{
		claims: &models.JwtClaims{
			UserId:    42,
			TokenType: models.TokenTypeRefresh,
			RegisteredClaims: jwt.RegisteredClaims{
				ID:        "refresh-id",
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
		},
		revokeErr: errors.New("database unavailable"),
	}

	recorder := refreshRequest(auth, jwt, "Bearer old-refresh-token")

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d: %s", http.StatusInternalServerError, recorder.Code, recorder.Body.String())
	}
	if jwt.revokeCalls != 1 || bytes.Contains(recorder.Body.Bytes(), []byte(`"message":"success"`)) {
		t.Errorf("failed refresh rotation must not return success: %s", recorder.Body.String())
	}
}

func TestLogoutSuccess(t *testing.T) {
	auth := &authenticationStub{}
	expiresAt := jwt.NewNumericDate(time.Now().Add(time.Hour)).Time
	jwt := &jwtStub{
		claims: &models.JwtClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				ID:        "token-id",
				ExpiresAt: jwt.NewNumericDate(expiresAt),
			},
		},
	}

	recorder := logoutRequest(auth, jwt, "Bearer signed-token")

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var response utils.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected response data object, got %#v", response.Data)
	}
	if response.Code != http.StatusOK || response.Message != "success" || data["logged_out"] != true {
		t.Errorf("unexpected response: %#v", response)
	}
	if auth.calls != 0 || auth.loginCalls != 0 || jwt.calls != 0 {
		t.Errorf("logout should not register, authenticate, or generate tokens: auth=%#v jwt=%#v", auth, jwt)
	}
	if jwt.validateCall != 1 || jwt.validated != "signed-token" {
		t.Errorf("expected middleware to validate the bearer token, got %d calls for %q", jwt.validateCall, jwt.validated)
	}
	if !jwt.revoked || jwt.revokedID != "token-id" || !jwt.revokedAt.Equal(expiresAt) {
		t.Errorf("expected logout to revoke token ID and expiry, got revoked=%t id=%q expiry=%v", jwt.revoked, jwt.revokedID, jwt.revokedAt)
	}

	recorder = logoutRequest(auth, jwt, "Bearer signed-token")
	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("expected revoked token to be rejected with status %d, got %d: %s", http.StatusUnauthorized, recorder.Code, recorder.Body.String())
	}
}

func TestLogoutRequiresBearerToken(t *testing.T) {
	auth := &authenticationStub{}
	jwt := &jwtStub{}

	recorder := logoutRequest(auth, jwt, "")

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d: %s", http.StatusUnauthorized, recorder.Code, recorder.Body.String())
	}
	if jwt.validateCall != 0 || jwt.revoked {
		t.Errorf("token should not be validated or revoked without authorization header: %#v", jwt)
	}
}

func TestLogoutRejectsInvalidBearerToken(t *testing.T) {
	auth := &authenticationStub{}
	jwt := &jwtStub{validateErr: errors.New("invalid token")}

	recorder := logoutRequest(auth, jwt, "Bearer invalid-token")

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d: %s", http.StatusUnauthorized, recorder.Code, recorder.Body.String())
	}
	if jwt.validateCall != 1 || jwt.revoked {
		t.Errorf("invalid token must not be revoked: %#v", jwt)
	}
}

func TestLogoutReturnsErrorWhenRevocationFails(t *testing.T) {
	auth := &authenticationStub{}
	jwt := &jwtStub{
		claims: &models.JwtClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				ID:        "token-id",
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
		},
		revokeErr: errors.New("database unavailable"),
	}

	recorder := logoutRequest(auth, jwt, "Bearer signed-token")

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d: %s", http.StatusInternalServerError, recorder.Code, recorder.Body.String())
	}
	if jwt.revoked {
		t.Error("token should not be marked revoked when persistence fails")
	}
	if bytes.Contains(recorder.Body.Bytes(), []byte(`"message":"success"`)) {
		t.Errorf("error response must not be followed by a success response: %s", recorder.Body.String())
	}
}

func TestProfileReturnsAuthenticatedUser(t *testing.T) {
	auth := &authenticationStub{
		profile: dto.ProfileDto{
			Id:        42,
			Username:  "jane",
			Email:     "jane@example.com",
			FirstName: "Jane",
			LastName:  "Doe",
			Phone:     "08123456789",
		},
	}
	jwt := &jwtStub{
		claims: &models.JwtClaims{
			UserId: 42,
		},
	}

	recorder := profileRequest(auth, jwt, "Bearer valid-token")

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	var response utils.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected profile object in response, got %#v", response.Data)
	}
	if response.Message != "success" || data["id"] != float64(42) || data["username"] != "jane" || data["email"] != "jane@example.com" || data["first_name"] != "Jane" || data["last_name"] != "Doe" || data["phone"] != "08123456789" {
		t.Errorf("unexpected profile response: %#v", response)
	}
	if _, hasPassword := data["password"]; hasPassword {
		t.Errorf("profile response must not expose a password: %#v", data)
	}
	if auth.profileCalls != 1 || auth.profileUserID != 42 {
		t.Errorf("expected profile lookup for authenticated user 42, got calls=%d userID=%d", auth.profileCalls, auth.profileUserID)
	}
}

func TestProfileRequiresValidBearerToken(t *testing.T) {
	auth := &authenticationStub{}
	jwt := &jwtStub{validateErr: errors.New("invalid token")}

	recorder := profileRequest(auth, jwt, "")

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d: %s", http.StatusUnauthorized, recorder.Code, recorder.Body.String())
	}
	if auth.profileCalls != 0 || jwt.validateCall != 0 {
		t.Errorf("profile lookup must not run when authorization is missing: profile calls=%d validation calls=%d", auth.profileCalls, jwt.validateCall)
	}
}

func TestProfileReturnsNotFoundWhenUserDoesNotExist(t *testing.T) {
	auth := &authenticationStub{profileErr: gorm.ErrRecordNotFound}
	jwt := &jwtStub{claims: &models.JwtClaims{UserId: 404}}

	recorder := profileRequest(auth, jwt, "Bearer valid-token")

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, recorder.Code, recorder.Body.String())
	}
	if auth.profileCalls != 1 || auth.profileUserID != 404 {
		t.Errorf("expected lookup for user 404, got calls=%d userID=%d", auth.profileCalls, auth.profileUserID)
	}
}

func TestProfileReturnsInternalServerErrorWhenLookupFails(t *testing.T) {
	auth := &authenticationStub{profileErr: errors.New("database unavailable")}
	jwt := &jwtStub{claims: &models.JwtClaims{UserId: 42}}

	recorder := profileRequest(auth, jwt, "Bearer valid-token")

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d: %s", http.StatusInternalServerError, recorder.Code, recorder.Body.String())
	}
}

func TestCheckSessionReturnsAuthenticatedSession(t *testing.T) {
	expiresAt := jwt.NewNumericDate(time.Now().Add(time.Hour))
	jwtService := &jwtStub{
		claims: &models.JwtClaims{
			UserId:    42,
			Username:  "jane",
			Email:     "jane@example.com",
			TokenType: models.TokenTypeAccess,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: expiresAt,
			},
		},
	}

	recorder := checkSessionRequest(jwtService, "Bearer valid-access-token")

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	var response utils.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected session object, got %#v", response.Data)
	}
	user, ok := data["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected user object, got %#v", data["user"])
	}
	if response.Message != "success" || data["authenticated"] != true {
		t.Errorf("unexpected session response: %#v", response)
	}
	if user["id"] != float64(42) || user["username"] != "jane" || user["email"] != "jane@example.com" {
		t.Errorf("unexpected user data: %#v", user)
	}
	if jwtService.validateCall != 1 || jwtService.validated != "valid-access-token" {
		t.Errorf("expected access token validation once, got calls=%d token=%q", jwtService.validateCall, jwtService.validated)
	}
}

func TestCheckSessionRequiresValidAccessToken(t *testing.T) {
	tests := []struct {
		name          string
		authorization string
		jwt           *jwtStub
	}{
		{
			name:          "missing token",
			authorization: "",
			jwt:           &jwtStub{},
		},
		{
			name:          "invalid token",
			authorization: "Bearer invalid-token",
			jwt:           &jwtStub{validateErr: errors.New("invalid token")},
		},
		{
			name:          "refresh token",
			authorization: "Bearer refresh-token",
			jwt: &jwtStub{claims: &models.JwtClaims{
				TokenType: models.TokenTypeRefresh,
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := checkSessionRequest(tt.jwt, tt.authorization)
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("expected status %d, got %d: %s", http.StatusUnauthorized, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestJWTServiceRejectsRevokedToken(t *testing.T) {
	store := &tokenRevocationStoreStub{revoked: make(map[string]bool)}
	jwtService := services.NewJwtService(store)

	token, err := jwtService.GenerateToken(42, "jane", "jane@example.com", false)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	claims, err := jwtService.ValidateToken(token)
	if err != nil {
		t.Fatalf("validate token before logout: %v", err)
	}
	if claims.ID == "" || claims.ExpiresAt == nil {
		t.Fatalf("generated token is missing revocation claims: %#v", claims)
	}

	if err := jwtService.RevokeToken(claims.ID, claims.ExpiresAt.Time); err != nil {
		t.Fatalf("revoke token: %v", err)
	}
	if _, err := jwtService.ValidateToken(token); !errors.Is(err, constant.ErrTokenRevoked) {
		t.Errorf("expected revoked-token error, got %v", err)
	}
}

func TestJWTServiceFailsValidationWhenRevocationStoreIsUnavailable(t *testing.T) {
	store := &tokenRevocationStoreStub{revoked: make(map[string]bool)}
	jwtService := services.NewJwtService(store)

	token, err := jwtService.GenerateToken(42, "jane", "jane@example.com", false)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	store.err = errors.New("database unavailable")
	if _, err := jwtService.ValidateToken(token); !errors.Is(err, constant.ErrTokenRevocationDB) {
		t.Errorf("expected revocation-store error, got %v", err)
	}
}

func TestJWTServiceCanRevokeLegacyTokenWithoutID(t *testing.T) {
	store := &tokenRevocationStoreStub{revoked: make(map[string]bool)}
	jwtService := services.NewJwtService(store)
	expiresAt := time.Now().Add(time.Hour)
	legacyToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &models.JwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}).SignedString([]byte(config.LoadConfig().SECURITY_CONFIG.JwtSecret))
	if err != nil {
		t.Fatalf("sign legacy token: %v", err)
	}

	claims, err := jwtService.ValidateToken(legacyToken)
	if err != nil {
		t.Fatalf("validate legacy token: %v", err)
	}
	if claims.ID != "" {
		t.Fatalf("expected legacy token without ID, got %q", claims.ID)
	}

	tokenID := helpers.TokenRevocationID(legacyToken)
	if err := jwtService.RevokeToken(tokenID, claims.ExpiresAt.Time); err != nil {
		t.Fatalf("revoke legacy token: %v", err)
	}
	if _, err := jwtService.ValidateToken(legacyToken); !errors.Is(err, constant.ErrTokenRevoked) {
		t.Errorf("expected revoked-token error, got %v", err)
	}
}
