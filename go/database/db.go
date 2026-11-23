package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"pos-system/go/utils"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var db *sql.DB

// DB mengembalikan koneksi database yang telah diinisialisasi.
func DB() *sql.DB { return db }

// Init membuka pool PostgreSQL Supabase, memeriksa konektivitas,
// membuat skema bila diperlukan, dan mengisi data demo yang belum ada.
func Init(ctx context.Context) error {
	url := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if url == "" {
		return fmt.Errorf("DATABASE_URL belum diatur; salin .env.example menjadi .env dan isi connection string Supabase")
	}

	opened, err := sql.Open("pgx", url)
	if err != nil {
		return fmt.Errorf("gagal menyiapkan koneksi PostgreSQL: %w", err)
	}
	opened.SetMaxOpenConns(10)
	opened.SetMaxIdleConns(5)
	opened.SetConnMaxLifetime(30 * time.Minute)
	opened.SetConnMaxIdleTime(5 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := opened.PingContext(pingCtx); err != nil {
		opened.Close()
		return fmt.Errorf("gagal tersambung ke Supabase: %w", err)
	}

	db = opened
	if err := migrate(ctx); err != nil {
		opened.Close()
		db = nil
		return err
	}
	if err := seed(ctx); err != nil {
		opened.Close()
		db = nil
		return err
	}
	return nil
}

// Close menutup pool koneksi.
func Close() {
	if db != nil {
		_ = db.Close()
		db = nil
	}
}

func migrate(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			user_id VARCHAR(10) PRIMARY KEY,
			username VARCHAR(50) NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			full_name VARCHAR(100) NOT NULL,
			role VARCHAR(20) NOT NULL CHECK (role IN ('admin', 'kasir'))
		)`,
		`CREATE TABLE IF NOT EXISTS products (
			product_id VARCHAR(10) PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			category VARCHAR(50) NOT NULL,
			price NUMERIC(12, 2) NOT NULL CHECK (price > 0),
			stock INTEGER NOT NULL DEFAULT 0 CHECK (stock >= 0),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS transactions (
			transaction_id VARCHAR(10) PRIMARY KEY,
			product_id VARCHAR(10) NOT NULL REFERENCES products(product_id) ON DELETE RESTRICT,
			product_name VARCHAR(100) NOT NULL,
			quantity INTEGER NOT NULL CHECK (quantity > 0),
			unit_price NUMERIC(12, 2) NOT NULL CHECK (unit_price > 0),
			discount_percent NUMERIC(5, 2) DEFAULT 0 CHECK (discount_percent >= 0 AND discount_percent <= 100),
			total_price NUMERIC(12, 2) NOT NULL CHECK (total_price >= 0),
			payment_method VARCHAR(20) NOT NULL CHECK (payment_method IN ('Cash', 'Debit', 'Credit')),
			transaction_date VARCHAR(15) NOT NULL,
			cashier_name VARCHAR(100) NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_trans_date ON transactions(transaction_date)`,
		`CREATE INDEX IF NOT EXISTS idx_trans_product ON transactions(product_id)`,
		`CREATE INDEX IF NOT EXISTS idx_product_category ON products(category)`,
		`CREATE INDEX IF NOT EXISTS idx_trans_date_id ON transactions(transaction_date, transaction_id)`,
	}
	for _, stmt := range statements {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("gagal membuat skema: %w", err)
		}
	}
	return nil
}

type seedUser struct {
	id, username, password, name, role string
}

type seedProduct struct {
	id, name, category string
	price              float64
	stock              int
}

func seed(ctx context.Context) error {
	// bcrypt hash dibuat saat runtime; tidak pernah menyimpan password polos.
	users := []seedUser{
		{"U001", "admin", "admin123", "Administrator", "admin"},
		{"U002", "andi", "kasir123", "Andi Pratama", "kasir"},
		{"U003", "budi", "kasir123", "Budi Santoso", "kasir"},
		{"U004", "citra", "kasir123", "Citra Dewi", "kasir"},
		{"U005", "dewi", "kasir123", "Dewi Lestari", "kasir"},
	}
	for _, u := range users {
		var existing string
		err := db.QueryRowContext(ctx, `SELECT user_id FROM users WHERE username = $1`, u.username).Scan(&existing)
		if err == nil {
			continue
		}
		if err != sql.ErrNoRows {
			return fmt.Errorf("gagal memeriksa user demo: %w", err)
		}
		hash, err := utils.HashPassword(u.password)
		if err != nil {
			return fmt.Errorf("gagal membuat hash password demo: %w", err)
		}
		_, err = db.ExecContext(ctx, `
			INSERT INTO users (user_id, username, password_hash, full_name, role)
			VALUES ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING`,
			u.id, u.username, hash, u.name, u.role)
		if err != nil {
			return fmt.Errorf("gagal mengisi user demo %s: %w", u.username, err)
		}
	}

	products := []seedProduct{
		{"P001", "Nasi Goreng", "Makanan", 25000, 100},
		{"P002", "Mie Ayam", "Makanan", 20000, 80},
		{"P003", "Ayam Geprek", "Makanan", 22000, 60},
		{"P004", "Sate Ayam", "Makanan", 30000, 50},
		{"P005", "Kopi Arabica", "Minuman", 18000, 150},
		{"P006", "Teh Manis", "Minuman", 8000, 200},
		{"P007", "Jus Jeruk", "Minuman", 15000, 100},
		{"P008", "Es Coklat", "Minuman", 12000, 120},
		{"P009", "Mouse Wireless", "Elektronik", 150000, 30},
		{"P010", "Keyboard Mech", "Elektronik", 350000, 20},
		{"P011", "Headset Gaming", "Elektronik", 275000, 25},
		{"P012", "Flash Disk 32GB", "Elektronik", 85000, 50},
		{"P013", "Pulpen", "Alat Tulis", 5000, 300},
		{"P014", "Buku Tulis A5", "Alat Tulis", 8000, 250},
		{"P015", "Penghapus", "Alat Tulis", 3000, 200},
		{"P016", "Pensil 2B", "Alat Tulis", 4000, 300},
		{"P017", "Sabun Cuci", "Kebutuhan RT", 15000, 100},
		{"P018", "Tisu Wajah", "Kebutuhan RT", 12000, 150},
		{"P019", "Sikat Gigi", "Kebutuhan RT", 10000, 100},
		{"P020", "Pasta Gigi", "Kebutuhan RT", 18000, 80},
	}
	for _, p := range products {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO products (product_id, name, category, price, stock)
			VALUES ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING`,
			p.id, p.name, p.category, p.price, p.stock); err != nil {
			return fmt.Errorf("gagal mengisi produk demo %s: %w", p.id, err)
		}
	}

	if err := seedTransactions(ctx); err != nil {
		return err
	}
	return nil
}

