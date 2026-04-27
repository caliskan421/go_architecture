package handler

import (
	"errors"
	"fmt"
	"time"

	"libra_management/internal/config"
	"libra_management/internal/dto"
	"libra_management/internal/httpx"
	"libra_management/internal/model"
	"libra_management/pkg/token"
	"libra_management/pkg/validate"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// bcryptCost — 12-14 arası önerilir; 14 yavaş ama çok daha dayanıklı.
const bcryptCost = 14

// AuthHandler, register/login/logout endpoint'lerinin sahibi.
// Tüm bağımlılıklar struct alanı olarak DI ile gelir — global state YOK.
// Test'te `&AuthHandler{db: testDB, tokens: testTM, ...}` ile kolayca mock'lanır.
type AuthHandler struct {
	db            *gorm.DB
	cfg           *config.Config
	tokens        *token.Manager
	val           *validator.Validate
	defaultRoleID uint   // cfg.DefaultRoleTitle'a karşılık DB'den çekilip cache'lendi (boot'ta bir kez)
	dummyHash     []byte // login timing-attack guard için sahte bcrypt hash
}

// NewAuthHandler, boot'ta bir kez çağrılır.
// Default role DB'de yoksa hata döner (Seed çalışmamış olabilir) — main.go bunu
// log.Fatalf ile yakalar; uygulama yanlış konfigle çalışmaya devam etmez.
func NewAuthHandler(db *gorm.DB, cfg *config.Config, tokens *token.Manager, val *validator.Validate) (*AuthHandler, error) {
	// 1) Default role'ün ID'sini boot'ta bir kez çek — her register'da DB'ye gitmemek için.
	var role model.Role
	if err := db.Where("title = ?", cfg.DefaultRoleTitle).First(&role).Error; err != nil {
		return nil, fmt.Errorf("default role %q DB'de bulunamadı (Seed çalıştı mı?): %w", cfg.DefaultRoleTitle, err)
	}

	// 2) Login'de "user yok" yolunda da bcrypt çalıştırmak için sahte hash.
	// Aksi halde "quick 401" (user yok) ile "slow 401" (şifre yanlış) latency farkından
	// saldırgan email enumeration yapabilir (timing attack).
	dummy, err := bcrypt.GenerateFromPassword([]byte("dummy"), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("dummy hash üretilemedi: %w", err)
	}

	return &AuthHandler{
		db:            db,
		cfg:           cfg,
		tokens:        tokens,
		val:           val,
		defaultRoleID: role.ID,
		dummyHash:     dummy,
	}, nil
}

// Register, yeni kullanıcı oluşturur.
//
//	POST /api/register  body: dto.RegisterRequest  -> 201 {"data": dto.UserResponse}
func (h *AuthHandler) Register(c fiber.Ctx) error {
	// Bind: HTTP body'yi struct'a parse eder. JSON tag'lerine göre alan eşlemesi yapar.
	var req dto.RegisterRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.ErrBadRequest.WithErr(err)
	}

	// Validator artık tüm kuralları (email format, min length, eqfield, required) tek
	// hamlede uygular. Eski "if password != confirm" / manual check'ler kalktı.
	if err := h.val.Struct(&req); err != nil {
		return httpx.ErrValidation.WithDetails(validate.Format(err))
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost)
	if err != nil {
		return httpx.ErrInternal.WithErr(err)
	}

	user := model.User{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Email:     req.Email,
		Password:  string(hashed),
		// FIX: hardcoded "RoleID: 2" yerine boot'ta title üzerinden çekilen ID.
		// Migration sırası değişse bile "user" rolü = doğru ID — privilege-escalation engellendi.
		RoleID: h.defaultRoleID,
	}
	err = h.db.Create(&user).Error
	switch {
	// Email uniqueIndex çakışması → 409. TranslateError:true sayesinde gorm.ErrDuplicatedKey'a çevrilir.
	// Mesaj generic: "kayıt mevcut" — saldırgan email enumeration yapamasın diye field detail vermiyoruz.
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return httpx.ErrConflict
	case err != nil:
		// Ham DB hatası DOĞRUDAN sızdırılmaz; generic 500. Caller log'tan görür.
		return httpx.ErrInternal.WithErr(err)
	}

	// Eski kod: db.Preload("Role").First(&user, user.ID) — gereksiz ikinci sorgu.
	// Çözüm: ID ve Title'ı zaten elimizde, struct'a elle yerleştir; Preload pahalı.
	user.Role = model.Role{ID: h.defaultRoleID, Title: h.cfg.DefaultRoleTitle}

	return httpx.Success(c, fiber.StatusCreated, dto.ToUserResponse(user))
}

// Login, kimliği doğrular ve JWT cookie set eder.
//
//	POST /api/login  body: dto.LoginRequest  -> 200 {"data": dto.UserResponse} + jwt cookie
func (h *AuthHandler) Login(c fiber.Ctx) error {
	var req dto.LoginRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.ErrBadRequest.WithErr(err)
	}
	if err := h.val.Struct(&req); err != nil {
		return httpx.ErrValidation.WithDetails(validate.Format(err))
	}

	var user model.User
	err := h.db.Preload("Role").Where("email = ?", req.Email).First(&user).Error

	// GÜVENLİK (user enumeration): "user yok" ile "şifre yanlış" arasında HİÇBİR ayrım
	// yapmıyoruz — ne mesaj farkı ne timing farkı. Saldırgan email'in sistemde olup
	// olmadığını response'tan anlayamaz.
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		// User yok ama bcrypt'i yine de çağırıyoruz: response süresi normal user ile aynı kalsın.
		_ = bcrypt.CompareHashAndPassword(h.dummyHash, []byte(req.Password))
		return httpx.ErrInvalidCredentials
	case err != nil:
		return httpx.ErrInternal.WithErr(err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return httpx.ErrInvalidCredentials
	}

	jwtToken, err := h.tokens.Generate(user.ID, user.RoleID)
	if err != nil {
		return httpx.ErrInternal.WithErr(err)
	}

	c.Cookie(&fiber.Cookie{
		Name:     "jwt",
		Value:    jwtToken,
		HTTPOnly: true,               // JS'den okunamaz — XSS guard
		Secure:   h.cfg.CookieSecure, // sadece HTTPS — production'da true
		SameSite: "Lax",              // CSRF guard (cross-site POST'larda gönderilmez)
		Expires:  time.Now().Add(h.cfg.JWTTTL),
	})

	return httpx.Success(c, fiber.StatusOK, dto.ToUserResponse(user))
}

// Logout, JWT cookie'yi geçmiş tarihle yazarak browser'da siler.
//
//	POST /api/logout  ->  200 {"data": {"message": "..."}}
func (h *AuthHandler) Logout(c fiber.Ctx) error {
	c.Cookie(&fiber.Cookie{
		Name:     "jwt",
		Value:    "",
		HTTPOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: "Lax",
		Expires:  time.Now().Add(-time.Hour),
	})
	return httpx.Success(c, fiber.StatusOK, fiber.Map{"message": "logout success"})
}
