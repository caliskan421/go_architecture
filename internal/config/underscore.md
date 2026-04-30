# Blank Identifier `_` — Tüm Kullanımları

`_` Go'da "blank identifier" (boş tanımlayıcı) denen özel bir isimdir. Anlamı: "bu
değeri umursamıyorum, sakla diye yer ayırma". Bu dosya `_`'nin **bütün** kullanımlarını
toplar; `config.go`'daki `_ = godotenv.Load()` sadece bir tanesi.

---

## Niye Var?

Go'nun katı kuralı: **kullanılmayan değişken ve import derleme hatasıdır**. Bu kural
kasıtlı; ölü kodu kaynakta bırakmasınlar diye. Ama bazı durumlarda bir değeri
**bilerek** atmak istersin. `_` o noktada devreye girer.

---

## Kullanım 1: Multiple Return'de İstemediğin Değer

Go fonksiyonları birden çok değer dönebilir. Hepsini kullanman zorunlu değil.

```go
// strconv.Atoi (string, error) döner.
n, _ := strconv.Atoi("42")
// hata umurumuzda değil; parse başarısızsa n = 0 zaten
```

Daha gerçek örnek: Fiber'in `Locals` çağrısı `(any, bool)` döner. Sadece değer lazımsa:

```go
v, _ := c.Locals("userID").(uint)
// type-assertion başarısız olursa v sıfır değer alır
```

### Ne zaman dikkatli ol?

`error`'u görmezden gelmek genelde kötü fikirdir. Aşağıdaki bug'a yol açar:

```go
data, _ := os.ReadFile("config.json")
// dosya yoksa data == nil, ama biz fark etmiyoruz
json.Unmarshal(data, &cfg)  // empty config sessizce devam eder
```

Hatayı yutmak istiyorsan **niyetini** koda yansıt:

```go
data, err := os.ReadFile("config.json")
if err != nil {
    // bilinçli karar: dosya yoksa boş config'le devam
    data = []byte("{}")
}
```

---

## Kullanım 2: Tek Return'lü Fonksiyon Çağrısında Niyet Beyanı

```go
godotenv.Load()        // derlenir, error sessizce atılır
_ = godotenv.Load()    // aynı şey ama "biliyorum" diyor
```

Davranış aynı: ikisi de error'u yutar. Fark **iletişim**:

- `godotenv.Load()` -> okuyucu "acaba unutuldu mu?" diye düşünebilir.
- `_ = godotenv.Load()` -> "bu hatayı kasten görmezden geliyorum" mesajı.

`errcheck`, `staticcheck` gibi linter'lar ilk biçimi flag'ler, ikincisini geçer.

### Performans?

Hiç fark yok. `_` derleme zamanı bir niyet beyanı; runtime'da hiçbir şey yapmaz, hiç
bellek tahsis etmez. Çalışan kod tıpatıp aynı.

```go
v := godotenv.Load()   // error tipinde değişken bellekte tutulur (8-16 byte)
_ = godotenv.Load()    // hiçbir bellek tahsisi yok, değer anında atılır
godotenv.Load()        // aynı şey, hiçbir bellek tahsisi yok
```

---

## Kullanım 3: `range`'de Index veya Value İstemediğinde

`range` slice/map için iki değer döner: index ve value. Birini istemiyorsan:

```go
// Sadece value lazım (index umursanmıyor)
for _, book := range books {
    fmt.Println(book.Name)
}

// Sadece index lazım
for i := range books {
    books[i].Counter++
}

// Map için: key-value, birini atabilirsin
for _, perm := range role.Permissions {
    set[perm] = struct{}{}
}
```

Bu projede `Authorizer.loadFromDB`'de bu örüntü var.

---

## Kullanım 4: Type Assertion'da `ok`'u Atmak

```go
// Genelde ok'u kontrol etmen gerekir:
roleID, ok := c.Locals("roleID").(uint)
if !ok {
    return httpx.ErrUnauthorized
}

// Ama eminseniz (test/mock kodu):
roleID := c.Locals("roleID").(uint)
// başarısız olursa panic (programlama hatası)
```

