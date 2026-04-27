# libra_management — Kod Okuma Rehberi (Claude için)

> Bu dosya yeni bir Claude conversation'ı için **kodu hızlıca anlama** rehberidir. Sıra ve gerekçe ile okunmalı; rastgele dosya açma yerine bu sırayı takip et.

## TL;DR — proje nedir
Go 1.26.1 + Fiber v3 + GORM + MySQL ile yazılmış küçük bir kütüphane yönetimi (library management) backend'i. Auth (register/login/logout) + RBAC iskeleti yerinde; Author/Book/Library CRUD'ları sıraya alınmış. Mimari **Faz 1**'de stabilize edildi (clean layout + DI + DTO/validator/httpx katmanları).

Yol haritası: `todo.md` (faz faz, sadece aktif faz detaylı). Detaylı orijinal plan: `~/.claude/plans/declarative-napping-pretzel.md`.

---

## Okuma Sırası (önerilen — yukarıdan aşağıya)

### A. Önce sözleşmeler — "ne anlamına geliyor?"
Bu dosyalar yorum yoğunlukludur ve diğer paketlerin neden böyle yazıldığını anlamanın anahtarıdır.

1. **`todo.md`** — Hangi fazda olduğumuzu, kapatılan açıkları, geçici köprüleri görür.
2. **`go.mod`** — Modül adı `libra_management`, Go 1.26.1, kullanılan kütüphaneler.
3. **`.env.example`** — Hangi konfigürasyon değişkenleri var, hangileri zorunlu.

### B. Composition root — "uygulama nasıl başlıyor?"
Boot zincirini takip ederek hangi paketin neye bağımlı olduğunu öğrenirsin. **Yukarıdan aşağıya tek dosya** oku, her satırda hangi pakete dallandığını not al.

4. **`cmd/server/main.go`** — Bağımlılık kurulum sırası:
   ```
   config.Load()
     → database.Open(cfg) → Migrate(db) → Seed(db)
       → token.New(secret, ttl)
         → validate.New()
           → handler.NewAuthHandler(db, cfg, tokens, val)
             → fiber.New(ErrorHandler: httpx.Handler) + cors(cfg.AllowedOrigins)
               → router.Setup(app, Deps{Auth, DB, TokenMgr})
                 → app.Listen(cfg.Port)
   ```

### C. Çekirdek altyapı paketleri — "ortak alet kutusu"
Bu paketler diğer her paket tarafından kullanılır; önce onları okumadan handler/router okumak boşa zaman.

5. **`internal/config/config.go`** — `Config` struct + `Load()` + `mustGet/getStr/getInt/splitCSV`. Konfigürasyon **tek doğruluk noktası**; hiçbir başka paket `os.Getenv` çağırmaz.
6. **`pkg/token/jwt.go`** — `Manager` struct + `New/Generate/Parse`. Algorithm-confusion guard `Parse` içinde (`SigningMethodHMAC` type assertion).
7. **`pkg/validate/validate.go`** — `New()` + `Format(err) []FieldError` (Türkçe mesajlar).
8. **`internal/httpx/`** — Üç dosya, **aynı paket** (`httpx`):
   - `apperror.go` → `AppError` + sentinel'lar (`ErrInvalidCredentials`, `ErrValidation`, `ErrBadRequest`, `ErrUnauthorized`, `ErrForbidden`, `ErrNotFound`, `ErrInternal`) + `WithDetails`/`WithErr` immutable.
   - `response.go` → `Success(c, status, data)` ve `Error(c, err)` — `{"data":...}` / `{"error":{...}}` zarflama.
   - `errorhandler.go` → Fiber'in central `Handler`; 5xx log + `Error()`'a delegasyon.

