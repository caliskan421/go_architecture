# Config Alanları — Tek Tek Açıklama

`Config` struct'ındaki her alan **neden var, neden bu tipte, neden env'den geliyor**.
Aşağıdaki anlatım `internal/config/config.go` dosyasını referans alır.

---

## 1. `DBDSn string` — Veritabanı Bağlantı String'i

DSN = "Data Source Name". Veritabanına nasıl bağlanılacağını anlatan tek satırlık adres.

```
user:password@tcp(localhost:3306)/mydb?parseTime=true&charset=utf8mb4
```

Parçaları:

| Parça             | Anlamı                                  |
|-------------------|-----------------------------------------|
| `user:password`   | DB kullanıcısı ve şifresi               |
| `tcp(host:port)`  | Bağlantı protokolü ve sunucu adresi     |
| `/mydb`           | Hangi veritabanı (schema)               |
| `?parseTime=true` | MySQL `DATETIME` -> Go `time.Time`      |
| `&charset=utf8mb4` | Türkçe karakter ve emoji desteği için  |

GORM bu string'i `mysql.Open(dsn)` ile alır ve bağlantı havuzu açar.

### Neden env'den geliyor?

- **Local'de**: `localhost:3306`'daki dev DB'ye bağlanırsın
- **Production'da**: AWS RDS / Cloud SQL gibi bir endpoint'e bağlanırsın
- **Şifre koda yazılamaz** — repo'ya commit edilirse public'e sızar (Faz 1'de düzeltilen
  açıklardan biri tam buydu).

Aynı kod, farklı DSN'lerle farklı veritabanlarına bağlanır. Kod değişmez, sadece env değişir.

### Neden `string`?

GORM'a verilen format zaten string. Parse etmeye gerek yok — gerekirse GORM driver'ı
kendisi parse eder. Biz tipleri abartmıyoruz, kütüphane API'sine uyuyoruz.

---

## 2. `JWTSecret string` — JWT İmzalama Anahtarı

JWT = JSON Web Token. Kullanıcı login olduğunda backend'in ona verdiği "kimlik kartı".

Token üç parçadan oluşur, hepsi base64 encoded ve nokta ile ayrılmıştır:

```
eyJhbGciOiJIUzI1NiJ9 . eyJ1c2VyX2lkIjoxMjN9 . SflKxw...imza
└── header ──────┘   └── payload ────┘   └── imza ─┘
```

İmza kısmı `JWTSecret` ile üretilir. Mantık:

1. Kullanıcı giriş yapar -> backend bir JWT oluşturur, **secret ile imzalar**, kullanıcıya verir.
2. Kullanıcı sonraki isteklerde bu JWT'yi cookie/header ile gönderir.
3. Backend JWT'yi alır, **aynı secret ile imzayı doğrular**.
4. İmza tutuyorsa "evet bu token'ı ben verdim, sahte değil" der.

### Neden bu kadar önemli?

Eğer secret sızarsa saldırgan istediği `user_id` ile kendi JWT'sini üretebilir; backend
imzayı doğru hesaplar (çünkü secret aynı), saldırgan admin gibi davranır. Bu yüzden:

- Asla koda yazılmaz, repo'ya commit edilmez (`.gitignore`'da `.env`).
- Env'den gelir; production'da çok güçlü ve rastgele olur (örn. `openssl rand -base64 32`).
- `mustGet` ile **zorunlu** tutulur — yoksa uygulama açılmaz. Sebebi: yanlışlıkla boş
  secret ile çalışan bir backend, herkesin imzasız token üretmesine kapı açar.

### Neden `string` (bu aşamada)?

Token kütüphanesi (`golang-jwt/jwt/v5`) `[]byte` ister, ama biz Config'de `string` olarak
tutuyoruz çünkü env zaten string döner. `[]byte`'a dönüşümü constructor'da bir kez yapıyoruz
(`pkg/token/jwt.go` -> `New()`), her token üretiminde değil. Konvertörün konumu önemlidir:
sıcak yolda dönüşüm yapmak israftır.

---

## 3. `JWTTTL time.Duration` — Token Geçerlilik Süresi

TTL = "Time To Live". JWT'nin ne kadar geçerli kalacağını belirler.

```go
JWTTTL: time.Duration(getInt("JWT_TTL_HOURS", 24)) * time.Hour
```

