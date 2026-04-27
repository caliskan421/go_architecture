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

// parseID, path'teki :id parametresini pozitif int'e çevirir.
// Fiber v3'te c.ParamsInt yok; c.Params string döner, biz strconv ile parse ediyoruz.
// Geçersiz/0/negatif id -> ErrBadRequest.
func parseID(c fiber.Ctx) (int, error) {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id < 1 {
		return 0, httpx.ErrBadRequest
	}
	return id, nil
}

// AuthorHandler, /api/authors altındaki 5 endpoint'in (List/Get/Create/Update/Delete)
// sahibi. AuthHandler ile aynı DI deseni: bağımlılıklar struct alanı, global state YOK.
// Test'te &AuthorHandler{db: testDB, val: testVal} ile mock'lanır.
type AuthorHandler struct {
	db  *gorm.DB
	val *validator.Validate
}

// NewAuthorHandler, boot'ta bir kez çağrılır.
// AuthHandler'ın aksine error dönmüyor — DB/role lookup gibi başlangıç işi yok,
// sadece referansları struct'a yerleştiriyor.
func NewAuthorHandler(db *gorm.DB, val *validator.Validate) *AuthorHandler {
	return &AuthorHandler{
		db:  db,
		val: val,
	}
}

// authorSortFields, sort=... query'sinde kabul edilecek kolon whitelist'i.
// Whitelist dışında bir değer gelirse ParseSort default'a düşer.
var authorSortFields = []string{"id", "name", "created_at", "updated_at"}

// List, author'ları sayfalı döner. ?q= ile name LIKE araması, ?sort=&order=
// ile sıralama, ?page=&page_size= ile sayfalama kabul eder.
//
//	GET /api/authors?page=1&page_size=20&sort=name&order=asc&q=yaşar
//	-> 200 {"data": httpx.Page[dto.AuthorResponse]}
func (h *AuthorHandler) List(c fiber.Ctx) error {
	pg := httpx.ParsePagination(c)
	sort := httpx.ParseSort(c, authorSortFields, "id")
	q := strings.TrimSpace(c.Query("q"))

	// Tek query builder'ı önce filter+count, sonra paginate+find için yeniden kullanıyoruz.
	// Session(&gorm.Session{}) "fresh statement" oluşturur ki Count ve Find aynı condition'ları
	// alsın ama LIMIT/OFFSET birbirine sızmasın.
	tx := h.db.Model(&model.Author{})
	if q != "" {
		tx = tx.Where("name LIKE ?", "%"+q+"%")
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return httpx.ErrInternal.WithErr(err)
	}

	var authors []model.Author
	err := tx.Order(sort.OrderClause()).Limit(pg.Limit()).Offset(pg.Offset()).Find(&authors).Error
	if err != nil {
		return httpx.ErrInternal.WithErr(err)
	}

	items := make([]dto.AuthorResponse, len(authors))
	for i, a := range authors {
		items[i] = dto.ToAuthorResponse(a)
	}
	return httpx.Success(c, fiber.StatusOK, httpx.NewPage(items, total, pg))
}

// Get, ID'ye göre tek author döner.
//
//	GET /api/authors/:id  ->  200 {"data": dto.AuthorResponse}
func (h *AuthorHandler) Get(c fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}

	var author model.Author
	err = h.db.First(&author, id).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return httpx.ErrNotFound
	case err != nil:
		return httpx.ErrInternal.WithErr(err)
	}
	return httpx.Success(c, fiber.StatusOK, dto.ToAuthorResponse(author))
}

// Create, yeni author oluşturur.
//
//	POST /api/authors  body: dto.CreateAuthorRequest  ->  201 {"data": dto.AuthorResponse}
func (h *AuthorHandler) Create(c fiber.Ctx) error {
	var req dto.CreateAuthorRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.ErrBadRequest.WithErr(err)
	}
	if err := h.val.Struct(&req); err != nil {
		return httpx.ErrValidation.WithDetails(validate.Format(err))
	}

	author := model.Author{Name: req.Name, Description: req.Description}
	err := h.db.Create(&author).Error
	switch {
	// uniqueIndex çakışması (MySQL 1062) -> 409. TranslateError:true sayesinde çalışır.
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return httpx.ErrConflict
	case err != nil:
		return httpx.ErrInternal.WithErr(err)
	}
	return httpx.Success(c, fiber.StatusCreated, dto.ToAuthorResponse(author))
}

// Update, mevcut author'ı günceller (PUT semantiği — full replace).
//
//	PUT /api/authors/:id  body: dto.UpdateAuthorRequest  ->  200 {"data": dto.AuthorResponse}
func (h *AuthorHandler) Update(c fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}

	var req dto.UpdateAuthorRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.ErrBadRequest.WithErr(err)
	}
	if err := h.val.Struct(&req); err != nil {
		return httpx.ErrValidation.WithDetails(validate.Format(err))
	}

	// Önce kaydı çek: yoksa 404 (Save sıfırdan oluşturma yapmasın).
	var author model.Author
	err = h.db.First(&author, id).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return httpx.ErrNotFound
	case err != nil:
		return httpx.ErrInternal.WithErr(err)
	}

	author.Name = req.Name
	author.Description = req.Description

	err = h.db.Save(&author).Error
	switch {
	// Başka bir author'ın Name'iyle çakıştı.
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return httpx.ErrConflict
	case err != nil:
		return httpx.ErrInternal.WithErr(err)
	}
	return httpx.Success(c, fiber.StatusOK, dto.ToAuthorResponse(author))
}

// Delete, author'ı soft-delete eder (gorm.Model.DeletedAt set edilir).
//
//	DELETE /api/authors/:id  ->  204 No Content
func (h *AuthorHandler) Delete(c fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}

	// Var mı kontrolü: idempotent davranış yerine bilinçli olarak 404 dönüyoruz
	// (client "var olmayan id'yi sildim" sanmasın).
	var author model.Author
	err = h.db.First(&author, id).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return httpx.ErrNotFound
	case err != nil:
		return httpx.ErrInternal.WithErr(err)
	}

	if err := h.db.Delete(&author).Error; err != nil {
		return httpx.ErrInternal.WithErr(err)
	}
	// 204'te body yok — httpx.Success yerine SendStatus.
	return c.SendStatus(fiber.StatusNoContent)
}
