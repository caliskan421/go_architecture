# Dependency Injection (DI)

## Tanım

Bir nesnenin ihtiyaç duyduğu bağımlılıkları **kendi yaratmak yerine
dışarıdan parametre olarak alması**.

## Anti-pattern (DI YOK)

```go
func NewManager() *Manager {
    return &Manager{
        secret: []byte(os.Getenv("JWT_SECRET")),  // kendi okuyor
        ttl:    24 * time.Hour,                    // kendi belirliyor
    }
}
```

Sorunlar: test edilemez, env'e sıkı bağlı, gizli bağımlılıklar.

## DI'lı Hali

```go
func New(secret string, ttl time.Duration) *Manager {
    return &Manager{secret: []byte(secret), ttl: ttl}
}
```

Manager artık secret'ın nereden geldiğini bilmiyor — env, vault,
test değeri, hepsi olabilir.

## Composition Root (`main.go`)

Bağımlılık grafiği yukarıdan aşağıya `main.go`'da kurulur:

```go
func main() {
    cfg := config.Load()
    
    db, _ := db.Open(cfg)
    jwtMgr := jwt.New(cfg.JWTSecret, cfg.JWTTTL)
    
    userRepo := repo.NewUserRepo(db)
    authSvc  := service.NewAuthService(userRepo, jwtMgr)
    handler  := handler.NewAuthHandler(authSvc)
    
    app.Post("/login", handler.Login)
}
```

Her katman bağımlılığını **yukarıdan** alır, kendisi yaratmaz.

## Test Avantajı

```go
testMgr := jwt.New("test-secret", time.Hour)
mockRepo := &MockUserRepo{...}
svc := service.NewAuthService(mockRepo, testMgr)
// gerçek env'e, gerçek DB'ye dokunmuyoruz
```

## Go'da DI Felsefesi

- Container/framework'e gerek yok
- Constructor injection yeterli (`New(...)` parametreleri)
- Bağımlılıklar `main.go`'da bir kez kurulur, aşağıya geçirilir

## Sağladıkları

| Fayda              | Açıklama                                 |
|--------------------|------------------------------------------|
| Test edilebilirlik | Mock'lanabilir bağımlılıklar             |
| Esneklik           | Kaynak değişince sadece main.go değişir  |
| Açık bağımlılık    | İmza neye ihtiyaç duyduğunu söyler       |
| Tek sorumluluk     | Manager sadece JWT işi yapar, env okumaz |