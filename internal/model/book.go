package model

import "time"

// Book, kütüphanedeki kitabı temsil eder.
//
// gorm.Model BİLİNÇLİ kullanılmadı — Author ile aynı gerekçe:
// soft-delete (DeletedAt) + uniqueIndex(name) MySQL'de hatalı UX yaratır
// (silinmiş "Suç ve Ceza" duruyorken yenisini insert 1062 verir).
//
// İlişki:
//   - Author → Book : 1-to-many. AuthorID FK; gorm "foreignKey:AuthorID" ile bağlanır.
//   - Library → Book: many-to-many (library_books) — Library tarafında tanımlı.
type Book struct {
	ID          uint `gorm:"primaryKey"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	AuthorID    uint   `gorm:"index;not null"` // FK; not null DB seviyesinde de zorlasın
	Author      Author `gorm:"foreignKey:AuthorID"`
	Name        string `gorm:"uniqueIndex;size:191"`
	Description string `gorm:"type:text"`
}
