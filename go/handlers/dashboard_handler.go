package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"pos-system/go/database"
	"pos-system/go/models"
	"pos-system/go/utils"
)

// Dashboard mengembalikan KPI, penjualan harian, top 5 produk, dan transaksi terbaru.
func Dashboard(c *gin.Context) {
	db := database.DB()
	ctx := c.Request.Context()

	var kpi struct {
		Revenue          float64 `json:"total_revenue"`
		Transactions     int64   `json:"total_transactions"`
		Products         int64   `json:"total_products"`
		LowStockProducts int64   `json:"low_stock_products"`
	}
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(SUM(total_price),0)::float8, COUNT(*) FROM transactions`).
		Scan(&kpi.Revenue, &kpi.Transactions); err != nil {
		utils.Error(c, http.StatusInternalServerError, "Gagal menghitung ringkasan penjualan", nil)
		return
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*), COUNT(*) FILTER (WHERE stock <= 10) FROM products`).
		Scan(&kpi.Products, &kpi.LowStockProducts); err != nil {
		utils.Error(c, http.StatusInternalServerError, "Gagal menghitung ringkasan stok", nil)
		return
	}

	// Rentang tetap menampilkan 7 hari terakhir termasuk tanggal hari ini.
	start := time.Now().AddDate(0, 0, -6).Format("2006-01-02")
	rows, err := db.QueryContext(ctx, `
		SELECT transaction_date, COALESCE(SUM(total_price),0)::float8, COUNT(*)
		FROM transactions WHERE transaction_date >= $1
		GROUP BY transaction_date ORDER BY transaction_date`, start)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Gagal mengambil grafik penjualan harian", nil)
		return
	}
	daily := make([]gin.H, 0)
	for rows.Next() {
		var date string
		var total float64
		var count int64
		if err := rows.Scan(&date, &total, &count); err != nil {
			rows.Close()
			utils.Error(c, http.StatusInternalServerError, "Gagal membaca grafik penjualan", nil)
			return
		}
		daily = append(daily, gin.H{"date": date, "revenue": total, "transactions": count})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		utils.Error(c, http.StatusInternalServerError, "Gagal membaca grafik penjualan", nil)
		return
	}
	rows.Close()

	rows, err = db.QueryContext(ctx, `
		SELECT product_id, product_name, SUM(quantity)::bigint AS sold,
		       SUM(total_price)::float8 AS revenue
		FROM transactions GROUP BY product_id, product_name
		ORDER BY sold DESC, revenue DESC, product_name LIMIT 5`)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Gagal mengambil produk terlaris", nil)
		return
	}
	top := make([]gin.H, 0)
	for rows.Next() {
		var id, name string
		var sold int64
		var revenue float64
		if err := rows.Scan(&id, &name, &sold, &revenue); err != nil {
			rows.Close()
			utils.Error(c, http.StatusInternalServerError, "Gagal membaca produk terlaris", nil)
			return
		}
		top = append(top, gin.H{"product_id": id, "product_name": name, "quantity_sold": sold, "revenue": revenue})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		utils.Error(c, http.StatusInternalServerError, "Gagal membaca produk terlaris", nil)
		return
	}
	rows.Close()

	rows, err = db.QueryContext(ctx, `
		SELECT transaction_id, product_id, product_name, quantity,
		       unit_price::float8, COALESCE(discount_percent,0)::float8,
		       total_price::float8, payment_method, transaction_date, cashier_name
		FROM transactions ORDER BY transaction_date DESC,
		LENGTH(transaction_id) DESC, transaction_id DESC LIMIT 8`)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Gagal mengambil transaksi terbaru", nil)
		return
	}
	recent := make([]models.Transaction, 0)
	for rows.Next() {
		var t models.Transaction
		if err := rows.Scan(&t.TransactionID, &t.ProductID, &t.ProductName, &t.Quantity,
			&t.UnitPrice, &t.DiscountPercent, &t.TotalPrice, &t.PaymentMethod,
			&t.TransactionDate, &t.CashierName); err != nil {
			rows.Close()
			utils.Error(c, http.StatusInternalServerError, "Gagal membaca transaksi terbaru", nil)
			return
		}
		recent = append(recent, t)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		utils.Error(c, http.StatusInternalServerError, "Gagal membaca transaksi terbaru", nil)
		return
	}
	rows.Close()

	utils.Success(c, http.StatusOK, "Dashboard berhasil dimuat", gin.H{
		"kpi": kpi, "daily_sales": daily, "top_products": top, "recent_transactions": recent,
	})
}
