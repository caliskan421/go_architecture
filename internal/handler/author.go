package handler

import (
	"errors"
	"strconv"

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

// List, tüm author'ları döner.
//
//	GET /api/authors  ->  200 {"data": []dto.AuthorResponse}
//
// Faz 6'da generic Page[T] ile sayfalanacak; "tümünü dön" şimdilik geçici.
func (h *AuthorHandler) List(c fiber.Ctx) error {
	var authors []model.Author
	if err := h.db.Find(&authors).Error; err != nil {
		return httpx.ErrInternal.WithErr(err)
	}
	// make+len: boş slice "null" yerine "[]" olarak JSON'a düşsün.
	out := make([]dto.AuthorResponse, len(authors))
	for i, a := range authors {
		out[i] = dto.ToAuthorResponse(a)
	}
	return httpx.Success(c, fiber.StatusOK, out)
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
