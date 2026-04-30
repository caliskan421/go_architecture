# GORM'da Preload, Joins ve Manuel Atama: İlişkili Veriyi Çekmenin Üç Yolu

İlişkili veri (User -> Role, Book -> Author, Library -> Books) çekerken sorgu
sayısı = round-trip sayısıdır; dolayısıyla maliyettir. Üç yöntemin **performansı,
trade-off'u, ne zaman hangisini seçileceği** bu dosyada.

---

## 1. Senaryo

İki ilişkili tablo:

```go
type User struct {
    ID     uint
    Name   string
    RoleID uint
    Role   Role  // ilişki
}

type Role struct {
    ID    uint
    Title string
}
```

User'ı çektiğinde Role boş gelir:

```go
var user User
db.First(&user, 1)
// user.Role == Role{} (boş!)
```

Role bilgisini doldurmanın yolları: `Preload`, `Joins`, **manuel atama**.

---

## 2. Preload — İki Sorgu

```go
db.Preload("Role").First(&user, 1)
```

Perde arkasında **iki ayrı SQL sorgusu** atılır:

```sql
-- 1. Sorgu
SELECT * FROM users WHERE id = 1;
-- user.RoleID = 2 öğrenildi

-- 2. Sorgu
SELECT * FROM roles WHERE id IN (2);
-- role bilgisi alındı

-- Go tarafında GORM ikisini birleştirir
```

İki sorgu = iki **database round-trip**. Her round-trip:

- Network gidiş-dönüş (lokal: ~0.5ms, remote: 5-50ms)
- Connection pool'dan connection alma
- Query parsing
- Sorgu yürütme
- Sonuç deserialize

Tek bir user için 2 sorgu görece az gibi, ama liste alırken katlanıyor (aşağıda
N+1).

---

## 3. Joins — Tek Sorgu

```go
db.Joins("Role").First(&user, 1)
```

**Tek SQL** sorgusu:

```sql
SELECT u.*, r.*
FROM users u
LEFT JOIN roles r ON u.role_id = r.id
WHERE u.id = 1;
```

Tek round-trip -> daha hızlı. Ama:

- Sonuç geniş bir satır olur (User + Role kolonları yan yana). Çok kolonlu tablolarda
  ağırlaşır.
- Çok katmanlı ilişkilerde (`A.B.C`) join sayısı artar; SQL planlaması zorlaşır,
  sonuç kümesi büyür.

---

## 4. Manuel Atama — Sıfır Sorgu

İlişki bilgisini **zaten biliyorsan**, DB'ye sormaya gerek yok:

```go
// auth.go Register'da:
db.Create(&user)  // user yaratıldı
user.Role = model.Role{
    ID:    h.defaultRoleID,        // boot'ta cache'lendi
    Title: h.cfg.DefaultRoleTitle, // config'de yazıyor
}
return httpx.Success(c, 201, dto.ToUserResponse(user))
```

Avantaj: 0 ekstra sorgu. Dezavantaj: yalnızca verinin tamamı senin elindeyse mümkün.
Cache pattern ile bağlantısı için bkz. `cache.md`.

---

## 5. N+1 Problemi — Kötü Senaryo

100 user listesi çekelim, her birinin role'üne ihtiyacımız var.

### Naif (felaket)

```go
var users []User
db.Find(&users)  // 1 sorgu

for i := range users {
    db.First(&users[i].Role, users[i].RoleID)  // her user için 1 sorgu
}
// Toplam: 1 + 100 = 101 sorgu
```

100 user 101 round-trip -> production'da uygulamanı dize getirir. "N+1" adı buradan:
1 ana sorgu, N tane çocuk sorgu.

### Preload — İyileştirilmiş

```go
db.Preload("Role").Find(&users)
-- 1. SORGU: SELECT * FROM users
-- 2. SORGU: SELECT * FROM roles WHERE id IN (1,2,3,...)
-- Toplam: 2 sorgu
```

GORM çocuk sorgusunu **IN clause** ile gruplar; round-trip 101'den 2'ye düşer.

### Joins — En Verimli

```go
db.Joins("Role").Find(&users)
-- Tek SQL: SELECT u.*, r.* FROM users u LEFT JOIN roles r ...
-- Toplam: 1 sorgu
```

---

## 6. Performans Karşılaştırması

| Yöntem                       | 1 user | 100 user |
|------------------------------|--------|----------|
| Naif loop                    | 1 + 1 = 2 | 1 + 100 = 101 |
| `Preload`                    | 1 + 1 = 2 | 1 + 1 = 2     |
| `Joins`                      | 1         | 1             |
| Manuel atama (veri elindeyse)| 0 ekstra  | 0 ekstra      |

