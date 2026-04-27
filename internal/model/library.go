package model

import "time"

// Library, kitap koleksiyonu (kütüphane) — Book ile many-to-many ilişki tutar.
//
// gorm.Model BİLİNÇLİ kullanılmadı (Author/Book ile aynı gerekçe).
// M2M ilişki tablosu: library_books (gorm tarafından otomatik üretilir).
type Library struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Name      string `gorm:"uniqueIndex;size:191"`
	Books     []Book `gorm:"many2many:library_books;"`
}
