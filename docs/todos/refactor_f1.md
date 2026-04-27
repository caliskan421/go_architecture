# libra_management — Yol Haritası

---

## Faz 1 — Mimari Stabilizasyon ✅

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

### 1.4 — `internal/database` ✅
- [x] `Open(cfg) (*gorm.DB, error)` — DSN artık cfg'den, panic yerine error döner
- [x] `Migrate(db)` — TÜM modeller (User, Role, Permission, Author, Book, Library)
- [x] `Seed(db)` — Faz 5'te genişletildi: roller + permissions + role-permission senkronizasyonu

### 1.5 — `pkg/token` ✅
- [x] `Manager` struct + `New(secret, ttl)` ctor
- [x] `Generate(userID, roleID)` ve `Parse(tokenString)` pointer-receiver metotları
- [x] **Güvenlik fix**: signing-method type-assertion ile algorithm-confusion attack kapatıldı

### 1.6 — `pkg/validate` + `internal/dto` ✅
- [x] `validate.New()` + `validate.Format(err)` (TR mesajlar)
- [x] `dto.RegisterRequest`, `dto.LoginRequest`, `dto.UserResponse` + mapper

### 1.7 — `internal/httpx` ✅
- [x] `AppError` + sentinel'lar + `WithDetails`/`WithErr` immutable extensions
- [x] `Success(c, status, data)` ve `Error(c, err)` zarflama
- [x] `Handler` central error handler

### 1.8 — `internal/middleware` ✅
- [x] `Auth(db, tokenMgr) fiber.Handler` factory
- [x] **Güvenlik fix**: silinmiş user kontrolü + RoleID DB'den taze çekme

### 1.9 — `internal/handler/auth.go` ✅
- [x] DI'lı `AuthHandler` + Register/Login/Logout
- [x] User enumeration + timing-attack guard (dummy bcrypt)
- [x] Privilege-escalation fix (RoleID config-driven)
- [x] Cookie HTTPOnly + Secure + SameSite=Lax

### 1.10 — `internal/router/router.go` ✅
- [x] `Setup(app, deps Deps)` — DI ile rotalar

### 1.11 — `cmd/server/main.go` ✅
- [x] Composition root: config → db → migrate → seed → token → val → handler → fiber → cors → router → listen

---

## Faz 2 — `Author` CRUD ✅

> Yeni desende ilk uçtan-uca uygulama. CRUD pattern'inin referans şablonu.

