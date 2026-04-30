# Struct mu, Sadece Fonksiyon mu?

## Kural

- **Sadece fonksiyon**: hiçbir state yok, parametreler girer, sonuç çıkar
- **Struct + metod**: birden çok fonksiyon aynı veriyi paylaşıyor veya
  veri çağrılar arasında "aynı kalmalı"

## Saf Fonksiyon Örneği (state yok)

```go
func HashPassword(plain string) (string, error) {
    return bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
}
```
## State'li Örnek (struct şart)

```go
type Manager struct {
    secret []byte
    ttl    time.Duration
}
func (m *Manager) Generate(userID uint) (string, error) {
// m.secret ve m.ttl otomatik elde, her çağrıda parametre geçmiyoruz
}
```

## Struct'ın Verdiği Şey

- **Encapsulation**: ilgili veri + davranış aynı kutuda
- **Tekrar yok**: config'i her çağrıda parametre olarak geçmeye gerek yok
- **Private field'lar**: küçük harf → dışarıdan değiştirilemez (immutable)

## Karar Tablosu

| Durum                                                      | Seçim                |
|------------------------------------------------------------|----------------------|
| Fonksiyon sadece parametrelerle çalışıyor                  | Fonksiyon            |
| Aynı veri tekrar tekrar lazım (config, connection, secret) | Struct               |
| State bir kez kurulup hep aynı kalmalı                     | Struct + constructor |