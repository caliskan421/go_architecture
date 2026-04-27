package handler

import (
	"errors"
	"strings"

	"libra_management/internal/dto"
	"libra_management/internal/httpx"
	"libra_management/internal/model"
	"libra_management/pkg/validate"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

// LibraryHandler, /api/libraries altındaki endpoint'lerin sahibi.
// CRUD + M2M Book yönetimi (AddBooks/RemoveBooks alt-route'ları).
type LibraryHandler struct {
	db  *gorm.DB
	val *validator.Validate
}

func NewLibraryHandler(db *gorm.DB, val *validator.Validate) *LibraryHandler {
	return &LibraryHandler{db: db, val: val}
}

// fetchBooksByIDs, verilen id listesindeki TÜM kitapların var olduğunu doğrular
// ve modelleri döndürür. Eksik id varsa "missing" listesini de döndürür ki
// caller validation detail olarak kullanıcıya gösterebilsin.
//
// Tek SQL: WHERE id IN (?). N+1 yapmıyoruz.
func (h *LibraryHandler) fetchBooksByIDs(ids []uint) (books []model.Book, missing []uint, err error) {
	if len(ids) == 0 {
		return nil, nil, nil
	}
	if err := h.db.Where("id IN ?", ids).Find(&books).Error; err != nil {
		return nil, nil, err
	}
	if len(books) == len(ids) {
		return books, nil, nil
	}
	// Eksikleri çıkarmak için set kur, sonra fark al.
	got := make(map[uint]struct{}, len(books))
	for _, b := range books {
		got[b.ID] = struct{}{}
	}
	for _, id := range ids {
		if _, ok := got[id]; !ok {
			missing = append(missing, id)
		}
	}
	return books, missing, nil
}

// dedupeUints, slice'tan duplicate'leri çıkarır — sıralamayı korur.
// Client {"book_ids":[1,1,2]} gönderirse M2M çift insert hatası almak yerine
// önceden tek hale getiriyoruz.
func dedupeUints(in []uint) []uint {
	seen := make(map[uint]struct{}, len(in))
	out := make([]uint, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// loadLibraryWithBooks, library'yi nested Book.Author Preload'larıyla çeker.
// Yanıt zarflarken aynı sorgu birden fazla yerde tekrarlanmasın diye helper.
func (h *LibraryHandler) loadLibraryWithBooks(id uint) (*model.Library, error) {
	var lib model.Library
	err := h.db.Preload("Books.Author").First(&lib, id).Error
	if err != nil {
		return nil, err
	}
	return &lib, nil
}

// librarySortFields, sort=... whitelist'i. Books M2M olduğu için sıralanabilir alan değil.
var librarySortFields = []string{"id", "name", "created_at", "updated_at"}

// List, kütüphaneleri sayfalı döner. Books + Books.Author her kayıt için Preload.
//
//	GET /api/libraries?page=1&page_size=10&sort=name&q=ank
//	-> 200 {"data": httpx.Page[dto.LibraryResponse]}
//
// NOT: Preload sayfalanan parent'a uygulanır — Books'un kendisi sayfalanmaz
// (kütüphane başına tüm kitaplar gelir). İleride ihtiyaç olursa Get/:id altında
// ayrı bir books endpoint'i açılabilir.
func (h *LibraryHandler) List(c fiber.Ctx) error {
	pg := httpx.ParsePagination(c)
	sort := httpx.ParseSort(c, librarySortFields, "id")
	q := strings.TrimSpace(c.Query("q"))

	tx := h.db.Model(&model.Library{})
	if q != "" {
		tx = tx.Where("name LIKE ?", "%"+q+"%")
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return httpx.ErrInternal.WithErr(err)
	}

	var libs []model.Library
	err := tx.Preload("Books.Author").
		Order(sort.OrderClause()).Limit(pg.Limit()).Offset(pg.Offset()).
		Find(&libs).Error
	if err != nil {
		return httpx.ErrInternal.WithErr(err)
	}

	items := make([]dto.LibraryResponse, len(libs))
	for i, l := range libs {
		items[i] = dto.ToLibraryResponse(l)
	}
	return httpx.Success(c, fiber.StatusOK, httpx.NewPage(items, total, pg))
}

// Get, ID'ye göre tek kütüphane döner.
//
//	GET /api/libraries/:id  ->  200 {"data": dto.LibraryResponse}
func (h *LibraryHandler) Get(c fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}
	lib, err := h.loadLibraryWithBooks(uint(id))
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return httpx.ErrNotFound
	case err != nil:
		return httpx.ErrInternal.WithErr(err)
	}
	return httpx.Success(c, fiber.StatusOK, dto.ToLibraryResponse(*lib))
}

// Create, yeni kütüphane oluşturur. Verilen BookIDs varsa M2M ilişkiyi kurar.
//
//	POST /api/libraries  body: dto.CreateLibraryRequest  ->  201 {"data": dto.LibraryResponse}
func (h *LibraryHandler) Create(c fiber.Ctx) error {
	var req dto.CreateLibraryRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.ErrBadRequest.WithErr(err)
	}
	if err := h.val.Struct(&req); err != nil {
		return httpx.ErrValidation.WithDetails(validate.Format(err))
	}

	ids := dedupeUints(req.BookIDs)

	// Tüm Book'ların DB'de var olduğunu önceden teyit ediyoruz —
	// transaction içine girmeden hızlı reddetmek için.
	books, missing, err := h.fetchBooksByIDs(ids)
	if err != nil {
		return httpx.ErrInternal.WithErr(err)
	}
	if len(missing) > 0 {
		return httpx.ErrValidation.WithDetails([]validate.FieldError{{
			Field:   "BookIDs",
			Message: "verilen book_ids listesinde DB'de bulunmayan id var",
		}})
	}

	lib := model.Library{Name: req.Name, Books: books}

	// Transaction: Library Create + library_books insert atomic olsun.
	// `db.Transaction` callback hata dönerse otomatik rollback yapar.
	err = h.db.Transaction(func(tx *gorm.DB) error {
		// Books slice'ı yukarıda dolduruldu; Create otomatik olarak join tablosuna
		// satır eklemez — biz "FullSaveAssociations" yerine Replace ile bağlıyoruz.
		// Önce parent kaydı oluştur:
		if err := tx.Omit("Books").Create(&lib).Error; err != nil {
			return err
		}
		// Sonra Association().Replace ile join tablosunu doldur.
		// `Replace([]Book)` aslında "set" semantiği: yeni library boş olduğu için
		// burada Append ile aynıdır.
		if len(books) > 0 {
			if err := tx.Model(&lib).Association("Books").Replace(books); err != nil {
				return err
			}
		}
		return nil
	})
	switch {
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return httpx.ErrConflict
	case err != nil:
		return httpx.ErrInternal.WithErr(err)
	}

	// Response için tam state'i Preload'la geri çek.
	full, err := h.loadLibraryWithBooks(lib.ID)
	if err != nil {
		return httpx.ErrInternal.WithErr(err)
	}
	return httpx.Success(c, fiber.StatusCreated, dto.ToLibraryResponse(*full))
}

