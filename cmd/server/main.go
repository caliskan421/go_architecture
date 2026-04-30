package main

import (
	"log/slog"
	"os"

	"libra_management/internal/config"
	"libra_management/internal/database"
	"libra_management/internal/handler"
	"libra_management/internal/httpx"
	"libra_management/internal/middleware"
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
//
//	slog -> config -> db open -> migrate -> seed -> token mgr -> validator
//	  -> handler -> authorizer -> fiber + cors + request-id -> router -> listen
func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg := config.Load()

	db, err := database.Open(cfg)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}

	if err := database.Migrate(db); err != nil {
		log.Fatalf("db migrate: %v", err)
	}

	if err := database.Seed(db); err != nil {
		log.Fatalf("db seed: %v", err)
	}

	tokenMgr := token.New(cfg.JWTSecret, cfg.JWTTTL)

	// 7) Validator (paylaşılan instance — tag cache).
	val := validate.New()

	// 8) Handler instance'ları.
	authHandler, err := handler.NewAuthHandler(db, cfg, tokenMgr, val)
	if err != nil {
		log.Fatalf("auth handler init: %v", err)
	}
	authorHandler := handler.NewAuthorHandler(db, val)
	bookHandler := handler.NewBookHandler(db, val)
	libraryHandler := handler.NewLibraryHandler(db, val)

	// 9) Authorizer: rol-izin çözümlemesini cache'leyen RBAC servisi.
	authorizer := middleware.NewAuthorizer(db)

	// 10) Fiber app — central error handler bağlandı.
	app := fiber.New(fiber.Config{
		ErrorHandler: httpx.Handler,
	})

	// 11) Global middleware'ler — sıra ÖNEMLİ:
	//   a) Request-ID önce: log'da her request bir id'yle eşleşsin.
	//   b) CORS sonra: 401/403 dönen requestler de CORS header alsın.
	app.Use(middleware.RequestID())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.AllowedOrigins,
		AllowCredentials: true,
	}))

	// 12) Router — tüm endpoint'ler tek yerden bağlanır.
	router.Setup(app, router.Deps{
		Auth:       authHandler,
		Author:     authorHandler,
		Book:       bookHandler,
		Library:    libraryHandler,
		DB:         db,
		TokenMgr:   tokenMgr,
		Authorizer: authorizer,
	})

	// 13) Listen.
	slog.Info("server starting", "port", cfg.Port)
	log.Fatal(app.Listen(cfg.Port))
}
