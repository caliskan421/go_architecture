package httpx

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// defaultPageSize, query'de page_size verilmediğinde kullanılır.
// maxPageSize, kötü niyetli "?page_size=1000000" ile sunucuyu boğmayı engeller.
const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// Pagination, sayfalama parametrelerini taşır. Limit/Offset hesaplaması bu
// struct'ta toplandı — handler'ların aritmetik yapması gerekmez.
type Pagination struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

// Limit, GORM'un .Limit() çağrısına gider.
func (p Pagination) Limit() int { return p.PageSize }

// Offset, GORM'un .Offset() çağrısına gider. 1-indexed sayfa → 0-indexed offset.
func (p Pagination) Offset() int { return (p.Page - 1) * p.PageSize }

// ParsePagination, query stringinden ?page= ve ?page_size= değerlerini okur.
// Eksik/geçersiz değerler bilinçli olarak fallback'e düşer; 400 dönmek istemiyoruz
// çünkü sayfalama "best effort" — istemci yanlış parametre yollasa bile liste gelmeli.
func ParsePagination(c fiber.Ctx) Pagination {
	page, _ := strconv.Atoi(c.Query("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(c.Query("page_size"))
	if size < 1 {
		size = defaultPageSize
	}
	if size > maxPageSize {
		size = maxPageSize
	}
	return Pagination{Page: page, PageSize: size}
}

// Sort, sıralama parametresini taşır. Field'ı handler'ın whitelist'inden geçer
// (SQL injection koruması) — DB kolon adıyla aynı olmalı.
type Sort struct {
	Field string
	Asc   bool
}

// OrderClause, GORM'un .Order() çağrısına gider: "name ASC" / "id DESC".
func (s Sort) OrderClause() string {
	dir := "ASC"
	if !s.Asc {
		dir = "DESC"
	}
	return s.Field + " " + dir
}

// ParseSort, query'den ?sort= ve ?order= okur. allowed listesinde olmayan alan
// reddedilip default'a düşülür — kullanıcı "sort=password" diye iç alan adı
// göndermesin, schema sızıntısı + injection vektörü olmasın.
func ParseSort(c fiber.Ctx, allowed []string, defaultField string) Sort {
	field := c.Query("sort", defaultField)
	ok := false
	for _, a := range allowed {
		if a == field {
			ok = true
			break
		}
	}
	if !ok {
		field = defaultField
	}
	asc := !strings.EqualFold(c.Query("order", "asc"), "desc")
	return Sort{Field: field, Asc: asc}
}

// Page[T], paginated response için generic zarftır.
// JSON çıkışı: {"items":[...], "total":N, "page":1, "page_size":20, "total_pages":M}
//
// Generic'in faydası: handler'da Items []dto.AuthorResponse olarak bilinir,
// derleyici tip uyumsuzluğunu yakalar; runtime cast yok.
type Page[T any] struct {
	Items      []T   `json:"items"`
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalPages int   `json:"total_pages"`
}

// NewPage, Items + total + Pagination'dan tam Page döndürür.
// total_pages hesabı: ceiling(total / page_size). total=0 ise 0 sayfa.
func NewPage[T any](items []T, total int64, p Pagination) Page[T] {
	if items == nil {
		items = []T{} // JSON'da "null" yerine "[]" garantisi
	}
	totalPages := 0
	if p.PageSize > 0 && total > 0 {
		totalPages = int((total + int64(p.PageSize) - 1) / int64(p.PageSize))
	}
	return Page[T]{
		Items:      items,
		Total:      total,
		Page:       p.Page,
		PageSize:   p.PageSize,
		TotalPages: totalPages,
	}
}
