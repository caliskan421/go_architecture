# Cache Pattern: Pahalı Hesaplamayı Tekrarlatma

Cache, **pahalı bir işlemin sonucunu saklayıp tekrar tekrar kullanmak** demek. Bu
projede iki yerde uygulanır: (1) `AuthHandler` boot-time cache, (2) `Authorizer`
runtime cache. Bu dosya: cache nedir, ne zaman cache, hangi hangi senaryoya uyar,
geçersizleştirme nasıl yapılır.

---

## 1. Cache'in Üç Koşulu

Bir veriyi cache'lemek mantıklıysa **üçü birden** sağlanmalı:

1. **Hesaplaması pahalı** (DB sorgusu, hash üretimi, network çağrısı, dosya okuma).
2. **Sık kullanılıyor** (her istekte tekrar lazım).
3. **Çok sık değişmiyor** (taze olmasına gerek yok).

Biri eksikse cache fayda sağlamaz, hatta zarar verir:

| Eksik koşul    | Olan kötü şey                                       |
|----------------|-----------------------------------------------------|
| Pahalı değil   | Sıfırdan hesaplama zaten ucuz; cache extra karmaşıklık |
| Sık değil       | Cache yer tutar, çoğu zaman okunmaz                  |
| Sık değişiyor  | Bayat veri okuyup yanlış sonuçlar üretirsin          |

---

## 2. Cache'in İki Tipi

| Tip                  | Hesaplama anı           | Saklama yeri            |
|----------------------|-------------------------|--------------------------|
| **Boot-time cache**  | Uygulama başlarken      | Struct alanı (handler vs.) |
| **Runtime cache**    | İlk istekte (lazy)      | RAM (map, sync.Map), Redis, memcached |

Üçüncü tip de var: **HTTP response cache** (CDN, reverse proxy) — backend dışı,
bu kapsam dışında.

---

## 3. Boot-Time Cache: `AuthHandler`

```go
type AuthHandler struct {
    db            *gorm.DB
    cfg           *config.Config
    tokens        *token.Manager
    val           *validator.Validate
    defaultRoleID uint   // cache'lenen veri
    dummyHash     []byte // cache'lenen veri
}

func NewAuthHandler(db *gorm.DB, cfg *config.Config, tokens *token.Manager, val *validator.Validate) (*AuthHandler, error) {
    // 1. DB'den default role'ün ID'sini bir kez çek
    var role model.Role
    if err := db.Where("title = ?", cfg.DefaultRoleTitle).First(&role).Error; err != nil {
        return nil, err
    }

    // 2. Bcrypt dummy hash'ini bir kez üret (~250ms maliyetli)
    dummy, err := bcrypt.GenerateFromPassword([]byte("dummy"), bcryptCost)
    if err != nil {
        return nil, err
    }

    return &AuthHandler{
        db:            db,
        cfg:           cfg,
        tokens:        tokens,
        val:           val,
        defaultRoleID: role.ID,
        dummyHash:     dummy,
    }, nil
}
```

İki şey cache'leniyor:

### a) `defaultRoleID`

- **Pahalı?** Evet — DB sorgusu (network round-trip).
- **Sık?** Evet — her register isteğinde lazım.
- **Değişmez mi?** Evet — production'da `roles` tablosu seed dışında değişmez.

Üçü de tam tutuyor. Boot'ta bir kez çek, struct'a yaz, hayat boyu kullan.

### b) `dummyHash`

- **Pahalı?** Evet — bcrypt cost=14 ~250ms CPU.
- **Sık?** Evet — login'de "user yok" yolunda kullanılıyor.
- **Değişmez mi?** Evet — sahte bir hash, içeriği önemsiz.

Niye lazım: timing-attack guard. Detay için `security.md`.

---

## 4. Cache OLMASA Ne Olurdu?

```go
func (h *AuthHandler) Register(c fiber.Ctx) error {
    // Her register'da DB sorgusu (cache yok)
    var role model.Role
    h.db.Where("title = ?", "user").First(&role)
    // user oluştur, role.ID'yi kullan...
}
```

