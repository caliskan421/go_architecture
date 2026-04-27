package validate

import (
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"
)

// New, uygulama yaşam süresince paylaşılacak *validator.Validate instance'ı döner.
// Validator instance'ı thread-safe ve struct meta'sını cache'ler — her istekte
// yeniden oluşturmak verimsiz olur. Bu yüzden boot'ta bir kez kurulup paylaşılır.
func New() *validator.Validate {
	return validator.New()
}

// FieldError, kullanıcıya geri dönecek tek bir doğrulama hatasını temsil eder.
// validator.FieldError kütüphane tipidir; biz onu kendi JSON sözleşmemize çeviriyoruz
// ki frontend predictable bir yapı görsün.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Format, validator'ın döndüğü hatayı kullanıcı-dostu []FieldError'a çevirir.
// errors.As: hata zincirinde belirli bir tipte değer arar; bulursa target'a atar.
// Validator hataları her zaman validator.ValidationErrors tipindedir (bir slice).
func Format(err error) []FieldError {
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		return nil // validator hatası değilse boş dön — caller bunu generic 500'e çevirir
	}

	out := make([]FieldError, 0, len(verrs))
	for _, e := range verrs {
		out = append(out, FieldError{
			Field:   e.Field(), // struct alan adı (örn. "Email")
			Message: messageFor(e),
		})
	}
	return out
}

// messageFor, tek bir validator kuralı için Türkçe açıklama üretir.
// e.Tag() kuralın adı ("required", "email", "min", "eqfield").
// e.Param() kuralın parametresi ("min=8" -> "8"; "eqfield=Password" -> "Password").
func messageFor(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return "alan zorunludur"
	case "email":
		return "geçerli bir e-posta giriniz"
	case "min":
		return fmt.Sprintf("en az %s karakter olmalıdır", e.Param())
	case "max":
		return fmt.Sprintf("en fazla %s karakter olmalıdır", e.Param())
	case "eqfield":
		return fmt.Sprintf("%s ile eşleşmelidir", e.Param())
	default:
		return e.Tag() + " kuralı sağlanmıyor"
	}
}
