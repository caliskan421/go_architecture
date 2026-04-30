# Migration: Şemayı Kod Gibi Versiyonlamak

Migration, **veritabanı şemasını kod gibi versiyonlayıp her ortamda aynı sonuca ulaşan
kontrollü değişiklik mekanizmasıdır**. Bu dosya: migration nedir, GORM'un `AutoMigrate`'i
nasıl çalışır, sınırı nedir, ne zaman manuel migration tool'larına geçilir.

---

## 1. Niye Migration?

Şema (tablolar, kolonlar, indeksler) zamanla değişir:

- "User'a `phone` alanı ekleyeceğiz."
- "Email kolonu `varchar(100)`'den `varchar(191)`'e çıksın."
- "Eski `is_active` kolonu kalksın."

Bu değişiklikleri:

- **Local'de** geliştirici, **CI'da** test runner, **production'da** deploy süreci
- aynı sonuca ulaştırarak yapmak zorunda.

Eğer şema değişikliklerini **tek tek elle** SQL ile uygularsan:

- Adımlardan biri unutulur.
- Local'de varsa, prod'da yoksa kaos olur.
- Geri almak (rollback) imkansız.

Migration: değişiklikleri **kod olarak** sakla; bir tool sırayla uygulasın. Versiyon
kontrolüne girer, code review'dan geçer, CI/CD'de tekrarlanabilir.

---

## 2. İki Yaklaşım

| Yaklaşım                  | Aracı                                | Esneklik | Karmaşıklık |
|---------------------------|--------------------------------------|----------|-------------|
| ORM otomatik migration    | GORM `AutoMigrate`, Hibernate `hbm2ddl` | Düşük | Düşük |
| Versiyonlu migration tool | golang-migrate, atlas, goose, alembic | Yüksek   | Orta-Yüksek |

Bu projede **ilki** kullanılıyor (`db.AutoMigrate(...)`). Sebebi: junior-friendly,
hızlı prototip, küçük bir uygulama. İlerde projeye girince eninde sonunda ikinciye
geçilir.

---

## 3. `AutoMigrate` Ne Yapar, Ne Yapmaz?

```go
db.AutoMigrate(
    &model.Permission{},
    &model.Role{},
    &model.User{},
    &model.Author{},
    &model.Book{},
    &model.Library{},
)
```

GORM verilen struct'ları okur, mevcut tablolarla karşılaştırır, **idempotent** şekilde
şemayı senkronize eder.

