package models

import "time"

// Product merepresentasikan satu baris tabel products.
type Product struct {
	ProductID string     `json:"product_id"`
	Name      string     `json:"name"`
	Category  string     `json:"category"`
	Price     float64    `json:"price"`
	Stock     int        `json:"stock"`
	CreatedAt *time.Time `json:"created_at"`
}

// ProductInput adalah body JSON untuk tambah/ubah produk.
// Price dan Stock memakai pointer agar field yang tidak dikirim terdeteksi.
type ProductInput struct {
	ProductID string   `json:"product_id"`
	Name      string   `json:"name"`
	Category  string   `json:"category"`
	Price     *float64 `json:"price"`
	Stock     *int     `json:"stock"`
}
