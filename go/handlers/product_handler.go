package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"pos-system/go/database"
	"pos-system/go/models"
	"pos-system/go/utils"
)

const productSelect = `SELECT product_id, name, category, price::float8, stock, created_at FROM products`

// ListProducts mengambil katalog produk dengan filter teks/kategori.
func ListProducts(c *gin.Context) {
	search := strings.TrimSpace(c.Query("search"))
	category := strings.TrimSpace(c.Query("category"))
	if len(search) > 100 || len(category) > 50 {
		utils.Error(c, http.StatusBadRequest, "Filter terlalu panjang", []utils.FieldError{{Field: "search/category", Message: "Panjang filter melebihi batas"}})
		return
	}

	query := productSelect + ` WHERE ($1 = '' OR name ILIKE '%' || $1 || '%' OR product_id ILIKE '%' || $1 || '%') AND ($2 = '' OR category = $2) ORDER BY category, name`
	rows, err := database.DB().QueryContext(c.Request.Context(), query, search, category)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Gagal mengambil daftar produk", nil)
		return
	}
	defer rows.Close()

	products := make([]models.Product, 0)
	for rows.Next() {
		var p models.Product
		if err := rows.Scan(&p.ProductID, &p.Name, &p.Category, &p.Price, &p.Stock, &p.CreatedAt); err != nil {
			utils.Error(c, http.StatusInternalServerError, "Gagal membaca data produk", nil)
			return
		}
		products = append(products, p)
	}
	if err := rows.Err(); err != nil {
		utils.Error(c, http.StatusInternalServerError, "Gagal membaca daftar produk", nil)
		return
	}
	utils.Success(c, http.StatusOK, "Daftar produk berhasil diambil", products)
}

// GetProduct mengambil detail satu produk.
func GetProduct(c *gin.Context) {
	id := strings.ToUpper(strings.TrimSpace(c.Param("id")))
	if !utils.IsValidID(id) {
		utils.Error(c, http.StatusBadRequest, "ID produk tidak valid", []utils.FieldError{{Field: "id", Message: "Gunakan ID maksimal 10 karakter"}})
		return
	}
	var p models.Product
	err := database.DB().QueryRowContext(c.Request.Context(), productSelect+` WHERE product_id = $1`, id).
		Scan(&p.ProductID, &p.Name, &p.Category, &p.Price, &p.Stock, &p.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		utils.Error(c, http.StatusNotFound, "Produk tidak ditemukan", nil)
		return
	}
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Gagal mengambil produk", nil)
		return
	}
	utils.Success(c, http.StatusOK, "Detail produk berhasil diambil", p)
}

func newProductID(c *gin.Context) (string, error) {
	var next int
	err := database.DB().QueryRowContext(c.Request.Context(), `
		SELECT COALESCE(MAX(NULLIF(regexp_replace(product_id, '[^0-9]', '', 'g'), '')::bigint), 0) + 1
		FROM products WHERE product_id ~ '^P[0-9]+$'`).Scan(&next)
	if err != nil {
		return "", err
	}
	for attempts := 0; attempts < 100; attempts++ {
		candidate := fmt.Sprintf("P%03d", next+attempts)
		var exists bool
		if err := database.DB().QueryRowContext(c.Request.Context(), `SELECT EXISTS(SELECT 1 FROM products WHERE product_id=$1)`, candidate).Scan(&exists); err != nil {
			return "", err
		}
		if !exists && len(candidate) <= 10 {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("tidak dapat membuat ID produk baru")
}

// CreateProduct menambahkan produk baru. Hanya dipanggil pada rute admin.
func CreateProduct(c *gin.Context) {
	var input models.ProductInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.Error(c, http.StatusBadRequest, "Format JSON tidak valid", []utils.FieldError{{Field: "body", Message: "Periksa kembali data produk"}})
		return
	}
	if errs := utils.ValidateProduct(&input, false); len(errs) > 0 {
		utils.Error(c, http.StatusBadRequest, "Data produk tidak valid", errs)
		return
	}
	if input.ProductID == "" {
		id, err := newProductID(c)
		if err != nil {
			utils.Error(c, http.StatusInternalServerError, "Gagal membuat ID produk", nil)
			return
		}
		input.ProductID = id
	}
	var product models.Product
	err := database.DB().QueryRowContext(c.Request.Context(), `
		INSERT INTO products (product_id,name,category,price,stock)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING product_id,name,category,price::float8,stock,created_at`,
		input.ProductID, input.Name, input.Category, *input.Price, *input.Stock).
		Scan(&product.ProductID, &product.Name, &product.Category, &product.Price, &product.Stock, &product.CreatedAt)
	if err != nil {
		utils.Error(c, http.StatusConflict, "ID produk sudah digunakan atau data tidak dapat disimpan", nil)
		return
	}
	utils.Success(c, http.StatusCreated, "Produk berhasil ditambahkan", product)
}

// UpdateProduct mengubah nama, kategori, harga, dan stok. Hanya admin.
func UpdateProduct(c *gin.Context) {
	id := strings.ToUpper(strings.TrimSpace(c.Param("id")))
	if !utils.IsValidID(id) {
		utils.Error(c, http.StatusBadRequest, "ID produk tidak valid", []utils.FieldError{{Field: "id", Message: "Gunakan ID maksimal 10 karakter"}})
		return
	}
	var input models.ProductInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.Error(c, http.StatusBadRequest, "Format JSON tidak valid", []utils.FieldError{{Field: "body", Message: "Periksa kembali data produk"}})
		return
	}
	if errs := utils.ValidateProduct(&input, false); len(errs) > 0 {
		utils.Error(c, http.StatusBadRequest, "Data produk tidak valid", errs)
		return
	}
	result, err := database.DB().ExecContext(c.Request.Context(), `
		UPDATE products SET name=$1, category=$2, price=$3, stock=$4 WHERE product_id=$5`,
		input.Name, input.Category, *input.Price, *input.Stock, id)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Gagal memperbarui produk", nil)
		return
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		utils.Error(c, http.StatusNotFound, "Produk tidak ditemukan", nil)
		return
	}
	var p models.Product
	if err := database.DB().QueryRowContext(c.Request.Context(), productSelect+` WHERE product_id=$1`, id).
		Scan(&p.ProductID, &p.Name, &p.Category, &p.Price, &p.Stock, &p.CreatedAt); err != nil {
		utils.Error(c, http.StatusInternalServerError, "Produk tersimpan tetapi gagal dibaca kembali", nil)
		return
	}
	utils.Success(c, http.StatusOK, "Produk berhasil diperbarui", p)
}

// DeleteProduct menghapus produk tanpa histori transaksi.
func DeleteProduct(c *gin.Context) {
	id := strings.ToUpper(strings.TrimSpace(c.Param("id")))
	if !utils.IsValidID(id) {
		utils.Error(c, http.StatusBadRequest, "ID produk tidak valid", []utils.FieldError{{Field: "id", Message: "Gunakan ID maksimal 10 karakter"}})
		return
	}
	result, err := database.DB().ExecContext(c.Request.Context(), `DELETE FROM products WHERE product_id=$1`, id)
	if err != nil {
		utils.Error(c, http.StatusConflict, "Produk memiliki histori transaksi dan tidak dapat dihapus", nil)
		return
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		utils.Error(c, http.StatusNotFound, "Produk tidak ditemukan", nil)
		return
	}
	utils.Success(c, http.StatusOK, "Produk berhasil dihapus", gin.H{"product_id": id})
}

