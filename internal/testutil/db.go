// Package testutil, sadece test'lerden çağrılan yardımcılardır.
// Üretimde import edilmez (build tag yok ama _test.go dışında kullanılması beklenmez).
//
// İçeriğin amacı:
//   - In-memory SQLite ile her test'e izole bir DB
//   - Migrate + Seed çalıştırarak gerçek schema'yla aynı state
//   - Tam Fiber app kurulumu: handler/middleware/router/error-handler — production'la aynı
//   - HTTP request helper'ları: cookie-aware GET/POST/PUT/DELETE
//   - "Login" yardımcıları: admin/user kullanıcısı yarat ve cookie döner
package testutil

import (
	"testing"

	"libra_management/internal/database"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewTestDB, her test için bağımsız in-memory SQLite veritabanı kurar.
// `:memory:` path'i süreç-içi tek bağlantıya bağlıdır; testler arası sızıntı
// yapmaz. Migrate + Seed çağrıları gerçek state'i aynen kuruyor — testler
// üretim ortamından izole değil, üretimle aynı schema/data üzerinde çalışır.
func NewTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger:         logger.Default.LogMode(logger.Silent), // test logu temiz kalsın
		TranslateError: true,
	})
	require.NoError(t, err, "open sqlite")

	require.NoError(t, database.Migrate(db), "migrate")
	require.NoError(t, database.Seed(db), "seed")
	return db
}