Token üretilirken payload'a `exp` (expires at) claim'i yazılır. Süre dolduğunda kütüphane
kendisi reddeder; sen ekstra kod yazmazsın.

### Neden ayarlanabilir?

Güvenlik vs. kullanım kolaylığı dengesi:

| TTL                      | Avantaj           | Dezavantaj                                 |
|--------------------------|-------------------|--------------------------------------------|
| Kısa (15 dk)             | Çalınsa az tehlikeli | Kullanıcı sürekli login olur            |
| Uzun (7 gün)             | Rahat kullanım    | Çalınırsa uzun süre saldırgan elinde     |

Banka uygulamaları kısa, sosyal medya uzun TTL kullanır. Sen ortama göre env'den ayarlarsın.

### Neden `time.Duration`?

Go'nun standart süre tipi. `time.Hour`, `time.Minute`, `time.Second` gibi sabitlerle
çarpılarak yazılır. Saniyeyi int olarak taşımak yerine `time.Duration` kullanmanın
faydası: tip-güvenli aritmetik. `time.Now().Add(ttl)` doğal görünür; `time.Now().Add(int)`
derlenmez.

### Neden env'den int (saat) okunuyor?

Env sadece string tutar. `JWT_TTL_HOURS=24` yaygın ve okunabilir bir gösterim — birim
operatöre belli. Kod tarafında `* time.Hour` ile `time.Duration`'a yükseltiyoruz.

---

## 4. `Port string` — Sunucu Portu

Backend'in dinleyeceği port numarası.

```go
Port: ":" + getStr("PORT", "3000")  // ":3000"
```

Sunucu sonunda `app.Listen(cfg.Port)` çağırır. Fiber bu string'i `net.Listen`'a verir.

### Neden başında iki nokta `:`?

`net.Listen("tcp", ":3000")` -> "tüm interface'lerde 3000 portunu dinle".
`net.Listen("tcp", "localhost:3000")` -> "sadece localhost'ta dinle, dışarıdan erişim yok".

Sadece `3000` yazsan stdlib hata verir — adresin formatı `[host]:port` olmak zorunda.

### Neden env'den?

- Local'de 3000 ile geliştirirsin.
- Production'da Heroku/Railway/Cloud Run gibi platformlar `PORT` env'ini **otomatik**
  atar; sen seçemezsin. Eğer "3000" hardcode etseydin uygulaman bu platformlarda
  başlayamazdı.
- Aynı makinede birden fazla servis varsa portlar çakışmasın.

### Neden `string` ve neden default `"3000"`?

`":3000"` formatı bizim için string. Default 3000: Node.js/Express dünyasından gelen
yaygın bir konvansiyon, geliştiriciler için tanıdık.

---

## 5. `AllowedOrigins []string` — CORS Whitelist

CORS = Cross-Origin Resource Sharing. Tarayıcıların güvenlik mekanizması.

### Senaryo

Frontend `https://myapp.com`'da, backend `https://api.myapp.com`'da. Tarayıcı
varsayılan olarak farklı domain'lere yapılan AJAX isteklerini engeller. Backend'in
açıkça "evet, bu domain'den gelen istekleri kabul ediyorum" demesi gerekir.

```go
AllowedOrigins: []string{"https://myapp.com", "https://admin.myapp.com"}
```

Backend her isteğin `Origin` header'ına bakar:

- Origin listede varsa -> izin ver, response'a `Access-Control-Allow-Origin` koy.
- Yoksa -> tarayıcı isteği reddeder (CORS hatası).

### Neden env'den ve neden CSV?

Env yalnızca string tutar, array tutamaz. Bu yüzden virgülle ayrılmış (CSV) bir string
veriyoruz, kod tarafında parse ediyoruz:

```
ALLOWED_ORIGINS=https://myapp.com,https://admin.myapp.com,http://localhost:3000
```

`splitCSV` fonksiyonu bu string'i `[]string`'e çevirir, baş/son boşlukları temizler
(`TrimSpace`).

### Neden `*` tehlikeli?

`*` "her yerden kabul" demek. Public ve hassas veri olmayan API'larda kullanılır.
Üretimde `*` koyman, kötü niyetli bir sitenin kullanıcının tarayıcısı üzerinden
backend'ine istek atmasını mümkün kılar. Default `"*"` koyduk ama production'da
**mutlaka** override edilmeli.

