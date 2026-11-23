package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"pos-system/go/database"
	"pos-system/go/models"
	"pos-system/go/utils"
)

// ListTransactions mengambil riwayat transaksi terbaru.
// ?limit=1..500 (default 200), ?date=YYYY-MM-DD, ?search=ID/nama produk/kasir.
func ListTransactions(c *gin.Context) {
	limit := 200
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 500 {
			utils.Error(c, http.StatusBadRequest, "Batas transaksi tidak valid", []utils.FieldError{{Field: "limit", Message: "Pilih angka dari 1 sampai 500"}})
			return
		}
		limit = parsed
	}
	date := strings.TrimSpace(c.Query("date"))
	if date != "" && !utils.IsValidDate(date) {
		utils.Error(c, http.StatusBadRequest, "Tanggal tidak valid", []utils.FieldError{{Field: "date", Message: "Tanggal harus berformat YYYY-MM-DD"}})
		return
	}
	search := strings.TrimSpace(c.Query("search"))
	if len(search) > 100 {
		utils.Error(c, http.StatusBadRequest, "Pencarian terlalu panjang", nil)
		return
	}

	rows, err := database.DB().QueryContext(c.Request.Context(), `
		SELECT transaction_id, product_id, product_name, quantity,
		       unit_price::float8, COALESCE(discount_percent,0)::float8,
		       total_price::float8, payment_method, transaction_date, cashier_name
		FROM transactions
		WHERE ($1 = '' OR transaction_date = $1)
		  AND ($2 = '' OR transaction_id ILIKE '%'||$2||'%' OR product_name ILIKE '%'||$2||'%' OR cashier_name ILIKE '%'||$2||'%')
		ORDER BY transaction_date DESC, LENGTH(transaction_id) DESC, transaction_id DESC
		LIMIT $3`, date, search, limit)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Gagal mengambil riwayat transaksi", nil)
		return
	}
	defer rows.Close()

	items := make([]models.Transaction, 0)
	for rows.Next() {
		var t models.Transaction
		if err := rows.Scan(&t.TransactionID, &t.ProductID, &t.ProductName, &t.Quantity,
			&t.UnitPrice, &t.DiscountPercent, &t.TotalPrice, &t.PaymentMethod,
			&t.TransactionDate, &t.CashierName); err != nil {
			utils.Error(c, http.StatusInternalServerError, "Gagal membaca data transaksi", nil)
			return
		}
		items = append(items, t)
	}
	if err := rows.Err(); err != nil {
		utils.Error(c, http.StatusInternalServerError, "Gagal membaca riwayat transaksi", nil)
		return
	}
	utils.Success(c, http.StatusOK, "Riwayat transaksi berhasil diambil", items)
}

// newTransactionID membuat ID acak yang tetap memenuhi batas VARCHAR(10).
func newTransactionID() (string, error) {
	var raw [4]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "T" + strings.ToUpper(hex.EncodeToString(raw[:])), nil
}