`_` burada genelde **kullanılmaz**, çünkü panic riski göze alınmıyorsa `ok`'u kontrol
etmek lazım. Ama `_, ok := ...` deyimi ile sadece "tip uyuyor mu" sorgusunu da yapabilirsin:

```go
_, ok := err.(*AppError)
if ok {
    // err *AppError tipinde
}
```

---

## Kullanım 5: `import _ "..."` — Side Effect Import'u

Bir paketi sadece **init fonksiyonunu çalıştırmak için** import etmek:

```go
import (
    _ "github.com/lib/pq"  // postgres driver'ı kaydeder
)
```

Paketi kodda kullanmıyoruz, ama import edilmiş olması gerekiyor — paketin `init()`
fonksiyonu DB driver'ını `database/sql`'e kaydediyor. `_`'siz import yazsan derlenmez
(kullanılmamış import). `_` "yalnızca yan etkisi için" sinyali veriyor.

GORM driver'larında da sık görünür ama bizim kodumuzda direkt API kullandığımız için yok.

---

## Kullanım 6: Interface Satisfaction Compile-Time Check

Bir struct'ın bir interface'i implement ettiğini **derleme anında** garanti etmek için:

```go
var _ error = (*AppError)(nil)
```

Anlamı: "`*AppError` `error` interface'ini implement etmiyorsa derleme başarısız olsun".
Çalışma zamanı bir şey yapmaz; sadece compiler'a kontrolü zorlatır. Bizim `httpx`
paketinde şu an kullanılmıyor ama iyi pattern olarak bilinmesi gerekir; özellikle
büyük interface'leri implement eden tipler için.

```go
// 5 metotlu bir interface'i atlamadığını garantilemek
var _ Repository = (*UserRepo)(nil)
```

`UserRepo` `Repository`'nin metotlarından birini eklemeyi unutursa, kullanım yerinde
değil **bu satırda** hata verir — hatanın yerini bulması kolaylaşır.

---

## Kullanım 7: Struct Field Adında — Padding/Tag

Nadir görünür ama mevcut: bir alanın yalnız tag'ini (örn. JSON marshalling için) tanımlamak
isteyip alanı boş bırakmak gibi durumlarda. Bu projede yok, üst düzey bilgi olarak burada.

---

## Özet Tablo

| Kullanım                          | Örnek                                  | Niye                                  |
|-----------------------------------|----------------------------------------|---------------------------------------|
| Multiple return discard           | `n, _ := strconv.Atoi(s)`              | Hata ya da değerden biri lazım değil  |
| Niyet beyanı (tek return)         | `_ = godotenv.Load()`                  | Linter'lar flag'lemesin               |
| `range` index/value atlama        | `for _, v := range slice { ... }`      | İki taraftan biri kullanılmıyor       |
| Type assertion sadece `ok`        | `_, ok := err.(*AppError)`             | Sadece tip uyumu sorgusu              |
| Side effect import                | `import _ "github.com/lib/pq"`         | Paket `init()`'ini çalıştır           |
| Compile-time interface check      | `var _ error = (*AppError)(nil)`       | Implementation'ı derleyiciye doğrulat |

---

## Niye Bu Kadar Çok Kullanım?

Go'nun "kullanılmayan değişken hata" kuralı yüzünden. Diğer dillerde `// unused`
yorumuyla geçiştirebileceğin şeyleri Go'da `_` ile **dilin parçası olarak**
geçiştirirsin. Bunun avantajı: niyet kodla okunaklı; dezavantajı: `_` ile çevrelenmiş
kod bazen anlamak zorlaşır.

## Tek Cümle

`_` Go'da "bu değeri umursamıyorum" demenin standart yoludur; runtime'da bir karşılığı
yoktur, sadece compiler'a ve okuyucuya niyet beyan eder — `_ = f()` ve `f()` aynı
çalışır ama biri "bilinçli", öteki "unutuldu mu?" mesajı verir.