---

## 6. `DefaultRoleTitle string` — Yeni Kullanıcı Rolü

Authorization (yetkilendirme) sistemiyle ilgili. `roles` tablosu:

```
id | title
1  | admin
2  | user
```

Yeni biri kayıt olduğunda hangi role'ü alacak? Genelde `"user"` — en kısıtlı yetki.

### Neden ID değil de title?

Eski kodda hardcoded `RoleID: 2` vardı. Sorun: migration sırası değişirse veya
seed başka türlü çalışırsa "user" rolünün ID'si 2 olmayabilir. Yeni kayıt yanlışlıkla
admin olur. Bu, **privilege-escalation** açığıydı (Faz 1'de kapatıldı).

Çözüm: env'de title yaz, kodda title'ı kullanarak DB'den ID'yi sorgula:

```go
// boot anında bir kez
var role model.Role
db.Where("title = ?", cfg.DefaultRoleTitle).First(&role)
defaultRoleID := role.ID  // cache'lendi
```

Title sabit ve okunabilir; ID veritabanına göre değişebilir.

### Neden cache'leniyor?

`title` -> `id` lookup'ı her register'da tekrar yapmak verimsiz; rol ID'si production'da
değişmez. Bkz. `internal/handler/cache.md`.

---

## 7. `CookieSecure bool` — Cookie'nin Secure Bayrağı

JWT'yi kullanıcıya iki yolla iletebilirsin:

1. Response body'de döner -> frontend `localStorage`'a koyar (XSS'e açık).
2. **HTTP cookie olarak set edersin** (modern ve güvenli yaklaşım).

Biz 2. yolu kullanıyoruz. Cookie'lerin `Secure` bayrağı:

- `Secure: true` -> cookie sadece **HTTPS** üzerinden gönderilir.
- `Secure: false` -> HTTP'de de gönderilir.

### Neden ortama göre değişir?

```
Production: HTTPS kullanılıyor
COOKIE_SECURE=true  -> cookie güvende, sadece şifreli bağlantıda gider

Development: localhost'ta HTTP
COOKIE_SECURE=false -> yoksa cookie hiç gönderilmez, login çalışmaz görünür
```

Yani local'de `false`, production'da `true`. Eğer prod'da yanlışlıkla `false` bırakırsan
kullanıcının JWT'si açık WiFi'da koklanabilir (man-in-the-middle saldırısı). `true` bırakırsan
dev'de cookie tarayıcıya hiç gelmez ve login bozulur.

### Neden `bool` ve neden bu satır?

```go
CookieSecure: getStr("COOKIE_SECURE", "false") == "true"
```

Env her zaman string döner. "true" string'iyle karşılaştırıp bool'a çeviriyoruz.
`strconv.ParseBool` da kullanılabilirdi (true/TRUE/1/0 vs. kabul eder) ama bizim
sözleşmemiz net: `"true"` veya değil. Daha az implicit davranış, daha az sürpriz.

---

## Özet Tablo

| Alan               | Tip             | Zorunlu | Default          | Niye env'den           |
|--------------------|-----------------|---------|------------------|------------------------|
| `DBDSn`            | `string`        | Evet    | -                | Şifre içerir + ortam değişir |
| `JWTSecret`        | `string`        | Evet    | -                | Sızması felaket         |
| `JWTTTL`           | `time.Duration` | Hayır   | 24h              | Güvenlik/UX dengesi     |
| `Port`             | `string`        | Hayır   | `:3000`          | PaaS otomatik atar      |
| `AllowedOrigins`   | `[]string`      | Hayır   | `*`              | Domain'ler ortamdan ortama değişir |
| `DefaultRoleTitle` | `string`        | Hayır   | `user`           | Kayıt davranışını değiştirebilmek |
| `CookieSecure`     | `bool`          | Hayır   | `false`          | HTTP/HTTPS ayrımı       |

## Tek Cümle

Config struct'ı uygulamanın **tüm** dış parametrelerini tip-güvenli bir kapta toplar;
zorunlu olanlar (`DBDSn`, `JWTSecret`) yoksa uygulama açılmaz, opsiyoneller mantıklı
default ile gelir — bu sayede hiçbir alt paket `os.Getenv` çağırmak zorunda kalmaz,
tek doğruluk noktası burasıdır.