### D. Veri katmanı — "nasıl sakladığımız"
9. **`internal/database/database.go`** — `Open/Migrate/Seed` üçlüsü; global `DB` **YOK** (Faz 1.9'da silindi).
10. **`internal/model/`** — GORM struct'ları. Sırayla oku: `permission` → `role` → `user` → `author` → `book` → `library`. İlişkiler: User→Role (FK), Role↔Permission (M2M), Book→Author (FK), Library↔Book (M2M).

### E. Sözleşme katmanı — "dış dünya ne görüyor"
11. **`internal/dto/auth.go`** — `RegisterRequest`/`LoginRequest`/`UserResponse` + `ToUserResponse(u)` mapper. **Model asla doğrudan JSON'a dönmez** — DTO mapper sızıntıyı engeller.

### F. HTTP katmanı — "request → response"
12. **`internal/middleware/auth.go`** — `Auth(db, tokenMgr)` factory + `RequireRole(roleIDs...)` factory. Her ikisi de Fiber `Handler` döner (closure). Auth: cookie parse → tokenMgr.Parse → DB user lookup → `c.Locals(userID, roleID)` → `c.Next()`.
13. **`internal/handler/auth.go`** — `AuthHandler` struct + `NewAuthHandler` ctor + `Register/Login/Logout`. Bu dosya **CRUD pattern'inin referans şablonudur** — yeni handler'lar bunu kopyalar.
14. **`internal/router/router.go`** — `Setup(app, Deps)` + public/protected groups.

---

## Request Lifecycle — bir POST /api/login örneği

```
HTTP request "POST /api/login + body"
    ↓
Fiber app (CORS middleware)
    ↓
Router → public group → handler.AuthHandler.Login
    ↓
[Login içinde]
  c.Bind().Body(&req)             ← dto.LoginRequest
  h.val.Struct(&req)              ← validator → ErrValidation+details ise return
  h.db.Preload("Role").Where(...) ← user.ID==0 mı?
    → ErrRecordNotFound: dummy bcrypt + ErrInvalidCredentials  ← timing-attack guard
  bcrypt.CompareHashAndPassword   ← yanlış: ErrInvalidCredentials
  h.tokens.Generate               ← JWT
  c.Cookie(jwt, HTTPOnly+Secure+SameSite=Lax)
  return httpx.Success(c, 200, dto.ToUserResponse(user))
    ↓
httpx.Success → c.Status(200).JSON({"data": ...})
```

Hata yolunda:
```
return httpx.ErrXxx (veya .WithDetails / .WithErr)
    ↓
Fiber → fiber.Config.ErrorHandler == httpx.Handler
    ↓
httpx.Handler → 5xx ise log → httpx.Error
    ↓
c.Status(appErr.HTTPStatus).JSON({"error": appErr})
```

---

## Paket Sözlüğü (tek satır özet)

| Paket                 | Görev                                             | Kim Bağımlı               |
|-----------------------|---------------------------------------------------|---------------------------|
| `cmd/server`          | Composition root (main)                           | Hepsi                     |
| `internal/config`     | Typed config + env loading                        | main, handler             |
| `internal/database`   | gorm.Open + Migrate + Seed                        | main                      |
| `internal/model`      | GORM modelleri                                    | her yer                   |
| `internal/dto`        | Request/response struct'ları + mapper             | handler                   |
| `internal/handler`    | HTTP handler struct'ları (DI'lı)                  | router, main              |
| `internal/middleware` | Auth + RequireRole factory'leri                   | router                    |
| `internal/router`     | Endpoint kayıt                                    | main                      |
| `internal/httpx`      | AppError + response zarfı + central error handler | handler, middleware, main |
| `pkg/token`           | JWT Manager (Generate/Parse)                      | handler, middleware, main |
| `pkg/validate`        | validator instance + Format()                     | handler, main             |

---

## Yeni Bir Entity (CRUD) Eklemek — Şablon

Author/Book/Library CRUD'ları bu kalıba göre yazılır. **Mimari karar yok**, yalnızca pattern uygulaması:

