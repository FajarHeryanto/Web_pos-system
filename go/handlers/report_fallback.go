package handlers

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const processedCSVHeader = "transaction_id,product_name,category,quantity,subtotal,discount_amount,net_total,payment_method,transaction_date"

type fallbackTotals struct {
	transactions int64
	quantity     int64
	subtotal     int64 // cents
	discount     int64 // cents
	net          int64 // cents
}

type fallbackCategory struct {
	name string
	fallbackTotals
}

type fallbackDay struct {
	date string
	fallbackTotals
}

type fallbackProduct struct {
	name string
	category string
	quantity int64
	net int64 // cents
}

// generateFallbackReport reproduces the fixed-width sections written by
// cobol/reporter.cob from the same C-processed CSV. It is used in Vercel when
// libcob or another reporter runtime dependency is unavailable.
func generateFallbackReport(inputPath, outputPath string) error {
	file, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("buka CSV terproses: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = 9
	reader.ReuseRecord = false
	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("baca header CSV terproses: %w", err)
	}
	if strings.Join(header, ",") != processedCSVHeader {
		return fmt.Errorf("header CSV terproses tidak sesuai")
	}

	categories := make([]fallbackCategory, 0, 16)
	categoryIndex := make(map[string]int)
	days := make([]fallbackDay, 0, 32)
	dayIndex := make(map[string]int)
	products := make([]fallbackProduct, 0, 64)
	productIndex := make(map[string]int)
	var grand fallbackTotals
	var minDate, maxDate string
	var skipped int64

	for {
		record, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("baca record CSV terproses: %w", readErr)
		}
		if len(record) != 9 {
			return fmt.Errorf("record CSV terproses tidak memiliki 9 field")
		}

		name := fixedText(record[1], 100)
		category := fixedText(record[2], 50)
		date := fixedText(record[8], 10)
		quantity, parseErr := strconv.ParseInt(strings.TrimSpace(record[3]), 10, 64)
		if parseErr != nil || quantity < 0 {
			return fmt.Errorf("quantity CSV tidak valid")
		}
		subtotal, parseErr := parseMoneyCents(record[4])
		if parseErr != nil {
			return fmt.Errorf("subtotal CSV tidak valid: %w", parseErr)
		}
		discount, parseErr := parseMoneyCents(record[5])
		if parseErr != nil {
			return fmt.Errorf("discount CSV tidak valid: %w", parseErr)
		}
		net, parseErr := parseMoneyCents(record[6])
		if parseErr != nil {
			return fmt.Errorf("net total CSV tidak valid: %w", parseErr)
		}

		grand.transactions++
		grand.quantity += quantity
		grand.subtotal += subtotal
		grand.discount += discount
		grand.net += net

		if minDate == "" || date < minDate {
			minDate = date
		}
		if date > maxDate {
			maxDate = date
		}

		if idx, ok := categoryIndex[category]; ok {
			addFallbackTotals(&categories[idx].fallbackTotals, quantity, subtotal, discount, net)
		} else if len(categories) < 50 {
			categoryIndex[category] = len(categories)
			categories = append(categories, fallbackCategory{name: category})
			addFallbackTotals(&categories[len(categories)-1].fallbackTotals, quantity, subtotal, discount, net)
		} else {
			skipped++
		}

		if idx, ok := dayIndex[date]; ok {
			addFallbackTotals(&days[idx].fallbackTotals, quantity, subtotal, discount, net)
		} else if len(days) < 400 {
			dayIndex[date] = len(days)
			days = append(days, fallbackDay{date: date})
			addFallbackTotals(&days[len(days)-1].fallbackTotals, quantity, subtotal, discount, net)
		} else {
			skipped++
		}

		if idx, ok := productIndex[name]; ok {
			products[idx].quantity += quantity
			products[idx].net += net
		} else if len(products) < 500 {
			productIndex[name] = len(products)
			products = append(products, fallbackProduct{name: name, category: category})
			products[len(products)-1].quantity += quantity
			products[len(products)-1].net += net
		} else {
			skipped++
		}
	}

	sort.SliceStable(days, func(i, j int) bool { return days[i].date < days[j].date })
	sort.SliceStable(products, func(i, j int) bool {
		if products[i].quantity != products[j].quantity {
			return products[i].quantity > products[j].quantity
		}
		return products[i].net > products[j].net
	})

	lines := make([]string, 0, 32+len(categories)+len(days)+10)
	put := func(line string) { lines = append(lines, strings.TrimRight(line, " ")) }
	sepEq := strings.Repeat("=", 91)
	sepDash := strings.Repeat("-", 91)
	now := time.Now()
	put(sepEq)
	put("                         LAPORAN PENJUALAN - POINT OF SALES (POS)")
	put("                         Dibuat oleh COBOL Batch Reporting Engine")
	put(sepEq)
	put("Tanggal cetak  : " + now.Format("2006-01-02 15:04:05"))
	if grand.transactions > 0 {
		put("Periode data   : " + minDate + " s/d " + maxDate)
	} else {
		put("Periode data   : -")
	}
	put("Jumlah record  : " + strings.TrimSpace(editedInteger(grand.transactions, 7, 9)) + " transaksi")
	put("")

	put("A. REKAPITULASI PENDAPATAN PER KATEGORI")
	put(sepDash)
	put(categoryHeader())
	put(sepDash)
	if len(categories) == 0 {
		put("(tidak ada data)")
	}
	for _, row := range categories {
		put(fixedText(row.name, 20) + " " +
			editedInteger(row.transactions, 6, 7) + " " +
			editedInteger(row.quantity, 7, 9) + " " +
			editedMoney(row.subtotal, 11, 17) + " " +
			editedMoney(row.discount, 10, 16) + " " +
			editedMoney(row.net, 11, 17))
	}
	put(sepDash)
	put(fixedText("GRAND TOTAL", 20) + " " +
		editedInteger(grand.transactions, 6, 7) + " " +
		editedInteger(grand.quantity, 7, 9) + " " +
		editedMoney(grand.subtotal, 11, 17) + " " +
		editedMoney(grand.discount, 10, 16) + " " +
		editedMoney(grand.net, 11, 17))
	put(sepDash)
	average := int64(0)
	if grand.transactions > 0 {
		average = grand.net / grand.transactions
		remainder := grand.net % grand.transactions
		if remainder >= (grand.transactions+1)/2 {
			average++
		}
	}
	put("Rata-rata net per transaksi : " + strings.TrimSpace(editedMoney(average, 11, 17)))
	put("")

	put("B. LAPORAN PENJUALAN HARIAN")
	put(sepDash)
	put(dailyHeader())
	put(sepDash)
	if len(days) == 0 {
		put("(tidak ada data)")
	}
	for _, row := range days {
		put(fixedText(row.date, 10) + "  " +
			editedInteger(row.transactions, 6, 7) + "  " +
			editedInteger(row.quantity, 7, 9) + "  " +
			editedMoney(row.net, 11, 17))
	}
	put(sepDash)
	put(fixedText("TOTAL", 10) + "  " +
		editedInteger(grand.transactions, 6, 7) + "  " +
		editedInteger(grand.quantity, 7, 9) + "  " +
		editedMoney(grand.net, 11, 17))
	put("")

	put("C. LAPORAN PRODUK TERLARIS (TOP 10)")
	put(sepDash)
	put(productHeader())
	put(sepDash)
	if len(products) == 0 {
		put("(tidak ada data)")
	}
	limit := len(products)
	if limit > 10 {
		limit = 10
	}
	for i := 0; i < limit; i++ {
		row := products[i]
		put(editedInteger(int64(i+1), 2, 2) + ". " +
			fixedText(row.name, 32) + " " +
			fixedText(row.category, 18) + " " +
			editedInteger(row.quantity, 7, 9) + " " +
			editedMoney(row.net, 11, 17))
	}
	put("")

	put(sepEq)
	if skipped > 0 {
		put("CATATAN: sebagian data melebihi kapasitas tabel laporan.")
	}
	put("                                   *** AKHIR LAPORAN ***")
	put(sepEq)

	content := strings.Join(lines, "\n") + "\n"
	if len(content) > maxReportBytes {
		return fmt.Errorf("laporan melebihi batas ukuran")
	}
	if err := os.WriteFile(outputPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("tulis laporan fallback: %w", err)
	}
	return nil
}