// Update, library adını ve (req.BookIDs verilmişse) M2M setini değiştirir.
//
//	PUT /api/libraries/:id  body: dto.UpdateLibraryRequest  ->  200
//
// BookIDs nil → ilişkiye dokunma; non-nil ([] dahil) → tam Replace.
func (h *LibraryHandler) Update(c fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}
	var req dto.UpdateLibraryRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.ErrBadRequest.WithErr(err)
	}
	if err := h.val.Struct(&req); err != nil {
		return httpx.ErrValidation.WithDetails(validate.Format(err))
	}

	var lib model.Library
	err = h.db.First(&lib, id).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return httpx.ErrNotFound
	case err != nil:
		return httpx.ErrInternal.WithErr(err)
	}

	// BookIDs verilmişse FK kontrolü.
	var booksToSet []model.Book
	if req.BookIDs != nil {
		ids := dedupeUints(*req.BookIDs)
		books, missing, err := h.fetchBooksByIDs(ids)
		if err != nil {
			return httpx.ErrInternal.WithErr(err)
		}
		if len(missing) > 0 {
			return httpx.ErrValidation.WithDetails([]validate.FieldError{{
				Field:   "BookIDs",
				Message: "verilen book_ids listesinde DB'de bulunmayan id var",
			}})
		}
		booksToSet = books
	}

	err = h.db.Transaction(func(tx *gorm.DB) error {
		lib.Name = req.Name
		// Updates yerine Save kullanıyoruz: tüm alanlar yazılsın (PUT semantiği).
		// Books'u dışarıda yönettiğimiz için Omit ediyoruz.
		if err := tx.Omit("Books").Save(&lib).Error; err != nil {
			return err
		}
		if req.BookIDs != nil {
			// Replace: mevcut bağı tamamen silip yeniyi kur.
			// Boş slice gelirse library_books'tan tüm satırlar silinir.
			if err := tx.Model(&lib).Association("Books").Replace(booksToSet); err != nil {
				return err
			}
		}
		return nil
	})
	switch {
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return httpx.ErrConflict
	case err != nil:
		return httpx.ErrInternal.WithErr(err)
	}

	full, err := h.loadLibraryWithBooks(lib.ID)
	if err != nil {
		return httpx.ErrInternal.WithErr(err)
	}
	return httpx.Success(c, fiber.StatusOK, dto.ToLibraryResponse(*full))
}

