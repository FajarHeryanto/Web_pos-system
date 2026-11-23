package handlers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"pos-system/go/utils"
)

const maxReportBytes = 2 * 1024 * 1024

// GenerateReport menjalankan COBOL Reporter menggunakan processed_data.csv.
func GenerateReport(c *gin.Context) {
	engineMu.Lock()
	defer engineMu.Unlock()
	inputPath := utils.ProcessedCSVPath()
	if _, err := os.Stat(inputPath); err != nil {
		utils.Error(c, http.StatusConflict, "Data belum diproses. Jalankan C Engine terlebih dahulu", nil)
		return
	}
	binary, err := utils.FindBinary("reporter", "COBOL_REPORTER_PATH", "cobol")
	if err != nil {
		utils.Error(c, http.StatusServiceUnavailable,
			"COBOL Reporter belum tersedia. Kompilasi cobol/reporter.cob atau gunakan alur hybrid yang dijelaskan di README.", nil)
		return
	}

	outputPath := utils.ReportOutputPath()
	tempOutputPath := outputPath + ".tmp"
	ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, inputPath, tempOutputPath)
	cmd.Dir = utils.ProjectRoot()
	cmd.Env = setEnvironment(os.Environ(), "INPUT_CSV_PATH", inputPath)
	cmd.Env = setEnvironment(cmd.Env, "OUTPUT_REPORT_PATH", tempOutputPath)
	if runtime.GOOS == "linux" {
		libDir := filepath.Join(utils.ProjectRoot(), "bin", "lib")
		if info, statErr := os.Stat(libDir); statErr == nil && info.IsDir() {
			libraryPath := libDir
			if inherited := os.Getenv("LD_LIBRARY_PATH"); inherited != "" {
				libraryPath += string(os.PathListSeparator) + inherited
			}
			cmd.Env = setEnvironment(cmd.Env, "LD_LIBRARY_PATH", libraryPath)
		}
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			utils.Error(c, http.StatusGatewayTimeout, "COBOL Reporter melewati batas waktu", nil)
			return
		}
		utils.Error(c, http.StatusUnprocessableEntity, "COBOL Reporter gagal membuat laporan", nil)
		return
	}

	if _, err := os.Stat(tempOutputPath); err != nil {
		utils.Error(c, http.StatusUnprocessableEntity, "COBOL Reporter selesai tetapi file laporan tidak ditemukan", nil)
		return
	}
	// Tempatkan laporan baru hanya setelah reporter selesai tanpa error.
	_ = os.Remove(outputPath)
	if err := os.Rename(tempOutputPath, outputPath); err != nil {
		_ = os.Remove(tempOutputPath)
		utils.Error(c, http.StatusInternalServerError, "Gagal menyimpan file laporan", nil)
		return
	}
	utils.Success(c, http.StatusOK, "Laporan berhasil dibuat oleh COBOL Reporter", gin.H{
		"output_file": outputPath,
		"message":     string(output),
	})
}

// GetReport membaca laporan teks COBOL untuk ditampilkan di halaman Laporan.
func GetReport(c *gin.Context) {
	path := utils.ReportOutputPath()
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			utils.Error(c, http.StatusNotFound, "Laporan belum dibuat. Jalankan C Engine lalu COBOL Reporter", nil)
			return
		}
		utils.Error(c, http.StatusInternalServerError, "Gagal membuka laporan", nil)
		return
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, maxReportBytes+1))
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Gagal membaca laporan", nil)
		return
	}
	if len(content) > maxReportBytes {
		utils.Error(c, http.StatusRequestEntityTooLarge, "Ukuran laporan melebihi batas", nil)
		return
	}
	utils.Success(c, http.StatusOK, "Laporan berhasil diambil", gin.H{"report": string(content), "bytes": len(content)})
}

// setEnvironment mengganti nilai yang mungkin sudah ada di environment.
func setEnvironment(env []string, key, value string) []string {
	prefix := key + "="
	filtered := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if !strings.HasPrefix(entry, prefix) {
			filtered = append(filtered, entry)
		}
	}
	return append(filtered, prefix+value)
}
