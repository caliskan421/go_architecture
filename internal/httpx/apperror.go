package httpx

import (
	"errors"
	"net/http"
)

// AppError, uygulamanın iç hata tipidir. Bir hatanın *anlamını* (Code), client'a
// gösterilecek mesajı (Message), HTTP karşılığını (HTTPStatus), opsiyonel detayı
// (Details — örn. validation alan listesi) ve sarmalanan iç hatayı (Err — log için)
// tek yerde toplar.
//
// Handler'lar artık inline fiber.Map oluşturmuyor; "return apperror" diyor ve
// central error handler bunu JSON'a çeviriyor. Tek doğruluk noktası.
type AppError struct {
	Code       string      `json:"code"`              // makinenin okuyacağı sabit kod ("invalid_credentials")
	HTTPStatus int         `json:"-"`                 // JSON'a çıkmaz; HTTP layer kullanır
	Message    string      `json:"message"`           // insan okur, client gösterir
	Details    interface{} `json:"details,omitempty"` // opsiyonel — örn. validation field listesi
	Err        error       `json:"-"`                 // sarmalanan iç hata; sadece sunucu log'una gider
}

// Error, error interface'ini implement eder — AppError artık bir "error".
// Sarmalı hata varsa onu da göster; log için faydalı.
func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

// Unwrap, errors.Unwrap için: errors.Is/As iç hata zincirine inebilsin diye.
func (e *AppError) Unwrap() error { return e.Err }

// Is, errors.Is karşılaştırması için: aynı Code'a sahip iki AppError eşit sayılır.
// Pointer eşitliği şart değil — WithErr/WithDetails ile kopya alınmış olsa bile
// errors.Is(err, ErrInvalidCredentials) doğru çalışır.
func (e *AppError) Is(target error) bool {
	var t *AppError
	ok := errors.As(target, &t)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// WithDetails, sentinel'i mutate ETMEDEN detay eklenmiş yeni bir kopya döner.
// Sentinel'lerin global olarak değişmemesi için bu deyim önemli.
func (e *AppError) WithDetails(d interface{}) *AppError {
	cp := *e
	cp.Details = d
	return &cp
}

// WithErr, log için sarmalanmış iç hatayı eklenmiş yeni bir kopya döner.
// Client bu iç hatayı görmez (Err alanı `json:"-"`).
func (e *AppError) WithErr(err error) *AppError {
	cp := *e
	cp.Err = err
	return &cp
}

// --- Sentinel'ler (paket-level değişken) ---
// Convention: bu instance'lar mutate EDİLMEZ. Detay/iç hata eklemek için
// WithDetails/WithErr ile kopya al. Sentinel'lerin değişmezliği errors.Is
// karşılaştırmasının güvenliği için kritik.
var (
	ErrInvalidCredentials = &AppError{
		Code: "invalid_credentials", HTTPStatus: http.StatusUnauthorized,
		Message: "e-posta veya şifre hatalı",
	}
	ErrValidation = &AppError{
		Code: "validation_error", HTTPStatus: http.StatusBadRequest,
		Message: "girdi doğrulanamadı",
	}
	ErrBadRequest = &AppError{
		Code: "bad_request", HTTPStatus: http.StatusBadRequest,
		Message: "geçersiz istek",
	}
	ErrUnauthorized = &AppError{
		Code: "unauthorized", HTTPStatus: http.StatusUnauthorized,
		Message: "yetkisiz",
	}
	ErrForbidden = &AppError{
		Code: "forbidden", HTTPStatus: http.StatusForbidden,
		Message: "bu işlem için yetkiniz yok",
	}
	ErrNotFound = &AppError{
		Code: "not_found", HTTPStatus: http.StatusNotFound,
		Message: "kayıt bulunamadı",
	}
	ErrConflict = &AppError{
		Code: "conflict", HTTPStatus: http.StatusConflict,
		Message: "Kaynak mevcut bir kayıtla çakışıyor.",
	}
	ErrInternal = &AppError{
		Code: "internal_error", HTTPStatus: http.StatusInternalServerError,
		Message: "iç sunucu hatası",
	}
)
