package httpx

import (
	"errors"
	"log"

	"github.com/gofiber/fiber/v3"
)

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
func Handler(c fiber.Ctx, err error) error {
	// 5xx sınıfı hataları sunucu log'una yaz — operatör görsün diye.
	// 4xx (client hataları) log'lanmaz; aksi halde log spam olur.
	if appErr, ok := errors.AsType[*AppError](err); ok {
		if appErr.HTTPStatus >= 500 && appErr.Err != nil {
			log.Printf("internal error: code=%s underlying=%v", appErr.Code, appErr.Err)
		}
	}
	return Error(c, err)
}
