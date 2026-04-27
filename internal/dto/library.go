package dto

import "libra_management/internal/model"

// CreateLibraryRequest, POST /api/libraries body'sidir.
//
// BookIDs opsiyonel: kütüphane boş açılabilir. Verilmişse handler tarafında
// "tüm id'ler DB'de var mı?" toplu kontrolü yapılır (FK validation).
// `dive,gt=0` her bir element'in pozitif olduğunu doğrular.
type CreateLibraryRequest struct {
	Name    string `json:"name" validate:"required,min=2,max=191"`
	BookIDs []uint `json:"book_ids" validate:"dive,gt=0"`
}

// UpdateLibraryRequest, PUT /api/libraries/:id body'si.
// PUT semantiği: BookIDs verilmişse mevcut M2M ilişki TAMAMEN değiştirilir
// (Replace). Boş slice `[]` gelirse kütüphane boşaltılır. Hiç gelmezse
// (`book_ids` field yok) ilişkiye dokunulmaz — bu davranış için pointer kullandık.
type UpdateLibraryRequest struct {
	Name    string  `json:"name" validate:"required,min=2,max=191"`
	BookIDs *[]uint `json:"book_ids" validate:"omitempty,dive,gt=0"`
}

// LibraryBookIDsRequest, alt-route'lar için ortak body:
//
//	POST   /api/libraries/:id/books   body: {"book_ids":[1,2]}  -> Append
//	DELETE /api/libraries/:id/books   body: {"book_ids":[3]}    -> Remove
//
// "required" + "min=1" + "dive,gt=0": en az bir id gelmeli, hepsi pozitif olmalı.
type LibraryBookIDsRequest struct {
	BookIDs []uint `json:"book_ids" validate:"required,min=1,dive,gt=0"`
}

// LibraryResponse, dış dünyaya dönen library şeklidir.
// Books her zaman embed dönüyor — client tek istekte koleksiyonu görsün.
// Empty M2M -> `"books": []` (nil değil). Mapper bunu garanti eder.
type LibraryResponse struct {
	ID    uint           `json:"id"`
	Name  string         `json:"name"`
	Books []BookResponse `json:"books"`
}

// ToLibraryResponse, model'den DTO'ya dönüştürür.
// Books slice'ı boş bile olsa nil yerine boş slice döner ki JSON `[]` çıksın.
func ToLibraryResponse(l model.Library) LibraryResponse {
	books := make([]BookResponse, len(l.Books))
	for i, b := range l.Books {
		books[i] = ToBookResponse(b)
	}
	return LibraryResponse{
		ID:    l.ID,
		Name:  l.Name,
		Books: books,
	}
}
