package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config, uygulamanın ihtiyaç duyduğu tüm ayarları tek bir tipli yapıda toplar.
// Boot anında bir kere doldurulur ve handler/database/token gibi paketlere
// parametre olarak geçirilir. Böylece hiçbir alt paket os.Getenv'a bağımlı kalmaz.
type Config struct {
	DBDSn            string        // GORM'a verilecek MySQL bağlantı stringi
	JWTSecret        string        // JWT imzalama anahtarı (boş olamaz)
	JWTTTL           time.Duration // Token geçerlilik süresi (env'de saat olarak verilir)
	Port             string        // Fiber'in Listen() çağrısına gidecek ":3000" formatı
	AllowedOrigins   []string      // CORS whitelist; ALLOWED_ORIGINS env'inde virgülle ayrılır
	DefaultRoleTitle string        // Yeni kullanıcılara verilecek role'ün title'ı (DB'den ID'si çekilecek)
	CookieSecure     bool          // JWT cookie'sinde Secure flag — production'da true, http kullanılan dev'de false
}

// Load, .env dosyasını process env'ine yükler ve Config struct'ını doldurur.
// Zorunlu env değerleri eksikse log.Fatalf ile uygulamayı boot anında öldürür —
// böylece yanlış konfigle çalışan zombie bir server oluşmaz.
func Load() *Config {
	// godotenv.Load: .env dosyasındaki KEY=VALUE satırlarını os.Setenv ile process env'ine ekler.
	// Hata yutuluyor çünkü production'da .env dosyası bulunmayabilir; gerçek env'ler
	// (Docker, systemd, k8s) zaten process'e enjekte edilmiştir.
	_ = godotenv.Load()

	cfg := &Config{
		DBDSn:            mustGet("DB_DSN"),
		JWTSecret:        mustGet("JWT_SECRET"),
		JWTTTL:           time.Duration(getInt("JWT_TTL_HOURS", 24)) * time.Hour,
		Port:             ":" + getStr("PORT", "3000"),
		AllowedOrigins:   splitCSV(getStr("ALLOWED_ORIGINS", "*")),
		DefaultRoleTitle: getStr("DEFAULT_ROLE_TITLE", "user"),
		CookieSecure:     getStr("COOKIE_SECURE", "false") == "true",
	}

	return cfg
}

// mustGet, zorunlu bir env'i okur. Boşsa uygulamayı log.Fatalf ile öldürür.
// log.Fatalf: stderr'e formatlı mesaj yazar ve os.Exit(1) çağırır.
func mustGet(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("config: zorunlu env değişkeni eksik: %s", key)
	}
	return v
}

// getStr, opsiyonel bir env'i okur. Yoksa fallback döner.
func getStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getInt, env'i int olarak okur. Yoksa veya parse edilemezse fallback döner.
// strconv.Atoi: string'i int'e çevirir; hata dönerse fallback'e düşeriz.
func getInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Fatalf("config: %s sayı bekliyor, alındı: %q", key, v)
	}
	return n
}

// splitCSV, virgülle ayrılmış string'i []string'e çevirir; baş/son boşlukları temizler.
// strings.Split: ",", "a, b, c" -> ["a", " b", " c"] (boşluklar dahil) — bu yüzden TrimSpace.
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
