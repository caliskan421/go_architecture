# Binding ve Validation: HTTP'den Struct'a Güvenli Yol

HTTP request body'sinden Go struct'a iki ayrı kontrol gerekir: **format** (parse
edilebilir mi) ve **anlam** (iş kurallarına uyuyor mu). Fiber'in `Bind` metodu birinciyi,
`validator/v10` ikincisini yapar.

---

## 1. İki Aşamalı Süreç

```
HTTP Body (JSON string)
    |
    v
[1. BIND]      JSON parse, struct'a doldur
    |
    v
[2. VALIDATE]  iş kuralları (email format, min length, eqfield, vs.)
    |
    v
Güvenli struct (kullanıma hazır)
```

İlki **format kontrolü**, ikincisi **anlam kontrolü**.

---

## 2. Bind: Veri Aktarımı

```go
var req dto.RegisterRequest
if err := c.Bind().Body(&req); err != nil {
    return httpx.ErrBadRequest.WithErr(err)
}
```

Fiber'in `Bind` metodu:

1. `Content-Type` header'ına bakar (`application/json`, `application/xml` vs.).
2. Body'i parse eder (JSON ise `json.Unmarshal`).
3. JSON tag'lerine göre struct'a doldurur.

```go
type RegisterRequest struct {
    Email    string `json:"email"`
    Password string `json:"password"`
}
```

`{"email":"ali@x.com"}` gelince `req.Email = "ali@x.com"` olur.

### Bind Hata Durumları

| Hata sebebi                                    | Örnek                          |
|------------------------------------------------|--------------------------------|
| Bozuk JSON                                     | `{"email":}`                   |
| Yanlış tip                                     | `{"email": 123}` (string bekleniyor) |
| Eksik kapanış                                  | `{"email":"a"`                 |
| Desteklenmeyen Content-Type                    | `text/plain` body              |

Bunlar **400 Bad Request** olarak döner — sorun istemcide, gönderdiği veri yapısal
olarak hatalı.

### Bind İçin Pointer Şart

```go
c.Bind().Body(&req)  // doğru
c.Bind().Body(req)   // yanlış: kütüphane req'i değiştiremez
```

`Bind` parse ettiği değeri yazmak için struct adresine ihtiyaç duyar. Pointer geçmezsen
copy üzerinde çalışır, sen orijinal `req`'i boş görürsün.

---

## 3. Validate: İş Kuralları

```go
if err := h.val.Struct(&req); err != nil {
    return httpx.ErrValidation.WithDetails(validate.Format(err))
}
```

Bind başarılı olsa bile `req.Email = ""` olabilir (zorunlu mu?), `req.Password = "a"`
olabilir (yeterince uzun mu?). Bunları **iş kuralı** seviyesinde doğrularız.

### Validation Tag'leri

```go
type RegisterRequest struct {
    Email    string `json:"email"    validate:"required,email"`
    Password string `json:"password" validate:"required,min=8"`
    Confirm  string `json:"confirm"  validate:"eqfield=Password"`
}
```

Bu projede sık kullanılan tag'ler:

| Tag                          | Anlam                              |
|------------------------------|------------------------------------|
| `required`                   | Boş olamaz (nil/"" reddedilir)     |
| `email`                      | RFC 5322 email formatı             |
| `min=8`                      | String için en az 8 karakter, sayı için >= 8 |
| `max=100`                    | En fazla 100 karakter / değer      |
| `eqfield=X`                  | Aynı struct'taki X alanına eşit olmalı |
| `gte=0`                      | Sayı: >= 0                          |
| `lte=150`                    | Sayı: <= 150                        |
| `oneof=admin user moderator` | Sadece bu değerlerden biri          |

Tüm kurallar **deklaratif** (tag olarak) yazılır. Manuel `if` zinciri yok, tekrar yok.

Geniş referans: `pkg/validate/validator-tags.md`.

---

## 4. Niye İki Aşama? Tek Aşamada Olmaz mı?

Olabilir ama ayırmak daha sağlıklı:

| Sebep               | Açıklama                                        |
|---------------------|-------------------------------------------------|
| Farklı hata kodları | Bind hatası 400 (bad request), validation 400 (validation_error) ama farklı code/details |
| Farklı mesajlar     | "JSON bozuk" vs. "email formatı yanlış"         |
| Erken çıkış         | Bind hatalıysa validation'a girmeye gerek yok   |
| Sorumluluk ayrımı   | Bind = framework işi, Validate = iş mantığı     |
| Test edilebilirlik  | Validator instance'ı tek başına test edilebilir |