func addFallbackTotals(totals *fallbackTotals, quantity, subtotal, discount, net int64) {
	totals.transactions++
	totals.quantity += quantity
	totals.subtotal += subtotal
	totals.discount += discount
	totals.net += net
}

func parseMoneyCents(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("nilai kosong")
	}
	negative := strings.HasPrefix(value, "-")
	if negative {
		value = strings.TrimPrefix(value, "-")
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, fmt.Errorf("format desimal tidak valid")
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	const maxInt64 int64 = 1<<63 - 1
	if err != nil || whole < 0 || whole > maxInt64/100 {
		return 0, fmt.Errorf("bagian bilangan tidak valid")
	}
	fraction := "00"
	if len(parts) == 2 {
		fraction = parts[1]
		if len(fraction) > 2 || fraction == "" {
			return 0, fmt.Errorf("maksimal dua angka desimal")
		}
		for len(fraction) < 2 {
			fraction += "0"
		}
	}
	cents, err := strconv.ParseInt(fraction, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("angka desimal tidak valid")
	}
	if whole == maxInt64/100 && cents > maxInt64%100 {
		return 0, fmt.Errorf("nilai uang di luar jangkauan")
	}
	result := whole*100 + cents
	if negative {
		result = -result
	}
	return result, nil
}

func fixedText(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if len(value) > width {
		value = value[:width]
	}
	return value + strings.Repeat(" ", width-len(value))
}