func seedTransactions(ctx context.Context) error {
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM transactions`).Scan(&count); err != nil {
		return fmt.Errorf("gagal memeriksa transaksi demo: %w", err)
	}
	if count > 0 {
		return nil
	}

	// Buat pola deterministik agar data demo selalu dapat direproduksi.
	rng := rand.New(rand.NewSource(20261101))
	cashiers := []string{"Andi Pratama", "Budi Santoso", "Citra Dewi", "Dewi Lestari"}
	paymentFor := func(i int) string {
		p := i % 10
		if p < 5 {
			return "Cash"
		}
		if p < 8 {
			return "Debit"
		}
		return "Credit"
	}
	products := []seedProduct{
		{"P001", "Nasi Goreng", "Makanan", 25000, 100}, {"P002", "Mie Ayam", "Makanan", 20000, 80},
		{"P003", "Ayam Geprek", "Makanan", 22000, 60}, {"P004", "Sate Ayam", "Makanan", 30000, 50},
		{"P005", "Kopi Arabica", "Minuman", 18000, 150}, {"P006", "Teh Manis", "Minuman", 8000, 200},
		{"P007", "Jus Jeruk", "Minuman", 15000, 100}, {"P008", "Es Coklat", "Minuman", 12000, 120},
		{"P009", "Mouse Wireless", "Elektronik", 150000, 30}, {"P010", "Keyboard Mech", "Elektronik", 350000, 20},
		{"P011", "Headset Gaming", "Elektronik", 275000, 25}, {"P012", "Flash Disk 32GB", "Elektronik", 85000, 50},
		{"P013", "Pulpen", "Alat Tulis", 5000, 300}, {"P014", "Buku Tulis A5", "Alat Tulis", 8000, 250},
		{"P015", "Penghapus", "Alat Tulis", 3000, 200}, {"P016", "Pensil 2B", "Alat Tulis", 4000, 300},
		{"P017", "Sabun Cuci", "Kebutuhan RT", 15000, 100}, {"P018", "Tisu Wajah", "Kebutuhan RT", 12000, 150},
		{"P019", "Sikat Gigi", "Kebutuhan RT", 10000, 100}, {"P020", "Pasta Gigi", "Kebutuhan RT", 18000, 80},
	}
	discounts := []float64{0, 5, 10, 15}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("gagal memulai seeding transaksi demo: %w", err)
	}
	defer tx.Rollback()

	for i := 1; i <= 100; i++ {
		product := products[rng.Intn(len(products))]
		quantity := rng.Intn(4) + 1
		discount := discounts[rng.Intn(len(discounts))]
		subtotal := float64(quantity) * product.price
		total := utils.Round2(subtotal * (1 - discount/100))
		day := 1 + (i-1)%19
		date := fmt.Sprintf("2026-11-%02d", day)
		id := fmt.Sprintf("T%03d", i)
		cashier := cashiers[rng.Intn(len(cashiers))]

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO transactions
			(transaction_id, product_id, product_name, quantity, unit_price,
			 discount_percent, total_price, payment_method, transaction_date, cashier_name)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT DO NOTHING`,
			id, product.id, product.name, quantity, product.price, discount,
			total, paymentFor(i), date, cashier); err != nil {
			return fmt.Errorf("gagal mengisi transaksi demo %s: %w", id, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("gagal menyimpan transaksi demo: %w", err)
	}
	log.Printf("100 transaksi demo berhasil dibuat (2026-11-01 s/d 2026-11-19)")
	return nil
}
