package handler_test

import (
	"net/http"
	"testing"

	"libra_management/internal/dto"
	"libra_management/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Bu dosya AuthHandler'ın 3 endpoint'ini (Register, Login, Logout) entegrasyon
// seviyesinde test eder. Black-box: package handler_test ile dışarıdan import,
// sadece HTTP sözleşmesini doğrular — iç state'e bakmıyoruz (DB user.RoleID
// gibi olağanüstü durumlar dışında).

func TestRegister_Success(t *testing.T) {
	ta := testutil.NewTestApp(t)

	body := map[string]string{
		"first_name":       "Yaşar",
		"last_name":        "Kemal",
		"email":            "yk@example.com",
		"password":         "supersecret",
		"password_confirm": "supersecret",
	}
	resp := ta.Post(t, "/api/register", body, "")
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var u dto.UserResponse
	testutil.DecodeData(t, resp, &u)
	assert.Equal(t, "Yaşar", u.FirstName)
	assert.Equal(t, "yk@example.com", u.Email)
	assert.Equal(t, "user", u.RoleTitle, "default role 'user' atanmalı")
}

func TestRegister_PasswordMismatch_Returns400(t *testing.T) {
	ta := testutil.NewTestApp(t)

	body := map[string]string{
		"first_name":       "X",
		"last_name":        "Y",
		"email":            "xy@example.com",
		"password":         "supersecret",
		"password_confirm": "different1",
	}
	resp := ta.Post(t, "/api/register", body, "")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	code, _ := testutil.DecodeError(t, resp)
	assert.Equal(t, "validation_error", code)
}

func TestRegister_ShortFirstName_Returns400(t *testing.T) {
	ta := testutil.NewTestApp(t)

	body := map[string]string{
		"first_name":       "Y", // min=2 → fail
		"last_name":        "Kemal",
		"email":            "yk@example.com",
		"password":         "supersecret",
		"password_confirm": "supersecret",
	}
	resp := ta.Post(t, "/api/register", body, "")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRegister_DuplicateEmail_Returns409(t *testing.T) {
	ta := testutil.NewTestApp(t)

	body := map[string]string{
		"first_name":       "Ali",
		"last_name":        "Yılmaz",
		"email":            "dupe@example.com",
		"password":         "supersecret",
		"password_confirm": "supersecret",
	}
	resp := ta.Post(t, "/api/register", body, "")
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// Aynı email ile ikinci register → 409 conflict (uniqueIndex)
	resp = ta.Post(t, "/api/register", body, "")
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	code, _ := testutil.DecodeError(t, resp)
	assert.Equal(t, "conflict", code)
}

func TestLogin_Success(t *testing.T) {
	ta := testutil.NewTestApp(t)
	email, pass := ta.SeedUserUser(t)

	resp := ta.Post(t, "/api/login", map[string]string{
		"email": email, "password": pass,
	}, "")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// JWT cookie set olmalı
	hasJWT := false
	for _, c := range resp.Cookies() {
		if c.Name == "jwt" && c.Value != "" {
			hasJWT = true
			assert.True(t, c.HttpOnly, "JWT cookie HttpOnly olmalı")
		}
	}
	assert.True(t, hasJWT, "jwt cookie set edilmeli")
}

func TestLogin_WrongPassword_Returns401(t *testing.T) {
	ta := testutil.NewTestApp(t)
	email, _ := ta.SeedUserUser(t)

	resp := ta.Post(t, "/api/login", map[string]string{
		"email": email, "password": "wrongpassword",
	}, "")
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	code, _ := testutil.DecodeError(t, resp)
	assert.Equal(t, "invalid_credentials", code, "user enumeration koruması: tek tip hata")
}

func TestLogin_NonexistentUser_ReturnsSameAsWrongPassword(t *testing.T) {
	ta := testutil.NewTestApp(t)

	resp := ta.Post(t, "/api/login", map[string]string{
		"email": "nope@example.com", "password": "anything12345",
	}, "")
	// User enumeration koruması: "user yok" da "şifre yanlış" da AYNI cevap.
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	code, _ := testutil.DecodeError(t, resp)
	assert.Equal(t, "invalid_credentials", code)
}

func TestProtectedEndpoint_WithoutCookie_Returns401(t *testing.T) {
	ta := testutil.NewTestApp(t)
	resp := ta.Get(t, "/api/authors", "")
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestLogout_ClearsJWTCookie(t *testing.T) {
	ta := testutil.NewTestApp(t)
	resp := ta.Post(t, "/api/logout", nil, "")
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	for _, c := range resp.Cookies() {
		if c.Name == "jwt" {
			assert.Empty(t, c.Value, "logout cookie boşaltmalı")
		}
	}
}
