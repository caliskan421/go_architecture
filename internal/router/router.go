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
	Author   *handler.AuthorHandler
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

	// PROTECTED — Auth middleware'inden geçer.
	protected := app.Group("/api", middleware.Auth(deps.DB, deps.TokenMgr))
	protected.Get("/authors", deps.Author.List)
	protected.Get("/authors/:id", deps.Author.Get)
	protected.Post("/authors", deps.Author.Create)
	protected.Put("/authors/:id", deps.Author.Update)
	protected.Delete("/authors/:id", deps.Author.Delete)

	// userID ve roleID okuyabilir. Şu an boş; Faz 2+ ile dolacak.
	app.Group("/api", middleware.Auth(deps.DB, deps.TokenMgr))
}
