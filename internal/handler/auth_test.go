package handler_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/wzfukui/agent-native-im/internal/auth"
	"github.com/wzfukui/agent-native-im/internal/model"
)

func TestPing(t *testing.T) {
	resp := doJSON(t, "GET", "/api/v1/ping", nil, nil)
	assertStatus(t, resp, http.StatusOK)
}

func TestLogin(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)
	if token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestLoginBadCredentials(t *testing.T) {
	truncateAll(t)
	seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/auth/login", nil, map[string]string{
		"username": "testadmin",
		"password": "wrongpass",
	})
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestMeWithJWT(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "GET", "/api/v1/me", ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)

	data := parseOK(t, resp)
	entity, ok := data["name"]
	if !ok || entity != "testadmin" {
		t.Fatalf("expected name=testadmin, got %v", entity)
	}
}

func TestMeWithoutAuth(t *testing.T) {
	resp := doJSON(t, "GET", "/api/v1/me", nil, nil)
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestChangePassword(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Change password
	resp := doJSON(t, "PUT", "/api/v1/me/password", ptr(token), map[string]string{
		"old_password": "testpass",
		"new_password": "Newpass123",
	})
	assertStatus(t, resp, http.StatusOK)

	// Old password should no longer work
	resp = doJSON(t, "POST", "/api/v1/auth/login", nil, map[string]string{
		"username": "testadmin",
		"password": "testpass",
	})
	assertStatus(t, resp, http.StatusUnauthorized)

	// New password should work
	resp = doJSON(t, "POST", "/api/v1/auth/login", nil, map[string]string{
		"username": "testadmin",
		"password": "Newpass123",
	})
	assertStatus(t, resp, http.StatusOK)
}

func TestChangePasswordWrongOld(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "PUT", "/api/v1/me/password", ptr(token), map[string]string{
		"old_password": "wrong",
		"new_password": "Newpass123",
	})
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestCreateUser(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create a new user
	resp := doJSON(t, "POST", "/api/v1/admin/users", ptr(token), map[string]string{
		"username":     "newuser",
		"password":     "Userpass123",
		"display_name": "New User",
	})
	assertStatus(t, resp, http.StatusCreated)

	data := parseOK(t, resp)
	if data["name"] != "newuser" {
		t.Fatalf("expected name=newuser, got %v", data["name"])
	}

	// New user should be able to login
	resp = doJSON(t, "POST", "/api/v1/auth/login", nil, map[string]string{
		"username": "newuser",
		"password": "Userpass123",
	})
	assertStatus(t, resp, http.StatusOK)
}

func TestCreateUserShortPassword(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/admin/users", ptr(token), map[string]string{
		"username": "shortpw",
		"password": "12345",
	})
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestRefreshToken(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Refresh token
	resp := doJSON(t, "POST", "/api/v1/auth/refresh", ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	data := parseOK(t, resp)
	newToken, ok := data["token"].(string)
	if !ok || newToken == "" {
		t.Fatal("expected new token from refresh")
	}

	// New token should work
	resp = doJSON(t, "GET", "/api/v1/me", ptr(newToken), nil)
	assertStatus(t, resp, http.StatusOK)
}

func TestRefreshTokenWithRecentlyExpiredJWT(t *testing.T) {
	truncateAll(t)
	validToken := seedAdmin(t)
	claims, err := auth.ParseToken("test-secret", validToken)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}

	expiredTokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"entity_id":   float64(claims.EntityID),
		"entity_type": string(model.EntityUser),
		"exp":         time.Now().Add(-2 * time.Hour).Unix(),
		"iat":         time.Now().Add(-26 * time.Hour).Unix(),
	})
	expiredToken, err := expiredTokenObj.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("failed to sign expired token: %v", err)
	}

	resp := doJSON(t, "POST", "/api/v1/auth/refresh", &expiredToken, nil)
	assertStatus(t, resp, http.StatusOK)
	data := parseOK(t, resp)
	newToken, ok := data["token"].(string)
	if !ok || newToken == "" {
		t.Fatal("expected new token from refresh with expired JWT")
	}
}

