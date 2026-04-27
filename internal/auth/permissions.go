// Package auth, yetkilendirme (authorization) için ortak sabitleri ve
// yardımcı slice'ları barındırır. RBAC stringlerinin "magic value" olarak
// dağılmaması için tek bir doğruluk noktası burası — typo bug'larını derleme
// zamanına çekmek için sabit olarak tanımlandılar.
//
// Konvansiyon: <kaynak>:<aksiyon>
//
//	read   → list/get
//	write  → create/update + alt-route mutasyonları
//	delete → silme
//
// İleride daha granüler hale gelebilir (örn. "library:books:write" ayrı bir
// izin); şimdilik kaynak bazında üçlü pattern yeterli.
package auth

const (
	PermAuthorRead   = "author:read"
	PermAuthorWrite  = "author:write"
	PermAuthorDelete = "author:delete"

	PermBookRead   = "book:read"
	PermBookWrite  = "book:write"
	PermBookDelete = "book:delete"

	PermLibraryRead   = "library:read"
	PermLibraryWrite  = "library:write"
	PermLibraryDelete = "library:delete"
)

// AllPermissions, sistemde tanımlı tüm izinlerin listesi.
// Seed sırasında DB'ye eklenir; admin role'üne hepsi bağlanır.
var AllPermissions = []string{
	PermAuthorRead, PermAuthorWrite, PermAuthorDelete,
	PermBookRead, PermBookWrite, PermBookDelete,
	PermLibraryRead, PermLibraryWrite, PermLibraryDelete,
}

// UserPermissions, default "user" role'üne verilecek izinler — sadece okuma.
// Bu sözleşmenin değişmesi (örn. user'lar Book yaratabilsin) gerekirse burası
// güncellenir; davranış değişikliği DB seed'iyle yansır.
var UserPermissions = []string{
	PermAuthorRead,
	PermBookRead,
	PermLibraryRead,
}
