package main

import (
	"libra_management/internal/config"
	"libra_management/internal/database"
	"libra_management/internal/handler"
	"libra_management/internal/httpx"
	"libra_management/internal/router"
	"libra_management/pkg/token"
	"libra_management/pkg/validate"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/fiber/v3/middleware/cors"
)

// main, uygulamanın "composition root"udur: bağımlılıkları kurar ve birbirine takar.
// İş mantığı bu dosyada YOK — sadece kurulum sırası vardır.
//
// Sıra (her biri öncekine bağımlı):
//   config -> db open -> migrate -> seed -> token mgr -> validator -> handler -> fiber + cors -> router -> listen
func main() {
	// 1) Konfigürasyonu yükle. Zorunlu env eksikse Load() içinde log.Fatalf.
	cfg := config.Load()

	// 2) DB bağlantısı.
	db, err := database.Open(cfg)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}

	// 3) Şema migration — tüm modeller.
	if err := database.Migrate(db); err != nil {
		log.Fatalf("db migrate: %v", err)
	}

	// 4) Default kayıtları seed et (admin & user rolleri).
	if err := database.Seed(db); err != nil {
		log.Fatalf("db seed: %v", err)
	}

	// 5) Token manager (JWT).
	tokenMgr := token.New(cfg.JWTSecret, cfg.JWTTTL)

	// 6) Validator (paylaşılan instance — tag cache).
	val := validate.New()

	// 7) Handler instance'ı. ctor default role'ü DB'den çeker, dummy hash üretir.
	authHandler, err := handler.NewAuthHandler(db, cfg, tokenMgr, val)
	if err != nil {
		log.Fatalf("auth handler init: %v", err)
	}

	// 8) Fiber app — central error handler bağlandı; artık handler'lar `return apperror`
	//    diyebilir, JSON zarflama tek noktadan.
	app := fiber.New(fiber.Config{
		ErrorHandler: httpx.Handler,
	})

	// 9) CORS — wildcard "*" yerine cfg'den whitelist.
	//    AllowCredentials true: cookie tabanlı auth için zorunlu (browser cross-origin
	//    cookie göndersin diye).
	app.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.AllowedOrigins,
		AllowCredentials: true,
	}))

	// 10) Router — tüm endpoint'ler tek yerden bağlanır.
	router.Setup(app, router.Deps{
		Auth:     authHandler,
		DB:       db,
		TokenMgr: tokenMgr,
	})

	// 11) Listen.
	log.Fatal(app.Listen(cfg.Port))
}
