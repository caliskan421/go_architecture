# libra_management — Yol Haritası

---

## Faz 1 — Mimari Stabilizasyon (AKTİF)

> Hedef: Yeni özellik (Author/Book/Library CRUD) yazılmadan önce dizin yapısı, konfigürasyon, hata yönetimi, validasyon ve auth iskeletinin tek standartta toplanması. Mevcut auth akışı yeni desene taşınırken **tespit edilen mantık hataları ve güvenlik açıkları** da kapatılır.

### 1.1 — Modül adı + dizin iskeleti ✅
- [x] `go.mod` modül adını `first` → `libra_management` yap
- [x] `cmd/server/`, `internal/{database,model,handler,middleware,router}/`, `pkg/token/` klasörlerini aç (config/dto/httpx/validate sonraki adımlarda eklenecek)
- [x] Eski dosyaları yeni konumlara taşı (sadece taşıma + import path düzeltmesi, içerik refactor'u sonraki adımlarda)
- [x] `go build ./...` derlenir hâlde — DOĞRULANDI

### 1.2 — Repo hijyeni ✅
- [x] `.gitignore` ekle (`.env*`, build artifacts, IDE, macOS, air `tmp/`)
- [x] `.env.example` ekle (şablon, gerçek secret yok)
- [x] `JWT_SECRET` rotate edildi (`openssl rand -base64 48`)
- [x] `.env` Faz 1.3'te `config.Load()` tarafından okunacak ek değişkenlerle dolduruldu (`DB_DSN`, `JWT_TTL_HOURS`, `PORT`, `ALLOWED_ORIGINS`, `DEFAULT_ROLE_TITLE`)
- [~] `git rm --cached .env` — proje henüz git repo değil; `git init` yapıldığında `.gitignore` zaten devrede olacak

### 1.3 — `internal/config` ✅
- [x] Typed `Config` struct: `DBDSn`, `JWTSecret`, `JWTTTL`, `Port`, `AllowedOrigins`, `DefaultRoleTitle`
- [x] `Load()` fonksiyonu — godotenv + tip dönüşümleri + zorunlu alan kontrolü
- [x] Eksik kritik env'de `log.Fatalf` (panic yerine); hatalı sayı formatında da fail-fast

### 1.4 — `internal/database` ✅ (geçici köprü ile)
- [x] `Open(cfg) (*gorm.DB, error)` — DSN artık cfg'den, panic yerine error döner
- [x] `Migrate(db)` — TÜM modeller (User, Role, Permission, Author, Book, Library)
- [x] `Seed(db)` — `admin` ve `user` rolleri `FirstOrCreate` ile idempotent
- [~] **`var DB *gorm.DB` global'i GEÇİCİ olarak duruyor** — handler/middleware 1.9'da refactor edilince main.go'daki `database.DB = db` köprüsü ile birlikte SİLİNECEK
- [x] main.go composition root'a çevrildi: `config.Load → Open → Migrate → Seed → bridge → fiber`

### 1.5 — `pkg/token` ✅ (köprü ile)
- [x] `Manager` struct + `New(secret, ttl)` ctor — secret/TTL artık DI ile
- [x] `Generate(userID, roleID)` ve `Parse(tokenString)` pointer-receiver metotları
- [x] **Güvenlik fix**: signing-method type-assertion ile algorithm-confusion attack kapatıldı
- [~] Eski `GenerateToken`/`ParseToken` package-level fonksiyonları DEPRECATED yorumlu olarak duruyor (handler/middleware bağımlılıkları için); 1.9'da silinecek
- [x] **Köprü güvenliği**: deprecated fonksiyonlara da signing-method check uygulandı — açık 1.9'a kadar açık beklemiyor

### 1.6 — `pkg/validate` + `internal/dto` ✅
- [x] `validate.New()` — `*validator.Validate` singleton ctor
- [x] `validate.Format(err)` — `validator.ValidationErrors` → kullanıcı-dostu `[]FieldError` çevrimi (TR mesajlar)
- [x] `dto.RegisterRequest` — `required`, `email`, `min=8`, `eqfield=Password` tag'leri
- [x] `dto.LoginRequest`
- [x] `dto.UserResponse` + `ToUserResponse(u)` mapper — password ve gorm.Model sızıntısını engeller

### 1.7 — `internal/httpx` ✅
- [x] `AppError` struct + `Error()`/`Unwrap()`/`Is()` (errors.Is Code üzerinden çalışır)
- [x] `WithDetails`/`WithErr` immutable extension (sentinel mutate edilmez)
- [x] Sentinel'lar: `ErrInvalidCredentials`, `ErrValidation`, `ErrBadRequest`, `ErrUnauthorized`, `ErrForbidden`, `ErrNotFound`, `ErrInternal`
- [x] `Success(c, status, data)` ve `Error(c, err)` — `{"data": ...}` / `{"error": {...}}` zarflama
- [x] `Handler` — Fiber `ErrorHandler` kancası: 5xx log + central JSON dönüş

### 1.8 — `internal/middleware` ✅
- [x] `Auth(db, tokenMgr) fiber.Handler` factory — closure ile DI
- [x] **Güvenlik fix**: DB user lookup eklendi (silinmiş user fix)
- [x] **Güvenlik fix**: RoleID token'dan değil DB'den taze çekiliyor (rol değişiklikleri anında geçerli)
- [x] `RequireRole(roleIDs ...uint) fiber.Handler` factory — variadic
- [x] Router'daki `_ = api` ölü kodu temizlendi (1.10'a iter etmeden burada yapıldı çünkü eski `Auth` silinince zaten gerekiyordu)