- [x] `model.Author` — gorm.Model BİLİNÇLİ kullanılmadı (soft-delete + uniqueIndex çelişkisi)
- [x] `dto.CreateAuthorRequest`, `dto.UpdateAuthorRequest`, `dto.AuthorResponse` + mapper
- [x] `handler.AuthorHandler` — List/Get/Create/Update/Delete (DI'lı)
- [x] `parseID()` ortak yardımcı (handler paketi içinde)
- [x] `gorm.Config.TranslateError: true` → `gorm.ErrDuplicatedKey` → 409 Conflict
- [x] Build doğrulandı

---

## Faz 3 — `Book` CRUD + Author ilişkisi ✅

> Foreign-key validation + Preload pattern.

- [x] `model.Book` — `AuthorID` FK + `Author` Preload alanı (gorm.Model yerine manuel timestamps + uniqueIndex)
- [x] `dto.CreateBookRequest` / `UpdateBookRequest` — `author_id required,gt=0`; `BookResponse` nested `AuthorResponse` ile
- [x] `handler.BookHandler` — List/Get/Create/Update/Delete
- [x] `authorExists(id)` helper — FK varlık kontrolü, validator detail formatında "AuthorID" alan hatası döner
- [x] List endpoint Preload("Author") ile tek round-trip
- [x] Update'te AuthorID değişiyorsa yeni id'nin varlığı doğrulanır
- [x] Build doğrulandı

---

## Faz 4 — `Library` CRUD + many-to-many `Book` ilişkisi ✅

> Association API + transaction kullanımı.

- [x] `model.Library` — `Books []Book gorm:"many2many:library_books"`
- [x] `dto.CreateLibraryRequest` (BookIDs opsiyonel), `UpdateLibraryRequest` (BookIDs `*[]uint` — nil = dokunma, [] = boşalt, [...] = replace)
- [x] `dto.LibraryBookIDsRequest` — alt-route'lar için ortak body (Add/Remove)
- [x] `handler.LibraryHandler` — CRUD + `AddBooks` + `RemoveBooks`
- [x] `fetchBooksByIDs(ids)` — tek SQL `WHERE id IN (...)`, eksikleri tespit eder
- [x] `dedupeUints` — duplicate id'leri filtreler
- [x] `db.Transaction` ile parent + Association atomic
- [x] `Association("Books").Replace/Append/Delete/Clear` API kullanımı
- [x] AddBooks zaten bağlı id'leri sessizce yok sayar (idempotent)
- [x] Delete: önce `Association.Clear()` sonra parent delete (orphan join row engellendi)
- [x] Build doğrulandı

---

## Faz 5 — Permission tabanlı yetkilendirme ✅

> RBAC: Role → Permissions → endpoint koruma. RequireRole'ün yerini RequirePermission aldı.

- [x] `internal/auth/permissions.go` — `Perm*` sabitleri, `AllPermissions`, `UserPermissions` listeleri
- [x] `database.Seed` genişletildi:
  - `seedPermissions` — `auth.AllPermissions` listesini DB'ye yaz
  - `seedRoles` — admin/user
  - `syncRolePermissions` — kod listesi DB ile **eşitlenir** (Replace semantiği; tek doğruluk noktası kod)
- [x] `middleware.Authorizer` — role→permission set'lerini cache'leyen servis (RWMutex + double-check pattern)
- [x] `Authorizer.RequirePermission(perm) fiber.Handler` — locals'tan roleID okur, cache'den izin set'ini çeker, kontrol eder
- [x] `Authorizer.Invalidate(roleID)` / `Clear()` — runtime değişiklikler için
- [x] router.go: her endpoint kendi `auth.Perm*` izniyle korunur
- [x] main.go'ya Authorizer enjekte edildi
- [x] Build doğrulandı

---

## Faz 6 — Sayfalama / filtreleme / sıralama ✅

> Generics fırsatı: `httpx.Page[T]`. Whitelist'li sort + temel search.

- [x] `httpx.Pagination` — Page/PageSize + Limit/Offset; `ParsePagination(c)` query parser
  - default page_size 20, max 100 (DoS guard)
- [x] `httpx.Sort` — Field/Asc + `OrderClause()`; `ParseSort(c, allowed, defaultField)` whitelist'li (SQL injection guard)
- [x] `httpx.Page[T any]` generic response zarfı — Items/Total/Page/PageSize/TotalPages
- [x] `NewPage(items, total, p)` — nil items'ı `[]` yapar (JSON `null` çıkmasın)
- [x] Author List: `?q=` (name LIKE), `?sort=`, `?order=`, `?page=`, `?page_size=`
- [x] Book List: aynısı + `?author_id=` filtresi
- [x] Library List: aynısı (Books + Books.Author Preload korunur)
- [x] Build doğrulandı

---

## Faz 7 — Test stratejisi ✅

> testify + httptest + in-memory SQLite. Black-box integration testleri.

- [x] `gorm.io/driver/sqlite` + `github.com/stretchr/testify` dependency eklendi
- [x] `internal/testutil/db.go` — `NewTestDB(t)` in-memory SQLite + Migrate + Seed
- [x] `internal/testutil/app.go`:
  - `NewTestApp(t)` — üretimle birebir aynı kompozisyonda Fiber app
  - `SeedAdminUser/SeedUserUser` — direkt DB'ye user yaz
  - `LoginAdmin/LoginUser` — JWT cookie döndürür
  - `Get/Post/Put/Delete` — JSON-aware request helper'ları
  - `DecodeData/DecodeError` — `{"data":...}`/`{"error":...}` zarflarını açar
- [x] `handler/auth_test.go` — register/login/logout + user enumeration koruması
- [x] `handler/author_test.go` — CRUD happy-path + 409 conflict + RBAC (user yazamaz) + sayfalama/filtre/sıralama
- [x] `handler/book_test.go` — CRUD + FK validation (404 yerine validation_error) + `?author_id=` filtre
- [x] `handler/library_test.go` — CRUD + M2M Add/Remove + Update Replace + paginated list
- [x] `go test ./...` — tüm testler **PASS**

---

## Faz 8 — Logging / observability ✅

> slog (Go 1.21+) + request-id middleware.

- [x] `middleware/requestid.go` — `RequestID()` middleware:
  - Incoming `X-Request-ID` header'ı kabul eder (distributed tracing)
  - Yoksa UUID v4 üretir
  - `c.Locals("request_id", id)` + response header
- [x] `middleware.GetRequestID(c)` getter
- [x] `httpx/errorhandler.go` slog'a geçti:
  - 5xx → `slog.Error` (request_id, code, path, method, error)
  - 4xx → `slog.Info` (operatörü ilgilendirmez ama audit için)
  - bilinmeyen tip → generic 500 + log
- [x] `cmd/server/main.go`:
  - `slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, ...)))` — production'da JSON
  - `app.Use(middleware.RequestID())` Auth/CORS'tan önce
  - `slog.Info("server starting", "port", cfg.Port)`
- [x] `testutil` slog'u `io.Discard`'a yönlendirir (test çıktısı temiz)
- [x] `go build ./...` + `go test ./...` — temiz

---

## Roadmap kapanışı

**Mevcut durum**: Faz 1–8 tamamlandı; uygulama uçtan uca üretime hazır iskelete sahip.
- 3 entity (Author/Book/Library) full CRUD + ilişkiler
- RBAC izin sistemi seed-driven
- Sayfalama/filtre/sıralama generic Page[T] ile
- Integration test suite (in-memory SQLite)
- Structured logging + request-id

**İleride opsiyonel**:
- Refresh token / token rotation
- Rate limiting middleware
- OpenAPI dokümantasyonu (swag)
- Soft-delete + audit log (Faz 4 ertelenen karar)
- Prometheus metric'leri / `/metrics` endpoint
