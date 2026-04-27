package router

import (
	"libra_management/internal/auth"
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
	Auth       *handler.AuthHandler
	Author     *handler.AuthorHandler
	Book       *handler.BookHandler
	Library    *handler.LibraryHandler
	DB         *gorm.DB
	TokenMgr   *token.Manager
	Authorizer *middleware.Authorizer
}

// Setup, tüm endpoint'leri rotalara bağlar.
// Yapı: PUBLIC (auth uçları) ve PROTECTED (Auth middleware arkası) gruplar.
//
// Yetkilendirme: protected grubunun altında her endpoint kendi izin
// gereksinimini RequirePermission ile beyan eder. Faz 5'te seed eklenen
// permission stringleri auth.Perm* sabitlerinden geliyor.
func Setup(app *fiber.App, deps Deps) {
	az := deps.Authorizer

	// PUBLIC — kimlik doğrulama gerektirmez
	public := app.Group("/api")
	public.Post("/register", deps.Auth.Register)
	public.Post("/login", deps.Auth.Login)
	public.Post("/logout", deps.Auth.Logout)

	// PROTECTED — Auth middleware'inden geçer.
	protected := app.Group("/api", middleware.Auth(deps.DB, deps.TokenMgr))

	// Author CRUD
	protected.Get("/authors", az.RequirePermission(auth.PermAuthorRead), deps.Author.List)
	protected.Get("/authors/:id", az.RequirePermission(auth.PermAuthorRead), deps.Author.Get)
	protected.Post("/authors", az.RequirePermission(auth.PermAuthorWrite), deps.Author.Create)
	protected.Put("/authors/:id", az.RequirePermission(auth.PermAuthorWrite), deps.Author.Update)
	protected.Delete("/authors/:id", az.RequirePermission(auth.PermAuthorDelete), deps.Author.Delete)

	// Book CRUD
	protected.Get("/books", az.RequirePermission(auth.PermBookRead), deps.Book.List)
	protected.Get("/books/:id", az.RequirePermission(auth.PermBookRead), deps.Book.Get)
	protected.Post("/books", az.RequirePermission(auth.PermBookWrite), deps.Book.Create)
	protected.Put("/books/:id", az.RequirePermission(auth.PermBookWrite), deps.Book.Update)
	protected.Delete("/books/:id", az.RequirePermission(auth.PermBookDelete), deps.Book.Delete)

	// Library CRUD + M2M Book yönetimi
	protected.Get("/libraries", az.RequirePermission(auth.PermLibraryRead), deps.Library.List)
	protected.Get("/libraries/:id", az.RequirePermission(auth.PermLibraryRead), deps.Library.Get)
	protected.Post("/libraries", az.RequirePermission(auth.PermLibraryWrite), deps.Library.Create)
	protected.Put("/libraries/:id", az.RequirePermission(auth.PermLibraryWrite), deps.Library.Update)
	protected.Delete("/libraries/:id", az.RequirePermission(auth.PermLibraryDelete), deps.Library.Delete)
	// Alt-route'lar M2M mutasyon yapıyor → library:write yeterli.
	protected.Post("/libraries/:id/books", az.RequirePermission(auth.PermLibraryWrite), deps.Library.AddBooks)
	protected.Delete("/libraries/:id/books", az.RequirePermission(auth.PermLibraryWrite), deps.Library.RemoveBooks)
}
