package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"pos-system/go/database"
	"pos-system/go/utils"
)

var engineMu sync.Mutex

// ProcessTransactions mengekspor transaksi ke CSV dan menjalankan C Engine.
func ProcessTransactions(c *gin.Context) {
	engineMu.Lock()
	defer engineMu.Unlock()
	ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
	defer cancel()

	inputPath := utils.TransactionsCSVPath()
	outputPath := utils.ProcessedCSVPath()
	count, err := utils.ExportTransactionsCSV(ctx, database.DB(), inputPath)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Gagal mengekspor transaksi dari Supabase", nil)
		return
	}

	binary, err := utils.FindBinary("processor", "C_PROCESSOR_PATH", "c")
	if err != nil {
		utils.Error(c, http.StatusServiceUnavailable,
			"C Engine belum tersedia. Kompilasi c/processor.c atau siapkan binary Linux untuk serverless.", nil)
		return
	}
	cmd := exec.CommandContext(ctx, binary, inputPath, outputPath)
	cmd.Dir = utils.ProjectRoot()
	output, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			utils.Error(c, http.StatusGatewayTimeout, "C Engine melewati batas waktu pemrosesan", nil)
			return
		}
		message := "C Engine gagal memproses CSV"
		if exit, ok := err.(*exec.ExitError); ok {
			switch exit.ExitCode() {
			case 1:
				message = "File transaksi tidak ditemukan oleh C Engine"
			case 2:
				message = "Format CSV tidak valid; periksa data ekspor transaksi"
			case 3:
				message = "C Engine kehabisan memori"
			}
		}
		utils.Error(c, http.StatusUnprocessableEntity, message, nil)
		return
	}

	// Laporan sebelumnya tidak lagi sesuai dengan CSV yang baru diproses.
	_ = os.Remove(utils.ReportOutputPath())
	utils.Success(c, http.StatusOK, "Transaksi berhasil diproses oleh C Engine", gin.H{
		"record_count": count,
		"input_file":   inputPath,
		"output_file":  outputPath,
		"statistics":   parseEngineStats(string(output)),
	})
}

// parseEngineStats mengambil baris STAT key=value yang dicetak oleh C Engine.
func parseEngineStats(output string) map[string]string {
	stats := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "STAT ") {
			parts := strings.SplitN(strings.TrimPrefix(line, "STAT "), "=", 2)
			if len(parts) == 2 {
				stats[parts[0]] = parts[1]
			}
		}
	}
	if len(stats) == 0 {
		return map[string]string{"message": fmt.Sprintf("%d baris diproses", len(output))}
	}
	return stats
}
