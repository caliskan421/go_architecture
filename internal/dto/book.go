package dto

import "libra_management/internal/model"

// CreateBookRequest, POST /api/books body'sidir.
//
// AuthorID: gt=0 ile sıfır/negatif değer engellenir; ayrıca handler katmanında
// "AuthorID DB'de var mı?" kontrolü yapılır (FK validation). Validator yalnızca
// formatı doğrular, varlık kontrolü uygulama mantığıdır.
type CreateBookRequest struct {
	AuthorID    uint   `json:"author_id" validate:"required,gt=0"`
	Name        string `json:"name" validate:"required,min=2,max=191"`
	Description string `json:"description" validate:"max=10000"`
}

// UpdateBookRequest, PUT /api/books/:id body'si.
// Author değişimi de izin veriliyor (kitap farklı yazara taşınabilir).
type UpdateBookRequest struct {
	AuthorID    uint   `json:"author_id" validate:"required,gt=0"`
	Name        string `json:"name" validate:"required,min=2,max=191"`
	Description string `json:"description" validate:"max=10000"`
}

// BookResponse, dış dünyaya dönen book şeklidir.
// Author bilgisi nested AuthorResponse olarak dönüyor — client tek istekte
// hem book hem author görebilsin diye. Preload edilmemiş Author boş gelirse
// AuthorResponse'un alanları sıfır değer dönecek; caller Preload sorumluluğunda.
type BookResponse struct {
	ID          uint           `json:"id"`
	AuthorID    uint           `json:"author_id"`
	Author      AuthorResponse `json:"author"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
}

// ToBookResponse, model'den DTO'ya dönüştürür.
// b.Author boşsa (Preload edilmemişse) AuthorResponse zero-value döner.
func ToBookResponse(b model.Book) BookResponse {
	return BookResponse{
		ID:          b.ID,
		AuthorID:    b.AuthorID,
		Author:      ToAuthorResponse(b.Author),
		Name:        b.Name,
		Description: b.Description,
	}
}
