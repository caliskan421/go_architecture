# Environment Variable İşleyişi: godotenv + os.Getenv

Bu dosya `Load()` fonksiyonunda gördüğün iki çağrının (`godotenv.Load()` ve `os.Getenv(...)`)
**arka planda ne yaptığını** anlatır. Kavram sırası: process environment nedir, kim
yazar, kim okur.

---

## 1. Process Environment Nedir?

Her process işletim sisteminden, başlarken **kendine ait** bir `key=value` listesi alır.
Buna o process'in "environment"'ı denir.

Önemli noktalar:

- Bu liste process'in **kendi belleğinde** yaşar. OS'ta merkezi bir havuz değildir.
- Bir process başka bir process'in env'ini doğrudan değiştiremez.
- Child process başlatıldığında parent'ın env'inin **kopyasını** alır.
- Process yaşam süresince env değişebilir (`os.Setenv`), ama dışarıya yansımaz.

```
[OS]
 ├── Process A (env: DB_DSN=..., PORT=3000)
 ├── Process B (env: AWS_KEY=..., HOME=/root)
 └── Process C (env: ...)
```

Her process'in env'i ayrı bir bellek bölgesidir.

---

## 2. Env'e Kim Yazar?

Birkaç kaynak vardır; hepsi aynı bellek bölgesine yazar:

| Kaynak                       | Ne zaman                  |
|------------------------------|---------------------------|
| Shell `export` komutu        | Process başlamadan önce   |
| Docker `-e KEY=VALUE`        | Container başlatılırken   |
| Kubernetes `env:` manifest'i | Pod oluşturulurken        |
| systemd `Environment=`       | Service başlatılırken     |
| `godotenv.Load()`            | Process içinde, runtime'da|
| `os.Setenv(key, val)`        | Process içinde, runtime'da|

Production'da genelde ilk dördü kullanılır. `.env` dosyaları tipik olarak local
geliştirme içindir.

---

## 3. `godotenv.Load()` — Yazıcı

```go
import "github.com/joho/godotenv"

_ = godotenv.Load()
```

Üçüncü parti kütüphane (`github.com/joho/godotenv`). İşi:

1. Çalışma dizinindeki `.env` dosyasını **disk'ten okur**.
2. Her satırı parse eder (`KEY=VALUE` formatında).
3. `os.Setenv(key, value)` ile process'in env belleğine yükler.

```
[disk]                    [process bellek]
.env dosyası   ──────>    KEY1=...
KEY1=...      Load()      KEY2=...
KEY2=...                  ...
```

Önemli davranışlar:

- Mevcut değerleri **ezmez**. Yani önce shell'den `export DB_DSN=prod` deyip sonra
  `.env`'de `DB_DSN=local` olsa bile `prod` kalır. Bu kasıtlıdır: production'da
  OS env'i öncelikli olmalı, dosya bunu override etmemeli.
