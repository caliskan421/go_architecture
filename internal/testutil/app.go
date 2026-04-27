package testutil

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"libra_management/internal/config"
	"libra_management/internal/handler"
	"libra_management/internal/httpx"
	"libra_management/internal/middleware"
	"libra_management/internal/model"
	"libra_management/internal/router"
	"libra_management/pkg/token"
	"libra_management/pkg/validate"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// silenceSlog, slog'un default handler'ını test çıktısı için sessizleştirir.
// sync.Once ile yalnız bir kez çalışır; paralel testlerde race olmasın.
var silenceSlog sync.Once

func quietSlog() {
	silenceSlog.Do(func() {
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	})
}

// TestApp, bir test için tam koşulmuş app + ilgili bağımlılıkları taşır.
// Test'lerde request-response akışı bunun üzerinden yürür.
type TestApp struct {
	App *fiber.App
	DB  *gorm.DB
	Cfg *config.Config
}

// NewTestApp, üretimle birebir aynı kompozisyonda Fiber app kurar:
//
//	config (test değerleri) → in-memory DB → handler'lar → authorizer → router
//
// Üretim main.go'sundaki tek fark: cookie.Secure=false (HTTP test client kullanıyoruz).
func NewTestApp(t *testing.T) *TestApp {
	t.Helper()
	quietSlog()

	db := NewTestDB(t)

	cfg := &config.Config{
		DBDSn:            "sqlite::memory:",
		JWTSecret:        "test-secret-not-for-prod-0123456789abcdef0123456789abcdef",
		JWTTTL:           time.Hour,
		Port:             ":0",
		AllowedOrigins:   []string{"*"},
		DefaultRoleTitle: "user",
		CookieSecure:     false,
	}

	tokenMgr := token.New(cfg.JWTSecret, cfg.JWTTTL)
	val := validate.New()

	authH, err := handler.NewAuthHandler(db, cfg, tokenMgr, val)
	require.NoError(t, err)
	authorH := handler.NewAuthorHandler(db, val)
	bookH := handler.NewBookHandler(db, val)
	libraryH := handler.NewLibraryHandler(db, val)
	az := middleware.NewAuthorizer(db)

	app := fiber.New(fiber.Config{ErrorHandler: httpx.Handler})
	app.Use(middleware.RequestID())
	router.Setup(app, router.Deps{
		Auth:       authH,
		Author:     authorH,
		Book:       bookH,
		Library:    libraryH,
		DB:         db,
		TokenMgr:   tokenMgr,
		Authorizer: az,
	})

	return &TestApp{App: app, DB: db, Cfg: cfg}
}

// --- Login helper'ları ---

// SeedAdminUser, DB'ye doğrudan bir admin kullanıcı ekler ve email/şifresini döner.
// Register endpoint'i sadece "user" role'ünü atadığı için admin olmak isteyen
// test'lerin DB'ye doğrudan satır eklemesi gerekiyor.
func (ta *TestApp) SeedAdminUser(t *testing.T) (email, password string) {
	t.Helper()
	password = "admin12345"
	email = "admin-test@example.com"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)

	var role model.Role
	require.NoError(t, ta.DB.Where("title = ?", "admin").First(&role).Error)

	u := model.User{
		FirstName: "Test", LastName: "Admin",
		Email: email, Password: string(hash), RoleID: role.ID,
	}
	require.NoError(t, ta.DB.Create(&u).Error)
	return email, password
}

// SeedUserUser, register'ın yarattığı default "user" role'üyle bir kullanıcı ekler.
func (ta *TestApp) SeedUserUser(t *testing.T) (email, password string) {
	t.Helper()
	password = "user123456"
	email = "user-test@example.com"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)

	var role model.Role
	require.NoError(t, ta.DB.Where("title = ?", "user").First(&role).Error)

	u := model.User{
		FirstName: "Test", LastName: "User",
		Email: email, Password: string(hash), RoleID: role.ID,
	}
	require.NoError(t, ta.DB.Create(&u).Error)
	return email, password
}

// Login, /api/login endpoint'ini çağırır ve JWT cookie'sini döner.
// Bu cookie sonraki request'lerin Cookie header'ına eklenir → protected
// endpoint'lere erişim sağlanır.
func (ta *TestApp) Login(t *testing.T, email, password string) string {
	t.Helper()
	body := map[string]string{"email": email, "password": password}
	resp := ta.Post(t, "/api/login", body, "")
	require.Equal(t, fiber.StatusOK, resp.StatusCode, "login should succeed")
	for _, c := range resp.Cookies() {
		if c.Name == "jwt" {
			return c.Value
		}
	}
	t.Fatal("jwt cookie not set")
	return ""
}

// LoginAdmin, kısa yol: admin seed et + login.
func (ta *TestApp) LoginAdmin(t *testing.T) string {
	email, pass := ta.SeedAdminUser(t)
	return ta.Login(t, email, pass)
}

// LoginUser, kısa yol: user seed et + login.
func (ta *TestApp) LoginUser(t *testing.T) string {
	email, pass := ta.SeedUserUser(t)
	return ta.Login(t, email, pass)
}

// --- HTTP helper'ları ---

// Request, body'yi JSON encode eder, cookie eklerse Cookie header'ı koyar,
// app.Test ile request'i çalıştırır ve http.Response döner.
//
// timeout'u -1 yapıyoruz: in-memory test'te middleware/handler async olmadığı
// için zaman aşımı semantiği test'i bozar. Fiber'in test client'ı bu değeri
// "timeout devre dışı" olarak yorumlar.
func (ta *TestApp) Request(t *testing.T, method, path string, body interface{}, cookie string) *http.Response {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		require.NoError(t, err)
		rdr = bytes.NewReader(buf)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "jwt", Value: cookie})
	}
	resp, err := ta.App.Test(req, fiber.TestConfig{Timeout: -1})
	require.NoError(t, err)
	return resp
}

// Get, kısa yol — GET request.
func (ta *TestApp) Get(t *testing.T, path, cookie string) *http.Response {
	return ta.Request(t, http.MethodGet, path, nil, cookie)
}

// Post, kısa yol — POST + JSON body.
func (ta *TestApp) Post(t *testing.T, path string, body interface{}, cookie string) *http.Response {
	return ta.Request(t, http.MethodPost, path, body, cookie)
}

// Put, kısa yol — PUT + JSON body.
func (ta *TestApp) Put(t *testing.T, path string, body interface{}, cookie string) *http.Response {
	return ta.Request(t, http.MethodPut, path, body, cookie)
}

// Delete, kısa yol — DELETE; body opsiyonel (alt-route'lar JSON body kullanıyor).
func (ta *TestApp) Delete(t *testing.T, path string, body interface{}, cookie string) *http.Response {
	return ta.Request(t, http.MethodDelete, path, body, cookie)
}

// DecodeData, response body'sinin {"data": ...} zarfını açıp out'a çözümler.
func DecodeData(t *testing.T, resp *http.Response, out interface{}) {
	t.Helper()
	defer resp.Body.Close()
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&env))
	require.NotEmpty(t, env.Data, "response 'data' alanı boş")
	require.NoError(t, json.Unmarshal(env.Data, out))
}

// DecodeError, response body'sinin {"error": ...} zarfını açıp Code+Message döner.
func DecodeError(t *testing.T, resp *http.Response) (code, message string) {
	t.Helper()
	defer resp.Body.Close()
	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&env))
	return env.Error.Code, env.Error.Message
}
