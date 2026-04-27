package dto

import "libra_management/internal/model"

// RegisterRequest, /api/register endpoint'ine gelen body'nin tipli halidir.
// Eski handler map[string]string kullanıyordu — type-unsafe ve sözleşme dışı.
// Burada alanlar + validation kuralları struct tag'leriyle açıkça tanımlı:
//   - "json": HTTP body parse edilirken hangi alanı hangi key'den okuyacağını söyler
//   - "validate": validator/v10'un uygulayacağı kuralları belirtir
type RegisterRequest struct {
	FirstName       string `json:"first_name" validate:"required,min=2"`
	LastName        string `json:"last_name" validate:"required,min=2"`
	Email           string `json:"email" validate:"required,email"`
	Password        string `json:"password" validate:"required,min=8"`             // güç politikası: en az 8 karakter
	PasswordConfirm string `json:"password_confirm" validate:"required,eqfield=Password"` // eşleşme kontrolü artık validator işi
}

// LoginRequest, /api/login body'si.
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// UserResponse, dış dünyaya gösterilen kullanıcı şeklidir.
// model.User'ı ASLA doğrudan döndürmüyoruz çünkü:
//   - Password (hash bile olsa) sızabilir
//   - gorm.Model'in DeletedAt gibi internal alanları client'ı ilgilendirmez
//   - Field isimleri/casing'i değişirse API kontratı kırılır
// DTO bu sızıntıları engelleyen bir "filtre" görevi görür.
type UserResponse struct {
	ID        uint   `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	RoleID    uint   `json:"role_id"`
	RoleTitle string `json:"role_title"`
}

// ToUserResponse, model'den DTO'ya dönüşüm yapan mapper.
// Mapper'ları DTO paketinde tutmak Go'da yaygın bir desen:
// "veri sözleşmesi" ve "veri sözleşmesinden modele bakış" aynı yerde durur.
func ToUserResponse(u model.User) UserResponse {
	return UserResponse{
		ID:        u.ID,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Email:     u.Email,
		RoleID:    u.RoleID,
		RoleTitle: u.Role.Title, // u.Role boşsa "" dönecek — caller Preload sorumluluğuna sahip
	}
}
