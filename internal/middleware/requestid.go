package middleware

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// RequestIDHeader, hem incoming hem outgoing request-id taşıyıcısı.
// Standart isim: X-Request-ID (de-facto endüstri konvansiyonu).
const RequestIDHeader = "X-Request-ID"

// RequestIDLocalsKey, c.Locals üzerinden okumak için sabit anahtar.
// Tip-güvenli olmadığı için sabit kullanıyoruz; magic string dağılmasın.
const RequestIDLocalsKey = "request_id"

// RequestID, her isteğe benzersiz bir id atar:
//   - İstemci X-Request-ID gönderdiyse onu kabul eder (distributed tracing için faydalı)
//   - Yoksa UUID v4 üretir
//   - c.Locals'a yazar (handler'lar log için kullanır)
//   - Response header'a yansıtır (istemci kendi log'unda eşleyebilir)
//
// Bu middleware Auth/Authorizer'dan ÖNCE bağlanmalı: 401/403 dönen requestler
// de bir request-id ile log'lansın.
func RequestID() fiber.Handler {
	return func(c fiber.Ctx) error {
		id := c.Get(RequestIDHeader)
		if id == "" {
			id = uuid.NewString()
		}
		c.Locals(RequestIDLocalsKey, id)
		c.Set(RequestIDHeader, id)
		return c.Next()
	}
}

// GetRequestID, c.Locals'tan request-id'yi okur. Set edilmemişse boş string döner;
// caller fallback'ini kendisi seçer (örn. "unknown").
func GetRequestID(c fiber.Ctx) string {
	if v, ok := c.Locals(RequestIDLocalsKey).(string); ok {
		return v
	}
	return ""
}