Saniyede 1000 register -> saniyede 1000 ekstra DB sorgusu. Cevap **hep aynı** ("user"
rolünün ID'si değişmiyor) ama her seferinde sorulur. Boşa connection pool tüketimi,
boşa CPU.

---

## 5. Runtime Cache: `Authorizer` (RWMutex + Double-Check)

`internal/middleware/authorizer.go`'da daha sofistike bir cache var. RBAC izinlerini
DB'den çekip RAM'de tutuyor.

```go
type Authorizer struct {
    db    *gorm.DB
    mu    sync.RWMutex
    cache map[uint]map[string]struct{}  // roleID -> izin seti
}
```

### Niye `sync.RWMutex`?

Cache'e:

- **Çok sık okuma** olur (her istekte `RequirePermission`).
- **Nadiren yazma** olur (sadece ilk cache miss'te).

`sync.Mutex` yazma/okuma ayrımı yapmaz; "tek sıra". `sync.RWMutex` ise:

- `RLock` -> birden fazla goroutine **aynı anda** okuyabilir.
- `Lock` -> tek başına yazıyor, herkes bekliyor.

Bizim profilimize uygun: yüksek-okuma, düşük-yazma.

### Double-Check Pattern

```go
func (a *Authorizer) permsFor(roleID uint) (map[string]struct{}, error) {
    // 1. Hızlı yol: read-lock ile bak
    a.mu.RLock()
    if set, ok := a.cache[roleID]; ok {
        a.mu.RUnlock()
        return set, nil
    }
    a.mu.RUnlock()

    // 2. Yavaş yol: write-lock al, ama tekrar kontrol et
    a.mu.Lock()
    defer a.mu.Unlock()

    // İki goroutine aynı anda 1. yolda miss yiyip 2. yola geçmiş olabilir.
    // Birinci bu lock'u alıp doldurur; ikinci lock'u beklerken cache zaten dolmuştur.
    // Tekrar kontrol et:
    if set, ok := a.cache[roleID]; ok {
        return set, nil
    }

    set, err := a.loadFromDB(roleID)
    if err != nil {
        return nil, err
    }
    a.cache[roleID] = set
    return set, nil
}
```

Pattern'in adı **double-checked locking**. İlk check optimizasyon: çoğu zaman cache
dolu, lock upgrade etmeden dön. İkinci check güvenlik: lock alana kadar başkası
doldurmuş olabilir, iki kere DB'ye gitme.

### Niye Boot'ta Yüklenmedi de Lazy?

Boot'ta da yüklenebilirdi:

```go
// her seed'de:
authorizer.WarmCache(adminRoleID)
authorizer.WarmCache(userRoleID)
```

Lazy seçildi çünkü:

- Yeni bir rol eklenirse boot kodu güncellenmeden çalışır.
- Hiç istek almayan bir rolün izinleri belleğe gereksiz yüklenmez.

---

## 6. Cache Invalidation (Geçersizleştirme)

> "There are only two hard things in Computer Science: cache invalidation and naming
> things." — Phil Karlton

Cache'lediğin veri değişirse cache'i temizlemen gerekir.

| Cache tipi   | Invalidation yolu                                  |
|--------------|----------------------------------------------------|
| Boot-time    | Uygulamayı restart et                              |
| Runtime      | TTL (otomatik), event-driven (`Invalidate(...)`), tam temizlik (`Clear()`) |

`Authorizer`'da iki metot var:

```go
func (a *Authorizer) Invalidate(roleID uint)  // tek bir rol
func (a *Authorizer) Clear()                  // tüm cache
```

Şu an çağıran yok (seed-only). İlerde admin paneli ile rol-izin ilişkisi runtime'da
değişirse buradan tetiklenir.

### Bayat Veri Riski

Cache invalidation kaçırırsan saldırgan eski izinlerle iş yapabilir. Örnek senaryo:

1. Admin "user" rolünden "book:write" iznini kaldırdı (DB güncel).
2. `Authorizer.cache[userRoleID]` hala eski seti tutuyor.
3. Bir user `POST /api/books` atıyor; cache'e bakılıyor, izin var sanılıyor.
4. Yetkisiz yazma gerçekleşiyor.

Bu yüzden invalidation yolunu açık tutmak ZORUNLU. TTL koymak ek savunma:

```go
type cached struct {
    set       map[string]struct{}
    expiresAt time.Time
}
```

---

## 7. Karar Tablosu

| Veri                            | Cache yeri                  | Neden                            |
|---------------------------------|-----------------------------|----------------------------------|
| Default role ID                 | Boot-time (struct)          | Hiç değişmez, sık lazım          |
| JWT secret                      | Boot-time (struct)          | Boot'ta config'den gelir         |
| Bcrypt dummy hash               | Boot-time (struct)          | Pahalı (~250ms), sık, sabit      |
| Role -> izin seti               | Runtime (RWMutex + map)     | Pahalı join, sık, nadiren değişir|
| User listesi                    | Cache YOK                   | Sürekli değişir                  |
| Aktif kullanıcı sayısı (örn.)   | Runtime (Redis, 1dk TTL)    | Pahalı sorgu, biraz eski OK      |
| HTTP response (GET /books)      | Runtime (Redis, CDN)        | Aynı sorgu sık tekrarlanır       |

---

## 8. Cache'in Olumsuz Yan Etkileri

Cache "tek yönlü iyi" değildir:

- **Bellek kullanımı artar** — büyük veri RAM'i şişirir.
- **Concurrency karmaşıklığı** — lock yanlış kurulursa race condition.
- **Hata ayıklama zorlaşır** — "DB değiştirdim ama uygulama eski değeri görüyor"
  kafanı yorar.
- **Cold start yavaşlar** — boot'ta tüm cache'i doldurursan açılış uzar.

Bu yüzden cache "varsayılan" değil, "ölçtükten sonra ekleyeceğin" bir araç olmalı.

---

## Tek Cümle

Cache, **pahalı + sık + nadiren değişen** verilerin sonucunu saklayıp tekrar
hesaplamayı önler; AuthHandler'daki `defaultRoleID` ve `dummyHash` boot-time uygulaması
(struct'a yaz, ömür boyu kullan), Authorizer'daki rol-izin map'i ise RWMutex korumalı
runtime uygulaması (lazy doldur, double-check ile race önle) — invalidation yolunu
**önceden** kur, çünkü "cache invalidation" bilgisayar bilimi tarihinin en zor iki
sorunundan biri.
