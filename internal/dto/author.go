package dto

import "libra_management/internal/model"

// CreateAuthorRequest, POST /api/authors body'sinin tipli halidir.
// Name zorunlu (min 2, max 191) ve DB seviyesinde uniqueIndex'li — aynı isimle
// ikinci insert MySQL Error 1062 fırlatır; handler bunu httpx.ErrConflict'e çevirir.
// Description opsiyonel; max=10000 ile abuse/DoS riskine üst sınır koyduk.
type CreateAuthorRequest struct {
	Name        string `json:"name" validate:"required,min=2,max=191"`
	Description string `json:"description" validate:"max=10000"`
}

// UpdateAuthorRequest, PUT /api/authors/:id body'sidir.
// Şu an CreateAuthorRequest ile alan-alan aynı görünüyor — bu BİLİNÇLİ bir ayrım,
// "DRY ihlali" değil:
//   - İki farklı operasyonun sözleşmesi (create vs update) bağımsız evrimleşmeli
//   - İleride PATCH semantiğine geçilirse (omitempty + partial update) Create'i bozmadan değişir
//   - Unique-name kontrolünde Update kendi kaydını dışlar (WHERE id != ?), kural farklılaşabilir
//
// type alias / embed ile birleştirmek bugünkü 4 satırı kurtarır, yarın söküm maliyeti yaratır.
type UpdateAuthorRequest struct {
	Name        string `json:"name" validate:"required,min=2,max=191"`
	Description string `json:"description" validate:"max=10000"`
}

// AuthorResponse, dış dünyaya dönen author şeklidir.
// model.Author doğrudan JSON'a dönmez çünkü gorm.Model embed'i CreatedAt/UpdatedAt/
// DeletedAt gibi internal alanları sızdırır ve API kontratı model değişikliğine bağımlı olur.
// CreatedAt/UpdatedAt şimdilik dahil edilmedi — UserResponse ile tutarlı kalmak için;
// ihtiyaç olursa Faz 6'da genel pattern olarak eklenecek.
type AuthorResponse struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ToAuthorResponse, model'den DTO'ya dönüşüm yapan mapper.
// dto/auth.go:ToUserResponse ile aynı desen — mapper'ı DTO paketinde tutmak,
// "veri sözleşmesi" ile "modele bakış"ı tek dosyada toplar.
func ToAuthorResponse(a model.Author) AuthorResponse {
	return AuthorResponse{
		ID:          a.ID,
		Name:        a.Name,
		Description: a.Description,
	}
}