- Dosya yoksa **error döner**, panic etmez. Production'da `.env` dosyası bulunmayabilir
  (env'ler zaten Docker/K8s tarafından enjekte edildiği için). Bu yüzden bizim kodumuzda
  hatayı yutuyoruz: `_ = godotenv.Load()`.

---

## 4. `os.Getenv(key)` — Okuyucu

```go
import "os"

v := os.Getenv("DB_DSN")
```

Go standart kütüphanesi, `os` paketine ait fonksiyon. İşi:

- Process'in env belleğinden anahtarın değerini okur.
- Dosya bilmez, OS'a syscall atmaz; sadece **in-memory lookup**.
- Anahtar varsa string değer döner; yoksa `""` (boş string) döner. Hata fırlatmaz.

### "Tanımlı ama boş" vs "hiç tanımlı değil"

`os.Getenv` ikisini de `""` döndürür, ayırt edemezsin. Ayırt etmen gerekiyorsa:

```go
v, ok := os.LookupEnv("DB_DSN")
// v: değer
// ok: env tanımlı mı (true/false)
```

Bu projede ayrımın önemi yok — `mustGet` boş veya tanımsız ikisini de "yok" sayıyor.

---

## 5. Akış

```
.env dosyası (disk)
        |
        |  godotenv.Load()  ->  parse + os.Setenv
        v
Process environment (bellek)   <--  Shell export, Docker -e,
        |                            K8s env, systemd Environment=
        |  os.Getenv(key)            de buraya yazar.
        v
Senin kodun (config.Load)
```

İki çağrı arasındaki rol farkı:

| Çağrı              | Yön                | Sıklık       |
|--------------------|--------------------|--------------|
| `godotenv.Load()`  | dosya -> bellek    | 1 kez (boot) |
| `os.Getenv(key)`   | bellek -> kod      | n kez (her okuma) |

---

## 6. `mustGet` ve `log.Fatalf` — Boot'ta Sertçe Ölmek

```go
func mustGet(key string) string {
    v := os.Getenv(key)
    if v == "" {
        log.Fatalf("config: zorunlu env değişkeni eksik: %s", key)
    }
    return v
}
```

`log.Fatalf` aslında iki şeyi birden yapan bir kombinasyondur:

1. `log.Printf` ile stderr'e formatlı mesaj yazar.
2. `os.Exit(1)` ile programı **anında** sonlandırır.

### `os.Exit` Nedir?

Programı anında sonlandıran fonksiyon. Aldığı sayı **exit code**'dur:

| Exit code | Anlam                                |
|-----------|--------------------------------------|
| `0`       | Başarılı bitiş                       |
| `1`       | Hata ile bitiş (yaygın)              |
| `2`, `3`...| Projeye göre özel anlamlar         |

Bu sayı OS'a dönen bir sinyaldir; CI/CD, Docker, Kubernetes gibi sistemler
exit code'a bakarak "uygulama düzgün başladı mı, çöktü mü" anlar:

```bash
go run ./cmd/server
echo $?    # son komutun exit code'unu gösterir
# 0 = başarılı, 1+ = hata
```

K8s `1` görürse pod'u restart eder, alarm verir.

### `os.Exit` vs `panic` — Niye `os.Exit`?

İkisi de programı durdurur ama farklı şekillerde:

| Davranış                | `panic`                           | `os.Exit`                  |
|-------------------------|-----------------------------------|----------------------------|
| `defer` çalışır mı?     | Evet (stack geri sarılır)         | Hayır (anında çıkar)       |
| Yakalanabilir mi?       | Evet (`recover` ile)              | Hayır                      |
| Stack trace yazar mı?   | Evet                              | Hayır (mesajı sen yazarsın)|
| Anlamı                  | "Beklenmedik durum"               | "Bilinçli sonlandırma"     |

Boot anında zorunlu config eksikse, bu **bilinçli sonlandırmadır**: yanlış konfigle
çalışan zombie bir server yaratmak istemiyoruz. `panic` daha çok "bu olmamalıydı,
bug var" semantiği taşır. Bu yüzden `log.Fatalf` doğru seçim.

---

## 7. `_ = godotenv.Load()` Niye?

Go'da kullanılmayan return değeri derleme hatasıdır. `godotenv.Load()` `error` döner.
Üç yazım:

```go
v := godotenv.Load()   // error tipinde değişken bellekte tutulur
_ = godotenv.Load()    // değer atılır, derlenir
godotenv.Load()        // hata: kullanılmayan return değeri (eğer return tek değilse)
```

Aslında Go bir return değerini görmezden gelmene **izin verir** — derleme hatası
**atanmamış değişken** için çıkar, return değeri için değil. Yani:

```go
godotenv.Load()  // bu derlenir, error sessizce yutulur
```

`_ = godotenv.Load()` yazmamızın sebebi farklı: **niyet beyanı**. Linter ve okuyan
geliştirici "buradaki hatayı bilinçli olarak yutuyoruz" der. `errcheck` gibi linter'lar
bu satırı flag'lemez. Daha açık bir kod.

Bkz. `underscore.md` — blank identifier'ın diğer kullanımları.

---

## Tek Cümle

`godotenv.Load()` `.env` dosyasını **diskten** alıp process'in env belleğine yazar
(boot'ta bir kez), `os.Getenv(key)` ise o bellekten **anahtarın değerini** çeker
(istediğin kadar) — ikisi de aynı bellek bölgesinde buluşur, biri yazıcı diğeri okuyucu;
production'da OS zaten env'i enjekte ettiği için godotenv yoksa da kod çalışır, bu yüzden
hata `_ =` ile bilinçli yutulur.