// Delete, library'yi siler. M2M satırları gorm "cascade-on-delete" davranışına
// bağlı değildir — Association().Clear() ile join tablosunu da temizliyoruz.
//
//	DELETE /api/libraries/:id  ->  204
func (h *LibraryHandler) Delete(c fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}

	var lib model.Library
	err = h.db.First(&lib, id).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return httpx.ErrNotFound
	case err != nil:
		return httpx.ErrInternal.WithErr(err)
	}

	err = h.db.Transaction(func(tx *gorm.DB) error {
		// Önce M2M kayıtlarını temizle ki orphan join row kalmasın.
		if err := tx.Model(&lib).Association("Books").Clear(); err != nil {
			return err
		}
		return tx.Delete(&lib).Error
	})
	if err != nil {
		return httpx.ErrInternal.WithErr(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// AddBooks, mevcut kütüphaneye verilen kitapları ekler (Append semantiği).
//
//	POST /api/libraries/:id/books  body: {"book_ids":[1,2]}  ->  200 {"data": dto.LibraryResponse}
//
// Mevcut bir id tekrar gelirse gorm join'i tekrar yazar — primary key ihlali
// olmasın diye önce dedupe + "şu an bağlı olmayanları" filtreliyoruz.
func (h *LibraryHandler) AddBooks(c fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}
	var req dto.LibraryBookIDsRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.ErrBadRequest.WithErr(err)
	}
	if err := h.val.Struct(&req); err != nil {
		return httpx.ErrValidation.WithDetails(validate.Format(err))
	}

	var lib model.Library
	err = h.db.Preload("Books").First(&lib, id).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return httpx.ErrNotFound
	case err != nil:
		return httpx.ErrInternal.WithErr(err)
	}

	wantIDs := dedupeUints(req.BookIDs)

	// Already-linked filtresi.
	linked := make(map[uint]struct{}, len(lib.Books))
	for _, b := range lib.Books {
		linked[b.ID] = struct{}{}
	}
	addIDs := make([]uint, 0, len(wantIDs))
	for _, id := range wantIDs {
		if _, ok := linked[id]; ok {
			continue
		}
		addIDs = append(addIDs, id)
	}

	if len(addIDs) > 0 {
		books, missing, err := h.fetchBooksByIDs(addIDs)
		if err != nil {
			return httpx.ErrInternal.WithErr(err)
		}
		if len(missing) > 0 {
			return httpx.ErrValidation.WithDetails([]validate.FieldError{{
				Field:   "BookIDs",
				Message: "verilen book_ids listesinde DB'de bulunmayan id var",
			}})
		}
		if err := h.db.Model(&lib).Association("Books").Append(books); err != nil {
			return httpx.ErrInternal.WithErr(err)
		}
	}

	full, err := h.loadLibraryWithBooks(lib.ID)
	if err != nil {
		return httpx.ErrInternal.WithErr(err)
	}
	return httpx.Success(c, fiber.StatusOK, dto.ToLibraryResponse(*full))
}

// RemoveBooks, mevcut kütüphaneden verilen kitapları çıkarır (Delete semantiği).
//
//	DELETE /api/libraries/:id/books  body: {"book_ids":[1]}  ->  200 {"data": dto.LibraryResponse}
//
// Bağlı olmayan id sessizce yok sayılır — idempotent.
func (h *LibraryHandler) RemoveBooks(c fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}
	var req dto.LibraryBookIDsRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.ErrBadRequest.WithErr(err)
	}
	if err := h.val.Struct(&req); err != nil {
		return httpx.ErrValidation.WithDetails(validate.Format(err))
	}

	var lib model.Library
	err = h.db.First(&lib, id).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return httpx.ErrNotFound
	case err != nil:
		return httpx.ErrInternal.WithErr(err)
	}

	ids := dedupeUints(req.BookIDs)
	// Burada FK varlık kontrolü yapmıyoruz: silmek için Book'un DB'de mutlaka
	// olması gerekmez (zaten silinmiş olabilir). Association.Delete sadece
	// join tablosundan satırı kaldırır, parent Book'a dokunmaz.
	dummies := make([]model.Book, len(ids))
	for i, bid := range ids {
		dummies[i] = model.Book{ID: bid}
	}
	if len(dummies) > 0 {
		if err := h.db.Model(&lib).Association("Books").Delete(dummies); err != nil {
			return httpx.ErrInternal.WithErr(err)
		}
	}

	full, err := h.loadLibraryWithBooks(lib.ID)
	if err != nil {
		return httpx.ErrInternal.WithErr(err)
	}
	return httpx.Success(c, fiber.StatusOK, dto.ToLibraryResponse(*full))
}
