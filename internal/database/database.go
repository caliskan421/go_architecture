package database

import (
	"fmt"
	"log"

	"libra_management/internal/auth"
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
// Sıra ÖNEMLİ:
//  1. Permissions  — atomic izin satırları
//  2. Roles        — admin/user
//  3. Role↔Permission bağı — admin'e tümü, user'a okuma izinleri
//
// Idempotent: tekrar çalıştırıldığında çift kayıt oluşturmaz, ama her boot'ta
// rol-izin ilişkisini yeniden senkronize eder (kod listesi DB'ye yansır).
func Seed(db *gorm.DB) error {
	perms, err := seedPermissions(db)
	if err != nil {
		return fmt.Errorf("seed permissions: %w", err)
	}

	roles, err := seedRoles(db)
	if err != nil {
		return fmt.Errorf("seed roles: %w", err)
	}

	if err := syncRolePermissions(db, roles, perms); err != nil {
		return fmt.Errorf("sync role-permission: %w", err)
	}
	return nil
}

// seedPermissions, auth.AllPermissions listesindeki her izni DB'ye yazar.
// Map dönüyor: name -> Permission (id'siyle); ilişki kurarken arayışı kolaylaştırır.
func seedPermissions(db *gorm.DB) (map[string]model.Permission, error) {
	out := make(map[string]model.Permission, len(auth.AllPermissions))
	for _, name := range auth.AllPermissions {
		p := model.Permission{Permission: name}
		if err := db.Where(model.Permission{Permission: name}).
			FirstOrCreate(&p).Error; err != nil {
			return nil, fmt.Errorf("permission %q: %w", name, err)
		}
		out[name] = p
	}
	return out, nil
}

// seedRoles, default rolleri (admin/user) yaratır ve title -> Role map'i döner.
func seedRoles(db *gorm.DB) (map[string]model.Role, error) {
	titles := []string{"admin", "user"}
	out := make(map[string]model.Role, len(titles))
	for _, t := range titles {
		r := model.Role{Title: t}
		if err := db.Where(model.Role{Title: t}).FirstOrCreate(&r).Error; err != nil {
			return nil, fmt.Errorf("role %q: %w", t, err)
		}
		log.Printf("seed: role hazır → %s (id=%d)", r.Title, r.ID)
		out[t] = r
	}
	return out, nil
}

// syncRolePermissions, rol→izin ilişkisini kod tarafındaki listeye eşitler.
// Replace semantiği: kod ne diyorsa DB o olur. Manual olarak DB'den izin
// eklenmişse bu Seed onu KALDIRIR — bu kasıtlı: tek doğruluk noktası kod.
//
// Eğer ileride dinamik admin paneliyle "ad-hoc izin" eklenecekse bu Seed
// re-design ister; bugünkü model "rol-izin sözleşmesi statiktir".
func syncRolePermissions(db *gorm.DB, roles map[string]model.Role, perms map[string]model.Permission) error {
	want := map[string][]string{
		"admin": auth.AllPermissions,
		"user":  auth.UserPermissions,
	}
	for title, names := range want {
		role := roles[title]
		set := make([]model.Permission, 0, len(names))
		for _, n := range names {
			p, ok := perms[n]
			if !ok {
				return fmt.Errorf("role %q: bilinmeyen permission %q", title, n)
			}
			set = append(set, p)
		}
		if err := db.Model(&role).Association("Permissions").Replace(set); err != nil {
			return fmt.Errorf("role %q permissions replace: %w", title, err)
		}
		log.Printf("seed: role %q → %d izin senkronize", title, len(set))
	}
	return nil
}
