package handler_test

import (
	"net/http"
	"testing"
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
		"new_password": "newpass123",
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
		"password": "newpass123",
	})
	assertStatus(t, resp, http.StatusOK)
}

func TestChangePasswordWrongOld(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	resp := doJSON(t, "PUT", "/api/v1/me/password", ptr(token), map[string]string{
		"old_password": "wrong",
		"new_password": "newpass123",
	})
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestCreateUser(t *testing.T) {
	truncateAll(t)
	token := seedAdmin(t)

	// Create a new user
	resp := doJSON(t, "POST", "/api/v1/admin/users", ptr(token), map[string]string{
		"username":     "newuser",
		"password":     "userpass123",
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
		"password": "userpass123",
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
		"password": "regular123",
	})
	assertStatus(t, resp, http.StatusCreated)

	regularToken := login(t, "regular", "regular123")

	// Regular user tries to create a user — should fail (admin only)
	resp = doJSON(t, "POST", "/api/v1/admin/users", ptr(regularToken), map[string]string{
		"username": "another",
		"password": "another123",
	})
	assertStatus(t, resp, http.StatusForbidden)
}