### 1.9 — `internal/handler/auth.go` ✅
- [x] `AuthHandler` struct + `NewAuthHandler` ctor (DB, config, token, validator inject)
- [x] `Register` — DTO + validator, RoleID `h.defaultRoleID` (privilege-escalation fix), response DTO, gereksiz Preload kaldırıldı
- [x] `Login` — generic `ErrInvalidCredentials` (user enumeration fix), `gorm.ErrRecordNotFound` ayrı dal, **dummy bcrypt** ile timing-attack guard
- [x] `Logout` — cookie temizleme yeni response zarfı ile
- [x] **Mantık fix**: passwords mismatch artık validator işi (`eqfield=Password`) → 400
- [x] Cookie: HTTPOnly + Secure (cfg.CookieSecure) + SameSite=Lax
- [x] `cfg.CookieSecure` alanı + `.env`/`.env.example` eklendi
- [x] Global `database.DB` köprüsü SİLİNDİ
- [x] Deprecated `token.GenerateToken/ParseToken` SİLİNDİ

### 1.10 — `internal/router/router.go` ✅
- [x] `Setup(app, deps Deps)` imzası — `Deps{Auth, DB, TokenMgr}` ile DI
- [x] Public group (`/api`) — register/login/logout
- [x] Protected group (`/api` + `middleware.Auth(deps.DB, deps.TokenMgr)`) — şu an boş, Faz 2+'da dolacak
- [x] `_ = api` ölü kodu temizlendi (1.8'de yapıldı)

### 1.11 — `cmd/server/main.go` ✅
- [x] Sıralı bağımlılık kurulumu: config → db → migrate → seed → token → validator → handler → fiber+errorhandler → cors → router → listen
- [x] CORS artık `cfg.AllowedOrigins` whitelist + `AllowCredentials: true` (cookie auth için)
- [x] Fiber `ErrorHandler: httpx.Handler` bağlandı
- [~] `slog` logger boot — şimdilik fiber'in `log.Fatalf`'ı yeterli; structured logging Faz 8'e ertelendi (overengineering kaçınma)

---

## Sonraki Fazlar (sıra geldikçe açılacak)
- **Faz 2** — `Author` CRUD (yeni desende ilk uçtan-uca uygulama) [X]
- **Faz 3** — `Book` CRUD + Author ilişkisi (Preload, FK validation)
- **Faz 4** — `Library` CRUD + many-to-many Book ilişkisi (Association API, transaction)
- **Faz 5** — Permission tabanlı yetkilendirme (Role → Permissions → endpoint koruma)
- **Faz 6** — Sayfalama / filtreleme / sıralama (generics fırsatı: `httpx.Page[T]`)
- **Faz 7** — Test stratejisi (testify + httptest + in-memory SQLite)
- **Faz 8** — Logging / observability (slog, request-id middleware)
