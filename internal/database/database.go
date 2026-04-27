package database

import (
	"fmt"
	"log"

	"libra_management/internal/config"
	"libra_management/internal/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Open, GORM ile MySQL bağlantısını açar. DSN cfg.DBDSn'den okunur.
// Hata dönerse caller (main) log.Fatalf ile yakalar.
func Open(cfg *config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(cfg.DBDSn), &gorm.Config{
		// Bu olmadan errors.Is(err, gorm.ErrDuplicatedKey) her zaman false döner → 409 yerine 500 dönerdik.
		TranslateError: true,
	})
	if err != nil {
		return nil, fmt.Errorf("gorm.Open: %w", err)
	}
	return db, nil
}

// Migrate, uygulamanın ihtiyaç duyduğu TÜM modelleri tablolara dönüştürür.
// AutoMigrate idempotent: tablo varsa kolonları senkronize eder, yoksa oluşturur.
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.Permission{},
		&model.Role{},
		&model.User{},
		&model.Author{},
		&model.Book{},
		&model.Library{},
	)
}

// Seed, uygulamanın çalışması için DB'de var olması ZORUNLU default kayıtları ekler.
// Şu an yalnızca rolleri seed ediyoruz; permission'lar Faz 5'te eklenecek.
// Idempotent: birden çok boot çalıştırıldığında aynı role tekrar yaratılmaz.
func Seed(db *gorm.DB) error {
	defaults := []model.Role{
		{Title: "admin"},
		{Title: "user"},
	}
	for i := range defaults {
		// FirstOrCreate: WHERE Title=... ile arar; bulamazsa create.
		// &defaults[i]: ID'nin slice'a geri yazılması için pointer-to-element.
		if err := db.Where(model.Role{Title: defaults[i].Title}).
			FirstOrCreate(&defaults[i]).Error; err != nil {
			return fmt.Errorf("seed role %q: %w", defaults[i].Title, err)
		}
		log.Printf("seed: role hazır → %s (id=%d)", defaults[i].Title, defaults[i].ID)
	}
	return nil
}