// CreateTransaction menyimpan seluruh item keranjang dan mengurangi stok
// dalam satu transaksi PostgreSQL. Jika salah satu item gagal, semua dibatalkan.
func CreateTransaction(c *gin.Context) {
	var input models.TransactionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.Error(c, http.StatusBadRequest, "Format JSON tidak valid", []utils.FieldError{{Field: "body", Message: "Periksa kembali data transaksi"}})
		return
	}
	if errs := utils.ValidateTransaction(&input); len(errs) > 0 {
		utils.Error(c, http.StatusBadRequest, "Data transaksi tidak valid", errs)
		return
	}

	cashier, _ := c.Get("full_name")
	cashierName, _ := cashier.(string)
	if cashierName == "" || len(cashierName) > 100 {
		utils.Error(c, http.StatusUnauthorized, "Informasi kasir tidak tersedia", nil)
		return
	}
	date := input.TransactionDate
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	tx, err := database.DB().BeginTx(c.Request.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Tidak dapat memulai transaksi penjualan", nil)
		return
	}
	defer tx.Rollback()

	created := make([]models.Transaction, 0, len(input.Items))
	for _, item := range input.Items {
		var product models.Product
		err := tx.QueryRowContext(c.Request.Context(), `
			SELECT product_id, name, category, price::float8, stock
			FROM products WHERE product_id=$1 FOR UPDATE`, item.ProductID).
			Scan(&product.ProductID, &product.Name, &product.Category, &product.Price, &product.Stock)
		if errors.Is(err, sql.ErrNoRows) {
			utils.Error(c, http.StatusNotFound, "Produk "+item.ProductID+" tidak ditemukan; tidak ada transaksi yang disimpan", nil)
			return
		}
		if err != nil {
			utils.Error(c, http.StatusInternalServerError, "Gagal membaca stok produk", nil)
			return
		}
		if product.Stock < item.Quantity {
			utils.Error(c, http.StatusConflict, fmt.Sprintf("Stok %s tidak cukup (tersedia %d)", product.Name, product.Stock),
				[]utils.FieldError{{Field: "items", Message: "Kurangi jumlah produk atau perbarui stok"}})
			return
		}

		subtotal := utils.Round2(product.Price * float64(item.Quantity))
		total := utils.Round2(subtotal * (1 - item.DiscountPercent/100))
		id, err := newTransactionID()
		if err != nil {
			utils.Error(c, http.StatusInternalServerError, "Gagal membuat nomor transaksi", nil)
			return
		}

		_, err = tx.ExecContext(c.Request.Context(), `
			INSERT INTO transactions
			(transaction_id, product_id, product_name, quantity, unit_price,
			 discount_percent, total_price, payment_method, transaction_date, cashier_name)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			id, product.ProductID, product.Name, item.Quantity, product.Price,
			item.DiscountPercent, total, input.PaymentMethod, date, cashierName)
		if err != nil {
			utils.Error(c, http.StatusInternalServerError, "Gagal menyimpan transaksi; stok belum diubah", nil)
			return
		}
		result, err := tx.ExecContext(c.Request.Context(), `
			UPDATE products SET stock=stock-$1 WHERE product_id=$2 AND stock >= $1`, item.Quantity, item.ProductID)
		if err != nil {
			utils.Error(c, http.StatusInternalServerError, "Gagal memperbarui stok; transaksi dibatalkan", nil)
			return
		}
		changed, _ := result.RowsAffected()
		if changed != 1 {
			utils.Error(c, http.StatusConflict, "Stok berubah saat transaksi diproses; silakan muat ulang", nil)
			return
		}
		created = append(created, models.Transaction{
			TransactionID: id, ProductID: product.ProductID, ProductName: product.Name,
			Quantity: item.Quantity, UnitPrice: product.Price, DiscountPercent: item.DiscountPercent,
			TotalPrice: total, PaymentMethod: input.PaymentMethod, TransactionDate: date, CashierName: cashierName,
		})
	}

	if err := tx.Commit(); err != nil {
		utils.Error(c, http.StatusInternalServerError, "Transaksi belum dapat dikonfirmasi; muat ulang riwayat sebelum mencoba lagi", nil)
		return
	}
	utils.Success(c, http.StatusCreated, "Transaksi berhasil disimpan", gin.H{
		"transactions": created,
		"item_count":   len(created),
		"total":        totalOf(created),
	})
}

func totalOf(items []models.Transaction) float64 {
	var total float64
	for _, item := range items {
		total += item.TotalPrice
	}
	return utils.Round2(total)
}

// DeleteTransaction menghapus satu baris transaksi dan mengembalikan stok atomik.
func DeleteTransaction(c *gin.Context) {
	id := strings.ToUpper(strings.TrimSpace(c.Param("id")))
	if !utils.IsValidID(id) {
		utils.Error(c, http.StatusBadRequest, "ID transaksi tidak valid", []utils.FieldError{{Field: "id", Message: "ID transaksi maksimal 10 karakter"}})
		return
	}
	tx, err := database.DB().BeginTx(c.Request.Context(), nil)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Tidak dapat memulai penghapusan transaksi", nil)
		return
	}
	defer tx.Rollback()

	var productID string
	var quantity int
	err = tx.QueryRowContext(c.Request.Context(), `
		SELECT product_id, quantity FROM transactions WHERE transaction_id=$1 FOR UPDATE`, id).
		Scan(&productID, &quantity)
	if errors.Is(err, sql.ErrNoRows) {
		utils.Error(c, http.StatusNotFound, "Transaksi tidak ditemukan", nil)
		return
	}
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Gagal membaca transaksi", nil)
		return
	}

	if _, err := tx.ExecContext(c.Request.Context(), `UPDATE products SET stock=stock+$1 WHERE product_id=$2`, quantity, productID); err != nil {
		utils.Error(c, http.StatusInternalServerError, "Gagal mengembalikan stok; transaksi tidak dihapus", nil)
		return
	}
	if _, err := tx.ExecContext(c.Request.Context(), `DELETE FROM transactions WHERE transaction_id=$1`, id); err != nil {
		utils.Error(c, http.StatusInternalServerError, "Gagal menghapus transaksi", nil)
		return
	}
	if err := tx.Commit(); err != nil {
		utils.Error(c, http.StatusInternalServerError, "Penghapusan belum dapat dikonfirmasi; muat ulang riwayat", nil)
		return
	}
	utils.Success(c, http.StatusOK, "Transaksi dihapus dan stok dikembalikan", gin.H{"transaction_id": id, "restored_quantity": quantity})
}

