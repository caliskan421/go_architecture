package httpx

import (
	"errors"

	"github.com/gofiber/fiber/v3"
)

// Success, başarılı bir cevabı standart zarfla döndürür: { "data": ... }
// Tek bir sözleşme: client her başarılı response'da "data" key'ini bekler.
func Success(c fiber.Ctx, status int, data interface{}) error {
	return c.Status(status).JSON(fiber.Map{"data": data})
}

// Error, herhangi bir hatayı standart hata zarfına çevirir: { "error": {...} }
// AppError gelirse onun Code/HTTPStatus/Message/Details alanları kullanılır;
// başka tip bir error gelirse generic 500'e sarılır (iç detay sızdırılmaz).
//
// Bu fonksiyonu hem central error handler çağırır (handler `return err` ettiğinde)
// hem de handler içinde direkt çağrılabilir. İkincil kullanım nadir; tipik akış:
//
//	return httpx.ErrInvalidCredentials
//
// Fiber bunu central handler'a verir, central handler Error()'ı çağırır.
func Error(c fiber.Ctx, err error) error {
	var appErr *AppError
	ok := errors.As(err, &appErr)
	if !ok {
		// Bilinmeyen hata: generic 500. İçeride orijinal hata tutulur (log için).
		appErr = ErrInternal.WithErr(err)
	}
	return c.Status(appErr.HTTPStatus).JSON(fiber.Map{"error": appErr})
}