| İşlem                        | AutoMigrate                                    | Manuel  |
|------------------------------|------------------------------------------------|---------|
| Tablo oluşturma              | Yapar                                          | -       |
| Yeni kolon ekleme            | Yapar                                          | -       |
| Index ekleme                 | Yapar (tag'den okuyarak)                       | -       |
| Kolon silme                  | YAPMAZ                                         | Gerekir |
| Kolon tipi değiştirme        | Sınırlı (string boyutu büyütme tamam, küçültme tehlikeli) | Gerekir |
| Kolon yeniden adlandırma     | YAPMAZ (silip yeniden ekler -> veri kaybı riski) | Gerekir |
| Tablo silme                  | YAPMAZ                                         | Gerekir |
| Veri taşıma (data migration) | YAPMAZ                                         | Gerekir |
| Default value değiştirme     | YAPMAZ (mevcut satırlara dokunmaz)             | Gerekir |

**Niye yapmaz?** Veri kaybı riski olan operasyonlar GORM'un kapsamı dışında bırakılmış —
"sessizce kolon silmek" production'da felakettir, deliberate decision olmalı.

---

## 4. Idempotency: Aynı Çağrıyı Tekrarlamak

```go
db.AutoMigrate(&model.User{})  // 1. çağrı: tablo yarat
db.AutoMigrate(&model.User{})  // 2. çağrı: hiçbir şey yapma (zaten var)
db.AutoMigrate(&model.User{})  // 3. çağrı: aynı
```

Boot'ta `Migrate(db)` çağrısı her seferinde yapılır. Idempotent olduğu için zarar
yok; tablo zaten varsa GORM aksiyon almaz. Yeni bir kolon eklediysen bir sonraki boot'ta
onu fark eder ve `ALTER TABLE ADD COLUMN` çalıştırır.

---

## 5. Boot-Time Migrate'in Avantaj/Dezavantajı

```go
// main.go
if err := database.Migrate(db); err != nil {
    log.Fatalf("db migrate: %v", err)
}
```

### Avantaj

- Deploy adımı yok; sadece binary çalıştırılır, şema kendi senkronize olur.
- Junior-friendly; ayrı bir tool öğrenmek gerekmiyor.

### Dezavantaj (production-grade için)

- **Birden fazla pod aynı anda boot olursa** ikisi de migrate çalıştırır; nadir ama
  race condition olur. (Tek replica'da sorun yok.)
- **Geri alma yok**: yanlış bir migration deploy edilirse rollback adımı manuel.
- **Görünmez**: hangi versiyonun uygulandığını dışarıdan göremezsin (versiyon tablosu yok).
- **Uzun migration boot'u kilitler**: 10 milyon satırlık tabloya `ADD COLUMN` çalışırsa
  app dakikalarca açılmaz.

Bu yüzden olgunlaşan projeler genelde **ayrı bir migration adımı** kullanır:

```
deploy adımları:
  1. migrate komutu çalıştır (golang-migrate up)
  2. yeni binary deploy et
```

---

## 6. Manuel Migration Tool'ları

`AutoMigrate`'in yetmediği durumda Go ekosisteminde popüler seçenekler:

| Tool                                              | Yaklaşım                            |
|---------------------------------------------------|-------------------------------------|
| [golang-migrate](https://github.com/golang-migrate/migrate) | SQL dosyaları (`up.sql`/`down.sql`) |
| [pressly/goose](https://github.com/pressly/goose) | SQL veya Go fonksiyonu              |
| [ariga/atlas](https://atlasgo.io)                 | Schema-as-code, declarative + diff  |

Tipik dosya yapısı (golang-migrate):

```
migrations/
  001_create_users.up.sql
  001_create_users.down.sql
  002_add_phone_to_users.up.sql
  002_add_phone_to_users.down.sql
```

Tool DB'de `schema_migrations` adında bir tabloda hangi versiyonların uygulandığını
takip eder. `up` ileri gider, `down` geri alır.

### Ne Zaman Geçilir?

- İlk kolon silme/rename ihtiyacı geldiğinde.
- Production'da veri taşıma gerekince (örn. `full_name` -> `first_name + last_name`).
- Birden çok geliştirici aynı anda şema değiştirince.

Bu projede şu an gereği yok; geçilirse `database.Migrate` fonksiyonu silinir, deploy
script'i tool'u çağırır.

---

## 7. GORM Tag'leri ile Şema Detayı

`AutoMigrate` struct tag'lerini okur, şemayı tag'lere göre düzenler.

```go
type User struct {
    gorm.Model
    Email string `gorm:"uniqueIndex;size:191"`
    Password string `json:"-"`
}
```

Burada:

| Tag                | Etkisi                                                                |
|--------------------|-----------------------------------------------------------------------|
| `uniqueIndex`      | `email` kolonuna unique index ekler — duplicate insert'te `ErrDuplicatedKey` |
| `size:191`         | `varchar(191)` (utf8mb4 + 4-byte multi-byte için MySQL'in eski limiti)|
| `primaryKey`       | Custom PK tanımlamak için (gorm.Model'de zaten var)                   |
| `foreignKey:RoleID`| Bu alanın FK olduğunu söyler, hangi alana bağlı olduğunu belirtir     |
| `many2many:tablo`  | M2M ilişki için join tablosunu adlandırır                             |
| `not null`         | NOT NULL constraint                                                   |
| `default:0`        | Default değer                                                          |

Detaylı liste: `internal/model/gorm-tags.md`.

`gorm.Model` neden çekiliyor:

```go
type Model struct {
    ID        uint           `gorm:"primaryKey"`
    CreatedAt time.Time
    UpdatedAt time.Time
    DeletedAt gorm.DeletedAt `gorm:"index"`  // soft delete
}
```

User soft-deletable. Author/Book/Library `gorm.Model` kullanmıyor — bilinçli tercih
(bkz. ilgili model yorumları).

---

## 8. Seed vs Migrate

| Aşama   | Sorumluluk                                |
|---------|-------------------------------------------|
| Migrate | **Şema** — tablolar, kolonlar, indeksler  |
| Seed    | **Veri** — default kayıtlar (admin/user role, izinler) |

Seed migration'dan sonra çalışır. Sırası önemli; izinler olmadan rolleri izinlere
bağlayamazsın. `database.Seed`'in iç sırası:

```
seedPermissions  ->  seedRoles  ->  syncRolePermissions
```

Seed da idempotent: `FirstOrCreate` ile çağrılır; varsa eklemez, yoksa ekler.

---

## Tek Cümle

GORM'un `AutoMigrate`'i tabloları/kolonları/indeksleri **eksik kalanları ekleyerek**
senkronize eder ama **silmez ve yeniden adlandırmaz**; bu yüzden geliştirme ve küçük
projelerde yeterli, ama veri kaybı riski olan operasyonlar için golang-migrate / atlas
gibi versiyonlu migration tool'larına geçmek gerekir — ne zaman? "Kolon silmem lazım"
dediğin gün.
