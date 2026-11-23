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
	"pos-system/go/database"
	"pos-system/go/utils"
)

const maxReportBytes = 2 * 1024 * 1024

// GenerateReport menjalankan COBOL Reporter. Pada Vercel, kegagalan binary
// (termasuk libcob/shared library yang tidak tersedia) dialihkan ke Go fallback.
func GenerateReport(c *gin.Context) {
	engineMu.Lock()
	defer engineMu.Unlock()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
	defer cancel()
	inputPath := utils.ProcessedCSVPath()
	if _, err := os.Stat(inputPath); err != nil {
		if !utils.IsVercel() {
			utils.Error(c, http.StatusConflict, "Data belum diproses. Jalankan C Engine terlebih dahulu", nil)
			return
		}
		// /tmp tidak dibagi antar instance serverless. Ekspor dan proses ulang
		// bila GenerateReport masuk ke instance berbeda dari POST /process.
		if err := regenerateProcessedCSV(ctx, inputPath); err != nil {
			utils.Error(c, http.StatusServiceUnavailable, "Data proses tidak tersedia di instance ini dan C Engine gagal dijalankan ulang", nil)
			return
		}
	}

	outputPath := utils.ReportOutputPath()
	tempOutputPath := outputPath + ".tmp"
	binary, err := utils.FindBinary("reporter", "COBOL_REPORTER_PATH", "cobol")
	if err != nil {
		if utils.IsVercel() {
			respondWithGoFallback(c, inputPath, tempOutputPath, outputPath)
			return
		}
		utils.Error(c, http.StatusServiceUnavailable,
			"COBOL Reporter belum tersedia. Kompilasi cobol/reporter.cob atau gunakan alur hybrid yang dijelaskan di README.", nil)
		return
	}

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
	_, runErr := cmd.CombinedOutput()
	if runErr != nil {
		if utils.IsVercel() {
			respondWithGoFallback(c, inputPath, tempOutputPath, outputPath)
			return
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			utils.Error(c, http.StatusGatewayTimeout, "COBOL Reporter melewati batas waktu", nil)
			return
		}
		utils.Error(c, http.StatusUnprocessableEntity, "COBOL Reporter gagal membuat laporan", nil)
		return
	}

	if _, err := os.Stat(tempOutputPath); err != nil {
		if utils.IsVercel() {
			respondWithGoFallback(c, inputPath, tempOutputPath, outputPath)
			return
		}
		utils.Error(c, http.StatusUnprocessableEntity, "COBOL Reporter selesai tetapi file laporan tidak ditemukan", nil)
		return
	}
	if err := publishReport(tempOutputPath, outputPath); err != nil {
		utils.Error(c, http.StatusInternalServerError, "Gagal menyimpan file laporan", nil)
		return
	}
	utils.Success(c, http.StatusOK, "Laporan berhasil dibuat oleh COBOL Reporter", gin.H{
		"output_file": outputPath,
		"engine":      "cobol",
	})
}

func regenerateProcessedCSV(ctx context.Context, processedPath string) error {
	csvPath := utils.TransactionsCSVPath()
	if _, err := utils.ExportTransactionsCSV(ctx, database.DB(), csvPath); err != nil {
		return err
	}
	binary, err := utils.FindBinary("processor", "C_PROCESSOR_PATH", "c")
	if err != nil {
		return err
	}
	command := exec.CommandContext(ctx, binary, csvPath, processedPath)
	command.Dir = utils.ProjectRoot()
	if _, err := command.CombinedOutput(); err != nil {
		return err
	}
	return nil
}

func respondWithGoFallback(c *gin.Context, inputPath, tempOutputPath, outputPath string) {
	_ = os.Remove(tempOutputPath)
	if err := generateFallbackReport(inputPath, tempOutputPath); err != nil {
		utils.Error(c, http.StatusInternalServerError,
			"COBOL Reporter tidak dapat dijalankan dan generator laporan Go gagal: "+err.Error(), nil)
		return
	}
	if err := publishReport(tempOutputPath, outputPath); err != nil {
		utils.Error(c, http.StatusInternalServerError, "Gagal menyimpan laporan fallback", nil)
		return
	}
	utils.Success(c, http.StatusOK, "Laporan berhasil dibuat oleh fallback Go", gin.H{
		"output_file": outputPath,
		"engine":      "go-fallback",
		"message":     "COBOL Reporter tidak tersedia di runtime; format laporan dihasilkan oleh Go.",
	})
}

func publishReport(tempPath, outputPath string) error {
	info, err := os.Stat(tempPath)
	if err != nil {
		return err
	}
	if info.Size() > maxReportBytes {
		_ = os.Remove(tempPath)
		return errors.New("ukuran laporan melebihi batas")
	}
	if runtime.GOOS == "windows" {
		_ = os.Remove(outputPath)
	}
	if err := os.Rename(tempPath, outputPath); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	return nil
}

func ensureVercelReport(parent context.Context, outputPath string) error {
	engineMu.Lock()
	defer engineMu.Unlock()
	if _, err := os.Stat(outputPath); err == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(parent, 45*time.Second)
	defer cancel()
	inputPath := utils.ProcessedCSVPath()
	if _, err := os.Stat(inputPath); err != nil {
		if err := regenerateProcessedCSV(ctx, inputPath); err != nil {
			return err
		}
	}
	tempPath := outputPath + ".tmp"
	_ = os.Remove(tempPath)
	if err := generateFallbackReport(inputPath, tempPath); err != nil {
		return err
	}
	return publishReport(tempPath, outputPath)
}

// GetReport membaca laporan teks COBOL/Go untuk ditampilkan di halaman Laporan.
func GetReport(c *gin.Context) {
	path := utils.ReportOutputPath()
	file, err := os.Open(path)
	if err != nil && os.IsNotExist(err) && utils.IsVercel() {
		// GET dapat masuk ke instance Vercel lain dari POST pembuat laporan.
		// Buat ulang file lokal instance ini agar hasil tetap bisa ditampilkan.
		if createErr := ensureVercelReport(c.Request.Context(), path); createErr == nil {
			file, err = os.Open(path)
		} else {
			utils.Error(c, http.StatusServiceUnavailable, "Laporan belum tersedia di instance ini. Coba buat laporan kembali.", nil)
			return
		}
	}
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
