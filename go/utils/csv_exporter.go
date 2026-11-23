package utils

import (
	"context"
	"database/sql"
	"encoding/csv"
	"os"
	"strconv"
)

// TransactionsCSVHeader adalah header transactions.csv (kontrak Go -> C).
var TransactionsCSVHeader = []string{
	"transaction_id", "product_id", "product_name", "category", "quantity",
	"unit_price", "discount_percent", "payment_method", "transaction_date", "cashier_name",
}

// ExportTransactionsCSV menulis seluruh transaksi dari database ke file CSV
// untuk diproses C Engine. Mengembalikan jumlah record yang diekspor.
func ExportTransactionsCSV(ctx context.Context, db *sql.DB, path string) (int, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT t.transaction_id, t.product_id, t.product_name,
		       COALESCE(p.category, 'Lainnya'),
		       t.quantity, t.unit_price::float8,
		       COALESCE(t.discount_percent, 0)::float8,
		       t.payment_method, t.transaction_date, t.cashier_name
		FROM transactions t
		LEFT JOIN products p ON p.product_id = t.product_id
		ORDER BY t.transaction_date, LENGTH(t.transaction_id), t.transaction_id`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	tmpPath := path + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return 0, err
	}

	writer := csv.NewWriter(file)
	count := 0
	fail := func(e error) (int, error) {
		file.Close()
		os.Remove(tmpPath)
		return 0, e
	}

	if err := writer.Write(TransactionsCSVHeader); err != nil {
		return fail(err)
	}

	for rows.Next() {
		var (
			id, productID, name, category, payment, date, cashier string
			quantity                                              int
			unitPrice, discount                                   float64
		)
		if err := rows.Scan(&id, &productID, &name, &category, &quantity,
			&unitPrice, &discount, &payment, &date, &cashier); err != nil {
			return fail(err)
		}
		record := []string{
			id, productID, name, category,
			strconv.Itoa(quantity),
			strconv.FormatFloat(unitPrice, 'f', 2, 64),
			strconv.FormatFloat(discount, 'f', 2, 64),
			payment, date, cashier,
		}
		if err := writer.Write(record); err != nil {
			return fail(err)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return fail(err)
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return fail(err)
	}
	if err := file.Close(); err != nil {
		os.Remove(tmpPath)
		return 0, err
	}
	os.Remove(path)
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return 0, err
	}
	return count, nil
}
