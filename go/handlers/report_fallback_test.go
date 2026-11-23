package handlers

import (
	"encoding/csv"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoFallbackMatchesCobolReport(t *testing.T) {
	reporter := os.Getenv("COBOL_REPORTER_TEST_BIN")
	if reporter == "" {
		t.Skip("COBOL_REPORTER_TEST_BIN is set by the Linux build workflow")
	}

	dir := t.TempDir()
	inputPath := filepath.Join(dir, "processed_data.csv")
	cobolPath := filepath.Join(dir, "cobol.txt")
	goPath := filepath.Join(dir, "go.txt")
	file, err := os.Create(inputPath)
	if err != nil {
		t.Fatal(err)
	}
	writer := csv.NewWriter(file)
	rows := [][]string{
		strings.Split(processedCSVHeader, ","),
		{"T001", "Nasi Goreng", "Makanan", "2", "50000.00", "5000.00", "45000.00", "Cash", "2026-11-01"},
		{"T002", "Teh Manis", "Minuman", "3", "24000.00", "0.00", "24000.00", "Debit", "2026-11-03"},
		{"T003", "Nasi Goreng", "Makanan", "1", "25000.00", "2500.00", "22500.00", "Credit", "2026-11-03"},
	}
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			file.Close()
			t.Fatal(err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	if err := generateFallbackReport(inputPath, goPath); err != nil {
		t.Fatalf("Go fallback: %v", err)
	}
	command := exec.Command(reporter, inputPath, cobolPath)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("COBOL Reporter gagal: %v; %s", err, output)
	}
	cobolReport, err := os.ReadFile(cobolPath)
	if err != nil {
		t.Fatal(err)
	}
	goReport, err := os.ReadFile(goPath)
	if err != nil {
		t.Fatal(err)
	}
	if normalizeReportForComparison(string(cobolReport)) != normalizeReportForComparison(string(goReport)) {
		t.Fatalf("Go fallback tidak sama dengan COBOL Reporter\n--- COBOL ---\n%s\n--- Go ---\n%s", cobolReport, goReport)
	}
}

func normalizeReportForComparison(report string) string {
	lines := strings.Split(report, "\n")
	for i, line := range lines {
		line = strings.TrimRight(line, " ")
		if strings.HasPrefix(line, "Tanggal cetak  : ") {
			line = "Tanggal cetak  : <runtime timestamp>"
		}
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}
