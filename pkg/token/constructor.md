# Constructor Pattern (`New` Fonksiyonu)

## Yapı

```go
type Manager struct {
    secret []byte         // private (küçük harf)
    ttl    time.Duration
}

func New(secret string, ttl time.Duration) *Manager {
    return &Manager{
        secret: []byte(secret),
        ttl:    ttl,
    }
}
```

## Ne Sağlar?

1. **Tek giriş noktası**: Manager sadece New üzerinden yaratılır
2. **Validation imkanı**: New içinde "secret boşsa hata ver" gibi kontroller yapılabilir
3. **Pahalı dönüşümler bir kez**: `[]byte(secret)` her token üretiminde değil, sadece kurulumda
4. **İmmutability**: field'lar private, dışarıdan değiştirilemez

## Field Görünürlüğü Önemli

```go
mgr.secret = []byte("hacked")  // ❌ derlenmez, private
mgr.Secret = []byte("hacked")  // ✅ derlenir → güvenlik açığı
```

Bu yüzden bağımlılıkları `New`'den geçirip private tut.

## Çalışırken Yaşam Döngüsü

```
main.go
  ↓ New(secret, ttl)  ← bir kez
Manager doğdu
  ↓
binlerce istek hep aynı Manager'ı kullanır
  ↓
uygulama kapanınca Manager ölür
```
