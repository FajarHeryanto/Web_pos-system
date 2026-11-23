package utils

import (
	"math"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"pos-system/go/models"
)

var (
	idPattern       = regexp.MustCompile(`^[A-Za-z0-9_-]{1,10}$`)
	usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{3,50}$`)
	datePattern     = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

const (
	maxPrice    = 9999999999.99 // batas NUMERIC(12,2)
	maxStock    = 1000000000
	maxQuantity = 100000
	maxCartItem = 100
)

// IsValidID memeriksa format ID (maks. 10 karakter alfanumerik, '_' atau '-').
func IsValidID(id string) bool {
	return idPattern.MatchString(id)
}

// IsValidDate memeriksa tanggal berformat YYYY-MM-DD dan benar-benar ada di kalender.
func IsValidDate(value string) bool {
	if !datePattern.MatchString(value) {
		return false
	}
	_, err := time.Parse("2006-01-02", value)
	return err == nil
}

// NormalizePayment mengembalikan penulisan baku metode pembayaran.
func NormalizePayment(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "cash":
		return "Cash", true
	case "debit":
		return "Debit", true
	case "credit":
		return "Credit", true
	}
	return "", false
}

// Round2 membulatkan nilai uang ke 2 desimal.
func Round2(value float64) float64 {
	return math.Round(value*100) / 100
}

func hasControlChar(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func validateText(field, label, value string, maxLen int) *FieldError {
	if value == "" {
		return &FieldError{Field: field, Message: label + " wajib diisi"}
	}
	if !utf8.ValidString(value) || hasControlChar(value) {
		return &FieldError{Field: field, Message: label + " mengandung karakter yang tidak diizinkan"}
	}
	if utf8.RuneCountInString(value) > maxLen {
		return &FieldError{Field: field, Message: label + " terlalu panjang"}
	}
	return nil
}

// ValidateLogin memeriksa input login. Input di-trim di tempat.
func ValidateLogin(in *models.LoginInput) []FieldError {
	var errs []FieldError
	in.Username = strings.TrimSpace(in.Username)
	if in.Username == "" {
		errs = append(errs, FieldError{Field: "username", Message: "Username wajib diisi"})
	} else if !usernamePattern.MatchString(in.Username) {
		errs = append(errs, FieldError{Field: "username", Message: "Username hanya boleh huruf, angka, titik, '_' atau '-' (3-50 karakter)"})
	}
	if in.Password == "" {
		errs = append(errs, FieldError{Field: "password", Message: "Password wajib diisi"})
	} else if len(in.Password) > 72 {
		errs = append(errs, FieldError{Field: "password", Message: "Password maksimal 72 karakter"})
	}
	return errs
}

// ValidateProduct memeriksa input produk. Input dinormalkan di tempat.
// Jika idRequired false, product_id boleh kosong (akan dibuat otomatis).
func ValidateProduct(in *models.ProductInput, idRequired bool) []FieldError {
	var errs []FieldError

	in.ProductID = strings.ToUpper(strings.TrimSpace(in.ProductID))
	in.Name = strings.TrimSpace(in.Name)
	in.Category = strings.TrimSpace(in.Category)

	if in.ProductID == "" {
		if idRequired {
			errs = append(errs, FieldError{Field: "product_id", Message: "ID produk wajib diisi"})
		}
	} else if !IsValidID(in.ProductID) {
		errs = append(errs, FieldError{Field: "product_id", Message: "ID produk maksimal 10 karakter (huruf, angka, '_' atau '-')"})
	}

	if fe := validateText("name", "Nama produk", in.Name, 100); fe != nil {
		errs = append(errs, *fe)
	}
	if fe := validateText("category", "Kategori", in.Category, 50); fe != nil {
		errs = append(errs, *fe)
	}

	if in.Price == nil {
		errs = append(errs, FieldError{Field: "price", Message: "Harga wajib diisi"})
	} else if math.IsNaN(*in.Price) || math.IsInf(*in.Price, 0) || *in.Price <= 0 {
		errs = append(errs, FieldError{Field: "price", Message: "Harga harus lebih dari 0"})
	} else if *in.Price > maxPrice {
		errs = append(errs, FieldError{Field: "price", Message: "Harga terlalu besar"})
	} else {
		rounded := Round2(*in.Price)
		if rounded <= 0 {
			errs = append(errs, FieldError{Field: "price", Message: "Harga harus lebih dari 0"})
		} else {
			in.Price = &rounded
		}
	}

	if in.Stock == nil {
		errs = append(errs, FieldError{Field: "stock", Message: "Stok wajib diisi"})
	} else if *in.Stock < 0 {
		errs = append(errs, FieldError{Field: "stock", Message: "Stok tidak boleh negatif"})
	} else if *in.Stock > maxStock {
		errs = append(errs, FieldError{Field: "stock", Message: "Stok terlalu besar"})
	}

	return errs
}

// ValidateTransaction memeriksa input transaksi dan menormalkannya di tempat:
// bentuk satu item diubah menjadi items, metode bayar dibakukan.
func ValidateTransaction(in *models.TransactionInput) []FieldError {
	var errs []FieldError

	if len(in.Items) == 0 && strings.TrimSpace(in.ProductID) != "" {
		in.Items = []models.TransactionItemInput{{
			ProductID:       in.ProductID,
			Quantity:        in.Quantity,
			DiscountPercent: in.DiscountPercent,
		}}
	}

	if len(in.Items) == 0 {
		errs = append(errs, FieldError{Field: "items", Message: "Keranjang masih kosong"})
	} else if len(in.Items) > maxCartItem {
		errs = append(errs, FieldError{Field: "items", Message: "Jumlah item keranjang maksimal 100"})
	}

	seen := make(map[string]bool)
	for i := range in.Items {
		item := &in.Items[i]
		item.ProductID = strings.ToUpper(strings.TrimSpace(item.ProductID))
		prefix := "items[" + itoa(i) + "]."

		if !IsValidID(item.ProductID) {
			errs = append(errs, FieldError{Field: prefix + "product_id", Message: "ID produk tidak valid"})
		} else if seen[item.ProductID] {
			errs = append(errs, FieldError{Field: prefix + "product_id", Message: "Produk yang sama muncul lebih dari sekali"})
		} else {
			seen[item.ProductID] = true
		}

		if item.Quantity <= 0 {
			errs = append(errs, FieldError{Field: prefix + "quantity", Message: "Jumlah harus lebih dari 0"})
		} else if item.Quantity > maxQuantity {
			errs = append(errs, FieldError{Field: prefix + "quantity", Message: "Jumlah terlalu besar"})
		}

		if math.IsNaN(item.DiscountPercent) || item.DiscountPercent < 0 || item.DiscountPercent > 100 {
			errs = append(errs, FieldError{Field: prefix + "discount_percent", Message: "Diskon harus di antara 0 dan 100"})
		} else {
			item.DiscountPercent = Round2(item.DiscountPercent)
		}
	}

	if method, ok := NormalizePayment(in.PaymentMethod); ok {
		in.PaymentMethod = method
	} else {
		errs = append(errs, FieldError{Field: "payment_method", Message: "Metode pembayaran harus Cash, Debit, atau Credit"})
	}

	in.TransactionDate = strings.TrimSpace(in.TransactionDate)
	if in.TransactionDate != "" && !IsValidDate(in.TransactionDate) {
		errs = append(errs, FieldError{Field: "transaction_date", Message: "Tanggal harus berformat YYYY-MM-DD"})
	}

	return errs
}

// itoa: konversi int kecil non-negatif ke string tanpa impor tambahan.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