---

## 7. Hangisini Ne Zaman?

| Durum                                          | Yöntem                          |
|------------------------------------------------|---------------------------------|
| İlişkili veri zaten elinde                     | Manuel atama                    |
| Performans kritik, basit ilişki                | `Joins`                         |
| Çok katmanlı ilişki, performans önemli değil   | `Preload` (kod basit)           |
| Çok büyük ilişkili tablo (1000+ kayıt)         | `Preload` + Limit / pagination  |
| Sadece bazı kolonlar lazım                     | `Preload` + `.Select(...)`      |

---

## 8. Kodumuzda Görünen Örnekler

### Manuel atama (auth.Register)

```go
// Eski:
db.Create(&user)
db.Preload("Role").First(&user, user.ID)  // 2 ekstra sorgu

// Yeni:
db.Create(&user)
user.Role = model.Role{ID: h.defaultRoleID, Title: h.cfg.DefaultRoleTitle}
// 0 ekstra sorgu
```

### Preload (book.List)

```go
tx.Preload("Author").Find(&books)
// Books listesi + Author satırları (IN clause ile gruplanmış)
```

Liste 20 satır olsa bile sorgu 2: `SELECT books` + `SELECT authors WHERE id IN (...)`.

### Nested Preload (library.Get)

```go
db.Preload("Books.Author").First(&lib, id)
-- 1. SELECT * FROM libraries WHERE id = ?
-- 2. SELECT * FROM library_books WHERE library_id = ?
-- 3. SELECT * FROM books WHERE id IN (...)
-- 4. SELECT * FROM authors WHERE id IN (...)
-- 4 sorgu, ama tüm graf doldu
```

`Books.Author` zinciri her seviye için bir IN sorgusu açar. Joins ile yapsaydık tek
sorgu olurdu ama M2M (library_books) join tablosu işin içine girdiği için sonuç kümesi
yelpazelenir, deserialization daha karmaşıktır. Bu yüzden M2M'de Preload genelde
daha temizdir.

---

## 9. Preload Tehlikeleri

### a) Çok Katmanlı Preload Çoklu Round-Trip

```go
db.Preload("Role.Permissions").Preload("Books.Author").Find(&users)
-- 1: users
-- 2: roles
-- 3: permissions
-- 4: books
-- 5: authors
-- 5 sorgu, hepsi ayrı round-trip
```

Çok katmanlı ilişkilerde büyür. Profile et, gerekirse Joins'e geç veya iki ayrı
sorguya böl.

### b) Büyük İlişkili Tablo

```go
db.Preload("Books").First(&user, 1)
// User'ın 10000 kitabı varsa hepsi belleğe yüklenir
// -> bellek patlar, response yavaşlar
```

Çözüm: pagination veya conditional preload:

```go
db.Preload("Books", func(db *gorm.DB) *gorm.DB {
    return db.Limit(10).Order("created_at DESC")
}).First(&user, 1)
```

### c) Gereksiz Veri Çekmek

Sadece role title lazımsa Preload tüm Role kolonlarını çeker. Çözüm:

```go
db.Preload("Role", func(db *gorm.DB) *gorm.DB {
    return db.Select("id", "title")
}).Find(&users)
```

---

## 10. Author Sorgusu — Niye `count(*)` Yerine `LIMIT 1`?

```go
// book.go authorExists:
err := db.Model(&model.Author{}).
    Select("id").Where("id = ?", id).
    Limit(1).Scan(&got).Error
```

`SELECT COUNT(*)` büyük tablolarda **tüm satırı tarar**. Bizim soru daha basit:
"bu id'de kayıt **var mı**?". Bir tane bulması yeterli, sayması değil. `LIMIT 1` ile
ilk eşleşmeyi bulup duruyor; planner'ın index üzerinde durdurmasını sağlıyor.

Bu, "soruna en küçük cevap" prensibi: gerekenden fazla iş yaptırma.

---

## Tek Cümle

`Preload` ilişkili veriyi **ayrı SQL sorgusuyla** çektiği için her ilişki bir
round-trip ekler; bu yüzden cevabı zaten bildiğin durumlarda manuel atama (0 sorgu),
basit ilişkilerde `Joins` (1 sorgu) tercih edilir — `Preload` sadece kod basit kalsın
istediğinde ve performans kritik olmadığında uygundur, ama N+1'den daima iyidir
(N+1: 101 sorgu, Preload: 2 sorgu).