func TestRefreshTokenRejectsTooOldExpiredJWT(t *testing.T) {
	truncateAll(t)
	validToken := seedAdmin(t)
	claims, err := auth.ParseToken("test-secret", validToken)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}

	oldExpiredTokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"entity_id":   float64(claims.EntityID),
		"entity_type": string(model.EntityUser),
		"exp":         time.Now().Add(-8 * 24 * time.Hour).Unix(),
		"iat":         time.Now().Add(-9 * 24 * time.Hour).Unix(),
	})
	oldExpiredToken, err := oldExpiredTokenObj.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("failed to sign old expired token: %v", err)
	}

	resp := doJSON(t, "POST", "/api/v1/auth/refresh", &oldExpiredToken, nil)
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestRefreshTokenRejectsBotEntity(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/entities", ptr(token), map[string]string{"name": "refresh-bot"})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	// New flow: creation returns api_key (aim_ prefix) directly
	apiKey, _ := data["api_key"].(string)
	if apiKey == "" {
		t.Fatal("expected api_key")
	}

	// Bot API keys should not be refreshable via JWT refresh endpoint
	resp = doJSON(t, "POST", "/api/v1/auth/refresh", &apiKey, nil)
	assertStatus(t, resp, http.StatusForbidden)
}

func TestUpdateProfile(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Update display name
	resp := doJSON(t, "PUT", "/api/v1/me", ptr(token), map[string]string{
		"display_name": "Chris Admin",
	})
	assertStatus(t, resp, http.StatusOK)
	data := parseOK(t, resp)
	if data["display_name"] != "Chris Admin" {
		t.Fatalf("expected display_name=Chris Admin, got %v", data["display_name"])
	}

	// Update avatar
	resp = doJSON(t, "PUT", "/api/v1/me", ptr(token), map[string]string{
		"avatar_url": "https://example.com/avatar.png",
	})
	assertStatus(t, resp, http.StatusOK)
	data = parseOK(t, resp)
	if data["avatar_url"] != "https://example.com/avatar.png" {
		t.Fatalf("expected avatar_url updated, got %v", data["avatar_url"])
	}

	// Verify via /me
	resp = doJSON(t, "GET", "/api/v1/me", ptr(token), nil)
	assertStatus(t, resp, http.StatusOK)
	data = parseOK(t, resp)
	if data["display_name"] != "Chris Admin" {
		t.Fatalf("expected display_name persisted, got %v", data["display_name"])
	}
}

func TestUpdateProfileEmpty(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Empty update should fail
	resp := doJSON(t, "PUT", "/api/v1/me", ptr(token), map[string]interface{}{})
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestCreateUserNonAdmin(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create a regular user
	resp := doJSON(t, "POST", "/api/v1/admin/users", ptr(token), map[string]string{
		"username": "regular",
		"password": "Regular123",
	})
	assertStatus(t, resp, http.StatusCreated)

	regularToken := login(t, "regular", "Regular123")

	// Regular user tries to create a user — should fail (admin only)
	resp = doJSON(t, "POST", "/api/v1/admin/users", ptr(regularToken), map[string]string{
		"username": "another",
		"password": "Another123",
	})
	assertStatus(t, resp, http.StatusForbidden)
}

func TestLoginDisabledUserForbidden(t *testing.T) {
	truncateAll(t)
	adminToken := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/admin/users", ptr(adminToken), map[string]string{
		"username": "disabled-user",
		"password": "Disabled123",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	userID := int64(data["id"].(float64))

	resp = doJSON(t, "PUT", fmt.Sprintf("/api/v1/admin/users/%d", userID), ptr(adminToken), map[string]string{
		"status": "disabled",
	})
	assertStatus(t, resp, http.StatusOK)

	resp = doJSON(t, "POST", "/api/v1/auth/login", nil, map[string]string{
		"username": "disabled-user",
		"password": "Disabled123",
	})
	assertStatus(t, resp, http.StatusForbidden)
}

func TestDisabledUserTokenRejectedByMeAndRefresh(t *testing.T) {
	truncateAll(t)
	adminToken := seedAdmin(t)

	resp := doJSON(t, "POST", "/api/v1/admin/users", ptr(adminToken), map[string]string{
		"username": "disabled-after-login",
		"password": "Disabled123",
	})
	assertStatus(t, resp, http.StatusCreated)
	data := parseOK(t, resp)
	userID := int64(data["id"].(float64))

	userToken := login(t, "disabled-after-login", "Disabled123")

	resp = doJSON(t, "PUT", fmt.Sprintf("/api/v1/admin/users/%d", userID), ptr(adminToken), map[string]string{
		"status": "disabled",
	})
	assertStatus(t, resp, http.StatusOK)

	resp = doJSON(t, "GET", "/api/v1/me", ptr(userToken), nil)
	assertStatus(t, resp, http.StatusForbidden)

	resp = doJSON(t, "POST", "/api/v1/auth/refresh", ptr(userToken), nil)
	assertStatus(t, resp, http.StatusForbidden)
}
