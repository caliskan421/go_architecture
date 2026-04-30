# `TranslateError: true` — DB Hatasını Domain Hatasına Çevirmek

```go
db, _ := gorm.Open(mysql.Open(dsn), &gorm.Config{
    TranslateError: true,
})
```

Bu tek satırın işi büyük: handler'ın `errors.Is(err, gorm.ErrDuplicatedKey)` ile
hata yakalayabilmesini sağlar. Bu dosya: o tek satır olmasaydı ne olurdu, hangi
hatalar nelere çevrilir, niye bu pattern doğru.

---

## 1. Sorun: Driver Hataları Anlaşılmaz

MySQL bir kayıt çakışmasında şöyle bir hata döner:

```
Error 1062 (23000): Duplicate entry 'ali@example.com' for key 'users.email'
```

Bu Go tarafına `*mysql.MySQLError` tipi olarak gelir. Anlamak için:

```go
var mysqlErr *mysql.MySQLError
if errors.As(err, &mysqlErr) {
    if mysqlErr.Number == 1062 {
        // duplicate
    }
}
```

Sorunlar:

- Handler'ın MySQL driver'ına bağımlı olması gerekir.
- "1062" sihirli sayısını ezberlemek lazım.
- Postgres'e geçersen tüm error matching kodu kırılır (Postgres'de duplicate `23505`).
- Test'te DB sahte ise hatayı taklit etmek zor.

---

## 2. Çözüm: GORM Sentinel Hataları

GORM driver-agnostic hata sentinel'ları tanımlar:

```go
// gorm/errors.go
var (
    ErrRecordNotFound        = ...
    ErrDuplicatedKey         = ...  // unique constraint
    ErrForeignKeyViolated    = ...  // FK ihlali
    ErrCheckConstraintViolated = ...
    ErrInvalidTransaction    = ...
    ErrNotImplemented        = ...
    ...
)
```

`TranslateError: true` flag'i açıkken GORM driver hatasını bu sentinel'lerden uygun
olana çevirir:

| MySQL hatası                                         | Çevrildiği GORM sentinel'i      |
|------------------------------------------------------|---------------------------------|
| `1062` Duplicate entry                               | `gorm.ErrDuplicatedKey`         |
| `1452` Cannot add or update child row (FK)           | `gorm.ErrForeignKeyViolated`    |
| Standart "no rows" (driver-bağımsız)                 | `gorm.ErrRecordNotFound`        |

Postgres'e geçersen GORM Postgres'in error code'larını **aynı** sentinel'lere çevirir.
Handler kodu değişmez — driver-agnostic kalmıştır.

`ErrRecordNotFound` özeldir: GORM sorgu sonucu boşsa otomatik üretir, "translate" gerekmez.
Diğerleri için flag açık olmalı.

---

## 3. Flag Olmazsa Ne Olur?

`TranslateError` default olarak `false`:

```go
err := db.Create(&user).Error  // duplicate email
errors.Is(err, gorm.ErrDuplicatedKey)  // <- false! sentinel çevrilmedi
```

Switch case'in `case errors.Is(err, gorm.ErrDuplicatedKey):` dalına hiç düşmez.
`default` dalına düşer:

```go
case err != nil:
    return httpx.ErrInternal.WithErr(err)  // 500 dönüyor
```

Yani client `409 Conflict` yerine `500 Internal Server Error` görür. Yanlış status
code, yanlış mesaj.

---

## 4. Handler -> Frontend Akışı

Hatanın yolculuğunu uçtan uca:

```
1. MySQL: "Error 1062"
        |
        v
2. GORM (TranslateError: true): "duplicate, ErrDuplicatedKey'e çevireyim"
        |
        v
3. Handler: "errors.Is ile yakaladım, domain hatasına çevireyim"
        |
        v
4. Central error handler (httpx.Handler): "AppError'ın HTTPStatus'u 409, JSON'a yaz"
        |
        v
5. Frontend: 409 gördü, kullanıcıya "kayıt zaten var" mesajı
```

`TranslateError` sadece 2. adımı etkiler. Ama 2 çalışmazsa 3 hatayı tanıyamaz, 4
yanlış status yazar, 5 yanlış mesaj gösterir.

---

## 5. Yanlış vs Doğru Yanıt Karşılaştırma

Flag kapalıyken (yanlış):

```json
{
  "error": {
    "code": "internal_error",
    "message": "iç sunucu hatası"
  }
}
```

500 + generic mesaj. Operatöre haksız bir alarm, kullanıcıya bilgisiz mesaj.

Flag açıkken (doğru):

```json
{
  "error": {
    "code": "conflict",
    "message": "Kaynak mevcut bir kayıtla çakışıyor."
  }
}
```

409 + anlamlı mesaj. Frontend "bu email zaten var" deyip kullanıcıya gösterebilir.

---

## 6. Niye Sentinel Pattern? — `errors.Is`

Hata karşılaştırmasında üç yöntem vardır:

```go
// 1. String match -> KÖTÜ
if strings.Contains(err.Error(), "duplicate") { ... }
// kütüphane mesajını değiştirirse kırılır

// 2. Tip kontrolü -> sınırlı
if _, ok := err.(*mysql.MySQLError); ok { ... }
// sarmalanmış hatalarda çalışmaz

// 3. errors.Is + sentinel -> DOĞRU
if errors.Is(err, gorm.ErrDuplicatedKey) { ... }
// hata zincirinde her yerde bulur
```

`errors.Is` zincire iner: `fmt.Errorf("create: %w", err)` ile sarmalanmış olsa bile
sentinel'i bulur. GORM API'sinde sözleşme `errors.Is` üzerinden tanımlanmıştır.

Bizim kendi `httpx.AppError`'umuz da aynı pattern'i kullanır (bkz. `apperror-pattern.md`).

---

## 7. Performans

`TranslateError: true` her hata için ek bir tip kontrolü ve switch yapar. Maliyeti
ihmal edilebilir (mikrosaniye altı), ama hatasız sorguları **etkilemez** — sadece
hata yolunda devreye girer. Ses yapma, açık bırak.

---

## 8. Bizim Kodda Kullanım Yerleri

```go
// handler/auth.go
case errors.Is(err, gorm.ErrDuplicatedKey):
    return httpx.ErrConflict   // 409

// handler/auth.go
case errors.Is(err, gorm.ErrRecordNotFound):
    _ = bcrypt.CompareHashAndPassword(h.dummyHash, []byte(req.Password))
    return httpx.ErrInvalidCredentials  // 401

// handler/book.go (Update'de FK hatası)
// Burada kullanmıyoruz çünkü FK ihlalini önceden authorExists ile yakalıyoruz;
// ama yakalamasaydık ErrForeignKeyViolated için 422/400 dönmek mantıklı olurdu.
```

---

## Tek Cümle

`TranslateError: true` driver'a-özel hata kodlarını (MySQL 1062, Postgres 23505 vs.)
GORM'un driver-agnostic sentinel'lerine (`ErrDuplicatedKey`, `ErrForeignKeyViolated`,
`ErrRecordNotFound`) çevirir; bu sayede handler `errors.Is(err, gorm.ErrXxx)` deyimiyle
hatayı yakalayıp doğru HTTP status'ünü dönebilir — flag'siz çalışırsa duplicate
insert'lerin tamamı 500 olarak görünür, çünkü 2. adım atlanmış demektir.