func editedInteger(value int64, integerDigits, width int) string {
	if value < 0 {
		return strings.Repeat("*", width)
	}
	digits := strconv.FormatInt(value, 10)
	if len(digits) > integerDigits {
		return strings.Repeat("*", width)
	}
	grouped := groupThousands(digits)
	if len(grouped) > width {
		return strings.Repeat("*", width)
	}
	return strings.Repeat(" ", width-len(grouped)) + grouped
}

func editedMoney(cents int64, integerDigits, width int) string {
	if cents < 0 {
		return strings.Repeat("*", width)
	}
	whole := cents / 100
	fraction := cents % 100
	digits := strconv.FormatInt(whole, 10)
	if len(digits) > integerDigits {
		return strings.Repeat("*", width)
	}
	formatted := groupThousands(digits) + fmt.Sprintf(".%02d", fraction)
	if len(formatted) > width {
		return strings.Repeat("*", width)
	}
	return strings.Repeat(" ", width-len(formatted)) + formatted
}

func groupThousands(digits string) string {
	if len(digits) <= 3 {
		return digits
	}
	first := len(digits) % 3
	if first == 0 {
		first = 3
	}
	var b strings.Builder
	b.Grow(len(digits) + len(digits)/3)
	b.WriteString(digits[:first])
	for i := first; i < len(digits); i += 3 {
		b.WriteByte(',')
		b.WriteString(digits[i : i+3])
	}
	return b.String()
}

func categoryHeader() string {
	return fixedText("KATEGORI", 20) + " " + fixedText("    TRX", 7) + " " +
		fixedText("      QTY", 9) + " " + fixedText("         SUBTOTAL", 17) + " " +
		fixedText("          DISKON", 16) + " " + fixedText("        NET TOTAL", 17)
}

func dailyHeader() string {
	return fixedText("TANGGAL", 10) + "  " + fixedText("    TRX", 7) + "  " +
		fixedText("      QTY", 9) + "  " + fixedText("        NET TOTAL", 17)
}

func productHeader() string {
	return fixedText("NO", 4) + fixedText("PRODUK", 32) + " " + fixedText("KATEGORI", 18) + " " +
		fixedText("      QTY", 9) + " " + fixedText("        NET TOTAL", 17)
}
