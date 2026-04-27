# libra_management

Go ile yazılmış küçük bir kütüphane yönetim (library management) backend'i.
**Eğitim amaçlı** bir proje — kod yorumlu, mimari bilinçli olarak basit tutulmuş.

> Stack: **Go 1.26.1** · **Fiber v3** · **GORM** · **MySQL** · **JWT (HS256)** · **bcrypt** · **go-playground/validator/v10**

---

## İçindekiler
- [Hızlı Başlangıç](#hızlı-başlangıç)
- [Dizin Yapısı](#dizin-yapısı)
- [Mimari Özet](#mimari-özet)
- [API Uçları](#api-uçları)
- [Faz Durumu](#faz-durumu)
- [Kapatılan Güvenlik / Mantık Açıkları](#kapatılan-güvenlik--mantık-açıkları)
- [Daha Fazla Bilgi](#daha-fazla-bilgi)

---

## Hızlı Başlangıç

### Ön koşullar
- Go 1.26+
- MySQL 8 (local'de çalışıyor olmalı; default DB adı `libra_db`)

### Kurulum
```bash
# 1) Repoyu klonla
git clone <repo-url> libra_management
cd libra_management

# 2) Konfigürasyon dosyasını oluştur
cp .env.example .env
# .env'i editöründe aç ve değerleri ortamına göre doldur:
#   - DB_DSN: MySQL bağlantı stringi
#   - JWT_SECRET: `openssl rand -base64 48` ile üret
#   - COOKIE_SECURE: dev'de false, production'da true
#   - ALLOWED_ORIGINS: CORS whitelist

# 3) MySQL'de DB'yi yarat
mysql -u root -p -e "CREATE DATABASE libra_db CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"

# 4) Bağımlılıklar
go mod download

# 5) Çalıştır (migration + seed otomatik)
go run ./cmd/server
```

İlk boot'ta:
- Tüm tablolar otomatik oluşturulur (User, Role, Permission, Author, Book, Library)
- `roles` tablosuna `admin` ve `user` rolleri seed edilir
- Server `cfg.Port` üzerinde dinlemeye başlar (default `:3000`)

---

## Dizin Yapısı

```
libra_management/
├── cmd/server/main.go                # Composition root — tüm bağımlılıklar burada bağlanır
├── internal/                         # Bu modül dışından import EDİLEMEZ (Go built-in kuralı)
│   ├── config/                       # Typed Config struct + .env yükleme
│   ├── database/                     # GORM Open + Migrate + Seed
│   ├── model/                        # GORM modelleri (User, Role, Permission, Author, Book, Library)
│   ├── dto/                          # Request/response struct'ları + mapper'lar
│   ├── handler/                      # HTTP handler'ları (DI'lı struct'lar)
│   ├── middleware/                   # Auth + RequireRole factory'leri
│   ├── router/                       # Endpoint kayıt
│   └── httpx/                        # Central error handling + response zarfı
├── pkg/                              # Projeden bağımsız, yeniden kullanılabilir paketler
│   ├── token/                        # JWT Manager (Generate/Parse, algorithm-confusion guard)
│   └── validate/                     # validator instance + hata format'ı
├── .env / .env.example / .gitignore
├── todo.md                           # Yol haritası (faz faz, sadece aktif faz detaylı)
├── CLAUDE.md                         # Claude için kod okuma rehberi (gelecek conversation'larda otomatik load)
└── README.md
```

---

## Mimari Özet

- **Composition root** (`cmd/server/main.go`): Bağımlılıklar **constructor injection** ile kurulur ve aşağı doğru zincirle dolaştırılır. Global state YOK.
- **`internal/`**: Go'nun import-restriction kuralı sayesinde proje dışı paketler buraya erişemez.
- **Konfigürasyon tek noktada**: Sadece `internal/config` `os.Getenv` çağırır; geri kalan paketler `*Config` parametre alır.
- **Hata yönetimi merkezi**: Handler'lar `return httpx.ErrXxx` der — central `ErrorHandler` JSON zarfını üretir. Inline `c.Status().JSON()` yok.
- **DTO sözleşmesi**: Model asla doğrudan JSON'a serialize edilmez; `dto.<E>Response` mapper sızıntıyı engeller.
- **Validator**: Tag-tabanlı (`required,email,min=8,eqfield=Password`); manual kontroller kalktı.

### Boot zinciri
```
config.Load()
  → database.Open(cfg) → Migrate(db) → Seed(db)
    → token.New(secret, ttl)
      → validate.New()
        → handler.NewAuthHandler(db, cfg, tokens, val)
          → fiber.New(ErrorHandler: httpx.Handler)
            → cors(cfg.AllowedOrigins, AllowCredentials: true)
              → router.Setup(app, Deps{...})
                → app.Listen(cfg.Port)
```

---

## API Uçları

> Tüm response'lar standart zarf içinde: başarı `{"data": ...}`, hata `{"error": {"code","message","details"}}`.

### Public

| Method | Path            | Body                                                         | Açıklama                               |
|--------|-----------------|--------------------------------------------------------------|----------------------------------------|
| POST   | `/api/register` | `{first_name, last_name, email, password, password_confirm}` | Yeni kullanıcı (default role = `user`) |
| POST   | `/api/login`    | `{email, password}`                                          | JWT cookie set eder                    |
| POST   | `/api/logout`   | —                                                            | JWT cookie'yi temizler                 |

### Protected (`Auth` middleware arkası)
Şu an boş — Faz 2+ ile dolacak (Author/Book/Library CRUD).

---

## Faz Durumu

| Faz   | Konu                                                             | Durum        |
|-------|------------------------------------------------------------------|--------------|
| **1** | Mimari stabilizasyon (layout + DI + DTO + httpx + auth refactor) | ✅ tamamlandı |
| **2** | Author CRUD (ilk uçtan-uca; pattern şablonu)                     | 🔵 aktif     |
| 3     | Book CRUD + Author ilişkisi                                      | ⏳            |
| 4     | Library CRUD + many-to-many Book ilişkisi                        | ⏳            |
| 5     | Permission tabanlı yetkilendirme                                 | ⏳            |
| 6     | Sayfalama / filtreleme / sıralama (generics fırsatı)             | ⏳            |
| 7     | Test stratejisi (testify + httptest)                             | ⏳            |
| 8     | Logging / observability (slog)                                   | ⏳            |

Detaylı faz adımları: **`todo.md`**.

---

## Kapatılan Güvenlik / Mantık Açıkları

Faz 1'de **13 sorun** bilinçli olarak kapatıldı; regression yapmamak için liste:

| #  | Sorun                                           | Çözüm                                      |
|----|-------------------------------------------------|--------------------------------------------|
| 1  | DB DSN kaynak kodda gömülü                      | `cfg.DBDSn` (env'den)                      |
| 2  | `.env` repoda + JWT_SECRET sızmış               | `.gitignore` + secret rotate               |
| 3  | CORS wildcard `*`                               | `cfg.AllowedOrigins` whitelist             |
| 4  | JWT algorithm-confusion attack                  | `SigningMethodHMAC` type assertion         |
| 5  | Login `404 user not found` → user enumeration   | Tek tip `ErrInvalidCredentials`            |
| 6  | Login timing-attack ile enumeration             | `dummyHash` ile sahte bcrypt               |
| 7  | `RoleID: 2` hardcoded → privilege escalation    | `cfg.DefaultRoleTitle` üzerinden DB lookup |
| 8  | Silinen user'ın token'ı hâlâ geçerli            | Auth middleware'inde DB kontrol            |
| 9  | Token'daki RoleID rol değişiminden sonra kalıcı | Her istekte DB'den taze çekiliyor          |
| 10 | Cookie eksik flag'leri                          | HTTPOnly + Secure(cfg) + SameSite=Lax      |
| 11 | Passwords mismatch → 401                        | `eqfield=Password` validator → 400         |
| 12 | DB ham hata mesajı client'a sızıyor             | Generic `ErrInternal` + log'da iç hata     |
| 13 | `model.User` doğrudan JSON                      | `dto.UserResponse` mapper                  |

---

## Daha Fazla Bilgi

- **Yol haritası ve yapılacaklar**: [`todo.md`](./todo.md)
- **Kod okuma rehberi (Claude veya yeni geliştirici için)**: [`CLAUDE.md`](./CLAUDE.md) — okuma sırası, request lifecycle, paket sözlüğü, CRUD şablonu
- Yorumlar Türkçe ve eğitim amaçlı (proje öğrenme için yazılıyor)
