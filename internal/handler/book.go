package handler

import (
	"errors"
	"strconv"
	"strings"

	"libra_management/internal/dto"
	"libra_management/internal/httpx"
	"libra_management/internal/model"
	"libra_management/pkg/validate"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

// BookHandler, /api/books altındaki 5 endpoint'in (List/Get/Create/Update/Delete)
// sahibi. AuthorHandler ile aynı DI deseni — sadece DB ve validator inject edilir.
type BookHandler struct {
	db  *gorm.DB
	val *validator.Validate
}

func NewBookHandler(db *gorm.DB, val *validator.Validate) *BookHandler {
	return &BookHandler{db: db, val: val}
}

// authorExists, verilen AuthorID'nin DB'de var olup olmadığını kontrol eder.
// SELECT count(*) yerine "SELECT 1 ... LIMIT 1" semantiği için Select+Limit kullanılıyor:
// büyük tablolarda count tarama yapar, biz sadece varlık bilgisi istiyoruz.
func (h *BookHandler) authorExists(id uint) (bool, error) {
	var got uint
	err := h.db.Model(&model.Author{}).Select("id").Where("id = ?", id).Limit(1).Scan(&got).Error
	if err != nil {
		return false, err
	}
	return got != 0, nil
}

// bookSortFields, sort=... whitelist'i.
var bookSortFields = []string{"id", "name", "author_id", "created_at", "updated_at"}

// List, kitapları sayfalı döner. ?q= name araması, ?author_id= ile filtreleme,
// ?sort=&order= ile sıralama, ?page=&page_size= ile sayfalama.
//
//	GET /api/books?author_id=3&q=suç&page=1&page_size=10
//	-> 200 {"data": httpx.Page[dto.BookResponse]}
func (h *BookHandler) List(c fiber.Ctx) error {
	pg := httpx.ParsePagination(c)
	sort := httpx.ParseSort(c, bookSortFields, "id")
	q := strings.TrimSpace(c.Query("q"))

	tx := h.db.Model(&model.Book{})
	if q != "" {
		tx = tx.Where("name LIKE ?", "%"+q+"%")
	}
	// ?author_id= ile parent filtresi — Author detay sayfasındaki kitap listesi için faydalı.
	if rawAID := c.Query("author_id"); rawAID != "" {
		aid, err := strconv.Atoi(rawAID)
		if err == nil && aid > 0 {
			tx = tx.Where("author_id = ?", aid)
		}
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return httpx.ErrInternal.WithErr(err)
	}

	var books []model.Book
	err := tx.Preload("Author").Order(sort.OrderClause()).Limit(pg.Limit()).Offset(pg.Offset()).Find(&books).Error
	if err != nil {
		return httpx.ErrInternal.WithErr(err)
	}

	items := make([]dto.BookResponse, len(books))
	for i, b := range books {
		items[i] = dto.ToBookResponse(b)
	}
	return httpx.Success(c, fiber.StatusOK, httpx.NewPage(items, total, pg))
}

// Get, ID'ye göre tek kitap döner.
//
//	GET /api/books/:id  ->  200 {"data": dto.BookResponse}
func (h *BookHandler) Get(c fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}
	var book model.Book
	err = h.db.Preload("Author").First(&book, id).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return httpx.ErrNotFound
	case err != nil:
		return httpx.ErrInternal.WithErr(err)
	}
	return httpx.Success(c, fiber.StatusOK, dto.ToBookResponse(book))
}

// Create, yeni kitap oluşturur.
//
//	POST /api/books  body: dto.CreateBookRequest  ->  201 {"data": dto.BookResponse}
func (h *BookHandler) Create(c fiber.Ctx) error {
	var req dto.CreateBookRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.ErrBadRequest.WithErr(err)
	}
	if err := h.val.Struct(&req); err != nil {
		return httpx.ErrValidation.WithDetails(validate.Format(err))
	}

	// FK varlık kontrolü: AuthorID DB'de yoksa 422 yerine 400+detail döndürüyoruz —
	// validator'ın "var olmayan AuthorID" alan-bazlı detayıyla aynı sözleşme korunur.
	exists, err := h.authorExists(req.AuthorID)
	if err != nil {
		return httpx.ErrInternal.WithErr(err)
	}
	if !exists {
		return httpx.ErrValidation.WithDetails([]validate.FieldError{{
			Field:   "AuthorID",
			Message: "verilen author_id ile kayıt bulunamadı",
		}})
	}

	book := model.Book{
		AuthorID:    req.AuthorID,
		Name:        req.Name,
		Description: req.Description,
	}
	err = h.db.Create(&book).Error
	switch {
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return httpx.ErrConflict
	case err != nil:
		return httpx.ErrInternal.WithErr(err)
	}

	// Author'ı response'a koyabilmek için tek round-trip Preload.
	// Alternatif: req.AuthorID'yi DB'den ilk lookup'ta çekip cache'lemek; mevcut
	// authorExists count-only sorgu olduğu için Author satırını burada yeniden çekiyoruz.
	if err := h.db.Preload("Author").First(&book, book.ID).Error; err != nil {
		return httpx.ErrInternal.WithErr(err)
	}
	return httpx.Success(c, fiber.StatusCreated, dto.ToBookResponse(book))
}

// Update, mevcut kitabı günceller (PUT semantiği — full replace).
//
//	PUT /api/books/:id  body: dto.UpdateBookRequest  ->  200 {"data": dto.BookResponse}
func (h *BookHandler) Update(c fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}
	var req dto.UpdateBookRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.ErrBadRequest.WithErr(err)
	}
	if err := h.val.Struct(&req); err != nil {
		return httpx.ErrValidation.WithDetails(validate.Format(err))
	}

	var book model.Book
	err = h.db.First(&book, id).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return httpx.ErrNotFound
	case err != nil:
		return httpx.ErrInternal.WithErr(err)
	}

	// AuthorID değişiyorsa yeni id'nin var olduğunu doğrula.
	// Aynı kalıyorsa gereksiz sorgu açmıyoruz.
	if req.AuthorID != book.AuthorID {
		exists, err := h.authorExists(req.AuthorID)
		if err != nil {
			return httpx.ErrInternal.WithErr(err)
		}
		if !exists {
			return httpx.ErrValidation.WithDetails([]validate.FieldError{{
				Field:   "AuthorID",
				Message: "verilen author_id ile kayıt bulunamadı",
			}})
		}
	}

	book.AuthorID = req.AuthorID
	book.Name = req.Name
	book.Description = req.Description

	err = h.db.Save(&book).Error
	switch {
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return httpx.ErrConflict
	case err != nil:
		return httpx.ErrInternal.WithErr(err)
	}

	if err := h.db.Preload("Author").First(&book, book.ID).Error; err != nil {
		return httpx.ErrInternal.WithErr(err)
	}
	return httpx.Success(c, fiber.StatusOK, dto.ToBookResponse(book))
}

// Delete, kitabı hard-delete eder (Author ile aynı tercih).
//
//	DELETE /api/books/:id  ->  204 No Content
func (h *BookHandler) Delete(c fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}

	var book model.Book
	err = h.db.First(&book, id).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return httpx.ErrNotFound
	case err != nil:
		return httpx.ErrInternal.WithErr(err)
	}

	if err := h.db.Delete(&book).Error; err != nil {
		return httpx.ErrInternal.WithErr(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
