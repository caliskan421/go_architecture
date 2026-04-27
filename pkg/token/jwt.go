package token

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims, JWT'nin payload kısmında taşıdığımız uygulama-spesifik alanlardır.
// jwt.RegisteredClaims gömülmesi sayesinde "exp", "iat" gibi IANA standart claim'leri de
// otomatik dahil olur ve jwt kütüphanesi expiry kontrolünü kendisi yapar.
type Claims struct {
	UserID uint
	RoleID uint
	jwt.RegisteredClaims
}

// Manager, JWT üretimi ve doğrulamasını kapsülleyen DI'lı yapıdır.
// Secret ve TTL artık os.Getenv'dan değil, constructor'dan alınır.
// Bu sayede test'te farklı bir Manager kurabilir, prod'da farklı kurabilirsiniz.
// / Neden struct? Çünkü secret ve TTL "uygulama başlarken bir kere belirlenir, sonra her yerde aynı kalır" şeklinde davranan state.
type Manager struct {
	secret []byte        // []byte olarak tutulur çünkü jwt API'si onu istiyor — her çağrıda dönüşüm yapmamak için
	ttl    time.Duration // token'ın geçerlilik süresi
}

// New, boot'ta bir kez çağrılır ve uygulama yaşam süresince kullanılacak Manager'ı döner.
func New(secret string, ttl time.Duration) *Manager {
	return &Manager{
		secret: []byte(secret),
		ttl:    ttl,
	}
}

// Generate, verilen kullanıcı için imzalanmış bir JWT string'i döner.
// Algoritma HS256 (HMAC + SHA-256) — symmetric, küçük servisler için yeterli ve hızlı.
func (m *Manager) Generate(userID, roleID uint) (string, error) {
	claims := Claims{
		UserID: userID,
		RoleID: roleID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(m.secret)
}

// Parse, token string'ini doğrular ve Claims'i geri döner.
// GÜVENLİK: signing-method doğrulaması zorunlu. Saldırgan header'da "alg":"none" ya da
// asymmetric "RS256" gönderip imzasız/sahte token kabul ettirmeye çalışabilir
// (algorithm-confusion attack). HMAC ailesinden olduğunu type-assertion ile garantiliyoruz.
func (m *Manager) Parse(tokenString string) (*Claims, error) {
	t, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		// "keyfunc": jwt kütüphanesi imzayı doğrulamak için secret'ı bizden ister.
		// Secret'ı VERMEDEN ÖNCE algoritmanın güvendiğimiz türde olduğunu doğrularız.
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("beklenmeyen imza yöntemi: %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}

	// Tip güvenli claim erişimi: kütüphane interface döner, biz Claims*'a cast ederiz.
	claims, ok := t.Claims.(*Claims)
	if !ok || !t.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

