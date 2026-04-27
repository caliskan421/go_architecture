package middleware

import (
	"sync"

	"libra_management/internal/httpx"
	"libra_management/internal/model"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

// Authorizer, role -> izin seti çözümünü kapsülleyen küçük bir servistir.
// Her istek başına DB'ye gitmemek için role bazlı izinleri RAM'de cache'ler.
//
// Cache stratejisi:
//   - İlk RequirePermission çağrısında roleID için DB'den izinler okunur.
//   - Sonraki çağrılar cache'ten okur.
//   - Rol-izin değişirse Invalidate(roleID) ya da Clear() çağrılmalı.
//
// İleride seed dışında runtime'da rol değişikliği eklenirse buradaki cache
// invalidation noktası önemli olacak (Faz 5'te yalnız boot-seed kullandığımız için
// geliştirme süresince Clear ihtiyacı yok).
type Authorizer struct {
	db    *gorm.DB
	mu    sync.RWMutex
	cache map[uint]map[string]struct{}
}

// NewAuthorizer, boot'ta bir kez çağrılır.
func NewAuthorizer(db *gorm.DB) *Authorizer {
	return &Authorizer{
		db:    db,
		cache: make(map[uint]map[string]struct{}),
	}
}

// loadFromDB, verilen role için Permissions tablosundan izin isimlerini çeker.
// Cache miss yolunda tek round-trip JOIN sorgusu üretir.
func (a *Authorizer) loadFromDB(roleID uint) (map[string]struct{}, error) {
	var role model.Role
	if err := a.db.Preload("Permissions").First(&role, roleID).Error; err != nil {
		return nil, err
	}
	set := make(map[string]struct{}, len(role.Permissions))
	for _, p := range role.Permissions {
		set[p.Permission] = struct{}{}
	}
	return set, nil
}

// permsFor, cache-aware lookup. RWMutex ile concurrent okuma destekli.
// Yazma yolunda double-check pattern: lock upgrade ettikten sonra başka
// goroutine doldurmuş olabilir, kontrol et.
func (a *Authorizer) permsFor(roleID uint) (map[string]struct{}, error) {
	a.mu.RLock()
	if set, ok := a.cache[roleID]; ok {
		a.mu.RUnlock()
		return set, nil
	}
	a.mu.RUnlock()

	a.mu.Lock()
	defer a.mu.Unlock()
	if set, ok := a.cache[roleID]; ok {
		return set, nil
	}
	set, err := a.loadFromDB(roleID)
	if err != nil {
		return nil, err
	}
	a.cache[roleID] = set
	return set, nil
}

// Invalidate, belirli bir rolün cache satırını temizler.
// Şu an çağıran yok (seed-only); ileride admin paneliyle rol-izin değişikliği
// eklenirse buradan çağrılır.
func (a *Authorizer) Invalidate(roleID uint) {
	a.mu.Lock()
	delete(a.cache, roleID)
	a.mu.Unlock()
}

// Clear, tüm cache'i temizler. Test ortamlarında DB resetlendiğinde gerekebilir.
func (a *Authorizer) Clear() {
	a.mu.Lock()
	a.cache = make(map[uint]map[string]struct{})
	a.mu.Unlock()
}

// RequirePermission, c.Locals'tan roleID'yi okur ve verilen izne sahip olup
// olmadığını cache üzerinden doğrular. Auth middleware'inden SONRA bağlanmalı:
//
//	app.Get("/api/authors", Auth(...), authorizer.RequirePermission(auth.PermAuthorRead), handler.List)
//
// Eski RequireRole'ün yerine geçer; rol-bazlı erişim, izin-bazlı erişimle
// yer değiştirdi (daha esnek + admin/user dışında roller eklenince genişler).
func (a *Authorizer) RequirePermission(perm string) fiber.Handler {
	return func(c fiber.Ctx) error {
		roleID, ok := c.Locals("roleID").(uint)
		if !ok {
			// Auth çalışmamış demektir; programlama hatası olarak 401 dönüyoruz.
			return httpx.ErrUnauthorized
		}
		set, err := a.permsFor(roleID)
		if err != nil {
			return httpx.ErrInternal.WithErr(err)
		}
		if _, ok := set[perm]; !ok {
			return httpx.ErrForbidden
		}
		return c.Next()
	}
}
