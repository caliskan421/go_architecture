package model

import "time"

// Author, kütüphanedeki yazarları temsil eder.
//
// gorm.Model BİLİNÇLİ kullanılmadı: gorm.Model'in DeletedAt'ı soft-delete açar.
// MySQL'in unique index'i deleted_at'i bilmediği için "silinmiş 'Yaşar Kemal'"
// satırı INSERT'i bloklar — list boş gözükür ama Create 409 döner (UX çelişkisi).
// Audit/restore ihtiyacı doğarsa Faz 8'de event-log ile çözülecek; şimdilik hard-delete.
type Author struct {
	ID          uint `gorm:"primaryKey"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Name        string `gorm:"uniqueIndex;size:191"`
	Description string `gorm:"type:text"`
}
