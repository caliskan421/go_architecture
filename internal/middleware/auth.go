package middleware

import (
	"errors"

	"libra_management/internal/httpx"
	"libra_management/internal/model"
	"libra_management/pkg/token"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

// Auth, JWT cookie'sini doğrulayan ve kullanıcının HÂLÂ DB'de var olduğunu teyit
// eden bir Fiber handler döner. "Factory pattern": dependency'leri closure ile
// yakalar, her istek için yeni state kurmaz.
//
// Akış:
//
//	cookie -> tokenMgr.Parse -> DB user lookup -> c.Locals (userID, roleID) -> c.Next
//
// Eski sürümden farklar:
//   - db ve tokenMgr DI ile geliyor (paket-level os.Getenv yok)
//   - Token geçerli olsa bile user silinmişse erişim reddedilir (eski sürümde açıktı)
//   - Hata response'ları httpx sentinel'leri ile tek tipte
func Auth(db *gorm.DB, tokenMgr *token.Manager) fiber.Handler {
	return func(c fiber.Ctx) error {
		jwtToken := c.Cookies("jwt")
		if jwtToken == "" {
			return httpx.ErrUnauthorized
		}

		claims, err := tokenMgr.Parse(jwtToken)
		if err != nil {
			// Underlying err'i log için sarıyoruz; client yine generic "unauthorized" görür
			return httpx.ErrUnauthorized.WithErr(err)
		}

		// GÜVENLİK: token süresi dolmamış ama user silinmiş olabilir.
		// Hafif bir SELECT id, role_id ile teyit ediyoruz — tam user'ı çekmiyoruz.
		var user model.User
		err = db.Select("id", "role_id").First(&user, claims.UserID).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return httpx.ErrUnauthorized
			}
			return httpx.ErrInternal.WithErr(err)
		}

		// c.Locals: Fiber'in REQUEST-SCOPE context store'u (sadece bu istek boyunca yaşar).
		// Sonraki handler/middleware buradan değerleri okur.
		// Token'daki RoleID yerine DB'den taze çekilen RoleID'yi yazıyoruz —
		// kullanıcının rolü değişmiş olabilir, token eski değeri tutuyor olabilir.
		c.Locals("userID", user.ID)
		c.Locals("roleID", user.RoleID)
		return c.Next()
	}
}

// RequireRole, c.Locals'tan roleID'yi okur ve verilen izinli liste içinde olup
// olmadığını kontrol eder. Variadic parametre: RequireRole(adminID) veya
// RequireRole(adminID, editorID) çağrılabilir.
//
// Kullanım sırası ÖNEMLİ — Auth bu middleware'den ÖNCE çalışmalı:
//
//	app.Get("/admin", Auth(db, tm), RequireRole(adminID), handler.AdminPanel)
//
// Aksi halde locals boş gelir.
func RequireRole(roleIDs ...uint) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Type assertion: c.Locals interface{} döner; uint'e cast etmek gerek.
		// ok=false olması Auth middleware'inin koşmadığını gösterir — programlama hatası,
		// güvenlik açığı olmaması için 401 (yetkisiz) ile reddediyoruz.
		roleID, ok := c.Locals("roleID").(uint)
		if !ok {
			return httpx.ErrUnauthorized
		}
		for _, allowed := range roleIDs {
			if roleID == allowed {
				return c.Next()
			}
		}
		return httpx.ErrForbidden
	}
}