---

## 5. DTO + Validation = Kontrat

DTO (Data Transfer Object) + validation tag'leri birlikte API'nin **kontratıdır**:

```go
type RegisterRequest struct {
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required,min=8"`
}
```

Bu struct hem belge hem savunma:

- **Belge**: Frontend bu struct'a bakar, "email zorunlu, email formatında olmalı,
  password en az 8 karakter" anlar. Swagger/OpenAPI üretimi de buradan yapılır.
- **Savunma**: Aynı kural backend'de **fiilen** uygulanır. Frontend uygulamasa bile
  validation backend'de yine yakalar.

---

## 6. Bind/Validate Sonrası Ne Var?

Bu noktada `req` **güvenli ve geçerli**. İş mantığına geçebilirsin:

```go
hashed, _ := bcrypt.GenerateFromPassword([]byte(req.Password), 14)
user := model.User{Email: req.Email, Password: string(hashed)}
db.Create(&user)
```

Validation'dan geçmeyen veri buraya **asla** ulaşmaz; o yüzden iç katmanlar tekrar
boş/format kontrolü yapmak zorunda değil.

---

## 7. DTO -> Model Ayrımı

Üç ayrı tip:

```go
req  := dto.RegisterRequest{}    // dış dünyadan gelen (input DTO)
user := model.User{}              // DB'de saklanan (domain model)
resp := dto.ToUserResponse(user)  // dış dünyaya giden (output DTO)
```

Niye üç ayrı tip?

| Sebep         | Senaryo                                                              |
|---------------|----------------------------------------------------------------------|
| Güvenlik      | `model.User`'da `Password` var. JSON'a direkt serialize edersen şifre sızar. DTO sadece güvenli alanları açar. |
| Esneklik      | DB şeması değişse de API kontratı korunur                            |
| Versiyonlama  | API v1/v2 farklı DTO'lar, model tek                                  |
| Validation    | Tag'ler yalnızca girdiye ait — model tag'lerle kirletilmez           |

---

## 8. Validator Instance'ı Niye Boot'ta Kuruluyor?

```go
// main.go
val := validate.New()
authHandler, _ := handler.NewAuthHandler(db, cfg, tokenMgr, val)
authorHandler  := handler.NewAuthorHandler(db, val)
// ...
```

`*validator.Validate` thread-safe ve struct meta'sını **cache'ler**. İlk
`val.Struct(&req)` çağrısında `RegisterRequest`'in tag'lerini reflection ile okur,
sonraki çağrılarda cache'ten kullanır. Her istekte yeni instance oluşturmak bu
cache'i sıfırlar -> her istekte reflection -> yavaşlar.

Çözüm: bir instance, her yere DI ile geçir. Tipik DI pattern (bkz. `pkg/token/di.md`).

---

## 9. `validate.Format` Niye Var?

Validator `validator.ValidationErrors` döner; bu raw bir slice — frontend'e direkt
göndermek istemediğimiz iç tipler içerir. `Format`:

```go
func Format(err error) []FieldError {
    var verrs validator.ValidationErrors
    if !errors.As(err, &verrs) {
        return nil
    }
    out := make([]FieldError, 0, len(verrs))
    for _, e := range verrs {
        out = append(out, FieldError{
            Field:   e.Field(),
            Message: messageFor(e),
        })
    }
    return out
}
```

İki iş yapıyor:

1. Kütüphane tipini bizim `[]FieldError`'a indirgiyor (predictable JSON).
2. Türkçe mesaj üretiyor (`messageFor`).

Bunun çıktısı `httpx.ErrValidation.WithDetails(...)` ile response'a iliştirilir:

```json
{
  "error": {
    "code": "validation_error",
    "message": "girdi doğrulanamadı",
    "details": [
      { "field": "Email",    "message": "geçerli bir e-posta giriniz" },
      { "field": "Password", "message": "en az 8 karakter olmalıdır" }
    ]
  }
}
```

---

## Tek Cümle

Bind = "JSON'u struct'a parse et" (format kontrolü), Validate = "alanlar iş kurallarına
uyuyor mu?" (anlam kontrolü); ikisi peş peşe çalışır, ikisinden de geçen veri handler'ın
**güvenli inputu** olur — DTO + validation tag'leri birlikte API'nin hem belgesi hem
savunmasıdır, bu yüzden frontend'in yaptığı validation'ı backend mutlaka **tekrar** yapar.
