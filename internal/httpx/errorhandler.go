package httpx

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v3"
)

// requestIDLocalsKey, request-id'yi c.Locals'tan okurken kullanılır.
// middleware paketinde de aynı sabit var; import cycle'ı engellemek için
// duplike ettik (httpx, middleware'i import edemez çünkü middleware httpx'e bağımlı).
const requestIDLocalsKey = "request_id"

// Handler, Fiber'in "central error handler" kancasıdır.
// fiber.Config{ErrorHandler: httpx.Handler} ile bağlanır.
// Bir handler `return err` döndürdüğünde Fiber bu fonksiyonu çağırır;
// böylece her handler kendi response biçimini üretmek zorunda kalmaz.
//
// Akış:
//
//	handler -> return apperror.ErrXxx
//	Fiber  -> Handler(c, err)  (bu fonksiyon)
//	Handler -> Error(c, err)   (JSON zarfı)
//
// Faz 8'de log.Printf yerine slog.Default()'a geçildi: structured log + JSON output.
// Request-id de log satırına ekleniyor — bir hata raporu geldiğinde aynı id'li
// log satırını grep'lemek mümkün.
func Handler(c fiber.Ctx, err error) error {
	rid, _ := c.Locals(requestIDLocalsKey).(string)

	if appErr, ok := errors.AsType[*AppError](err); ok {
		// 5xx'leri "error" seviyesinde, 4xx'leri "info" seviyesinde log'la.
		// 4xx (client hataları) operatörü ilgilendirmez ama yine de görünür kalsın
		// — debug/audit için faydalı.
		if appErr.HTTPStatus >= 500 {
			slog.Error("internal error",
				"request_id", rid,
				"code", appErr.Code,
				"path", c.Path(),
				"method", c.Method(),
				"error", appErr.Err,
			)
		} else {
			slog.Info("client error",
				"request_id", rid,
				"code", appErr.Code,
				"status", appErr.HTTPStatus,
				"path", c.Path(),
				"method", c.Method(),
			)
		}
	} else {
		// Bilinmeyen tip — generic 500.
		slog.Error("unhandled error",
			"request_id", rid,
			"path", c.Path(),
			"method", c.Method(),
			"error", err,
		)
	}
	return Error(c, err)
}
