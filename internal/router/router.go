package router

import (
	"libra_management/internal/handler"
	"libra_management/internal/middleware"
	"libra_management/pkg/token"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

// Deps, router'ın bağlamak için ihtiyaç duyduğu bağımlılıkların tek bir kabı.
// İleride yeni handler veya middleware eklendikçe bu struct büyür; main.go ve
// router.Setup() arasındaki sözleşme tek noktada görünür.
type Deps struct {
	Auth     *handler.AuthHandler
	DB       *gorm.DB
	TokenMgr *token.Manager
}

// Setup, tüm endpoint'leri rotalara bağlar.
// Yapı: PUBLIC (auth uçları) ve PROTECTED (Auth middleware arkası) gruplar.
// Faz 2+'da Author/Book/Library handler'ları PROTECTED'a eklenecek.
func Setup(app *fiber.App, deps Deps) {
	// PUBLIC — kimlik doğrulama gerektirmez
	public := app.Group("/api")
	public.Post("/register", deps.Auth.Register)
	public.Post("/login", deps.Auth.Login)
	public.Post("/logout", deps.Auth.Logout)

	// PROTECTED — Auth middleware'inden geçer; sonraki handler'lar c.Locals'tan
	// userID ve roleID okuyabilir. Şu an boş; Faz 2+ ile dolacak.
	_ = app.Group("/api", middleware.Auth(deps.DB, deps.TokenMgr))
}