1. **`internal/dto/<entity>.go`** — `Create<E>Request`, `Update<E>Request`, `<E>Response` + `To<E>Response(m)` mapper. Validation tag'leri `required,min,max,email,eqfield` vs.
2. **`internal/handler/<entity>.go`** — Handler struct (sadece ihtiyaç duyduğu deps) + ctor + 5 metot:
   - `List(c)`   — GET /api/<entity>s
   - `Get(c)`    — GET /api/<entity>s/:id
   - `Create(c)` — POST /api/<entity>s
   - `Update(c)` — PUT /api/<entity>s/:id
   - `Delete(c)` — DELETE /api/<entity>s/:id
3. **`internal/router/router.go`** — `Deps`'e yeni handler ekle, protected group'a 5 route bağla.
4. **`cmd/server/main.go`** — `<E>Handler := handler.New<E>Handler(...)` + `Deps`'e ekle.

**Her handler metodu aynı 4 adımı izler:**
```
1) Parse  → Bind().Body(&req)  veya  ParamsInt("id")  → ErrBadRequest
2) Validate → val.Struct(&req)                          → ErrValidation.WithDetails(...)
3) DB op + hata dallandır  → errors.Is(gorm.ErrRecordNotFound) → ErrNotFound
                          → diğer hata → ErrInternal.WithErr(err)
4) Response → httpx.Success(c, status, dto.To<E>Response(m))
```

---

## Konvansiyonlar (Kısa)

- **Hata dönüşü**: Handler **asla** inline `c.Status().JSON()` yazmaz. `return httpx.ErrXxx` veya `httpx.Success(...)` kullanır.
- **DI**: Constructor injection. Global state YOK (paket-level değişkenler yok). Test'te struct'ı elle kurarak mock'la.
- **Sentinel'lar**: `httpx.ErrXxx` ve `gorm.ErrRecordNotFound` mutate edilmez. Detay/sarmalı hata için `WithDetails`/`WithErr`.
- **JSON tag**: snake_case (REST geleneksel).
- **Eğitim amaçlı yorumlar**: bu proje öğrenme amaçlı — kodda Türkçe açıklayıcı yorumlar **bilinçli olarak vardır**. Genel "no comments" kuralı bu repo için override.
- **`internal/`**: Go'nun built-in import-restriction kuralı — bu dizindeki paketler dışarıdan import edilemez.
- **`pkg/`**: Projeye özel olmayan, başka servislerde de kullanılabilecek genel kod (`token`, `validate`).

---

## Kapatılmış Güvenlik Açıkları (Faz 1)

Bu açıklar koda dönmesin diye listeli (regression yapma):
1. DB DSN hardcoded → `cfg.DBDSn`
2. JWT_SECRET repoda → `.gitignore` + rotate edildi
3. CORS wildcard → `cfg.AllowedOrigins` whitelist
4. JWT algorithm-confusion → `SigningMethodHMAC` type assertion (`pkg/token/jwt.go`)
5. Login user enumeration → tek tip `ErrInvalidCredentials` (mesaj + timing)
6. Login timing-attack → `dummyHash` ile sahte bcrypt
7. RoleID hardcoded → `cfg.DefaultRoleTitle` üzerinden DB lookup (privilege-escalation)
8. Auth middleware'de silinmiş user kabulü → her istekte DB'de id var mı kontrolü
9. Token'daki RoleID değişiklik sonrası kalıcı → DB'den taze çekiliyor
10. Cookie eksik flag'leri → `HTTPOnly` + `Secure` (cfg) + `SameSite=Lax`
11. Passwords mismatch 401 → `eqfield` validator → 400
12. DB ham hata mesajı sızıntısı → generic `ErrInternal`
13. `model.User` doğrudan JSON → `dto.UserResponse` mapper

---

## Faza Bağlı Çalışma Notları

- **Aktif faz** ne ise `todo.md`'nin başında **detaylı** olur. Sonraki fazlar tek satır.
- Bir faz tamamlandığında **bir sonraki faz açılıp detaylanır** — hepsi tek seferde planlanmaz.
- Kullanıcı **junior Go developer**; her adımda **kavram + syntax açıklaması** beklenir, **"sen yaz" denmedikçe doğrudan dosya değiştirilmez**. (Bkz. memory `feedback_workflow.md`.)
