package utils

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

// IsVercel bernilai true bila proses berjalan di runtime Vercel.
func IsVercel() bool {
	return os.Getenv("VERCEL") != ""
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// ProjectRoot mencari folder akar proyek (yang memuat go.mod / web / bin).
// Bisa dipaksa lewat environment variable POS_ROOT.
func ProjectRoot() string {
	if root := os.Getenv("POS_ROOT"); root != "" {
		return root
	}
	candidates := []string{}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, wd, filepath.Dir(wd), filepath.Dir(filepath.Dir(wd)))
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates, dir, filepath.Dir(dir))
	}
	for _, dir := range candidates {
		if fileExists(filepath.Join(dir, "go.mod")) || dirExists(filepath.Join(dir, "web")) {
			return dir
		}
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return "."
}

// GetWorkDir mengembalikan direktori kerja untuk file CSV & laporan:
// /tmp di Vercel (filesystem lain read-only), data/ saat berjalan lokal.
func GetWorkDir() string {
	if IsVercel() {
		return "/tmp"
	}
	dir := filepath.Join(ProjectRoot(), "data")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return os.TempDir()
	}
	return dir
}

// TransactionsCSVPath adalah file yang diekspor Go untuk dibaca C.
func TransactionsCSVPath() string {
	return filepath.Join(GetWorkDir(), "transactions.csv")
}

// ProcessedCSVPath adalah file hasil C yang dibaca COBOL.
func ProcessedCSVPath() string {
	return filepath.Join(GetWorkDir(), "processed_data.csv")
}

// ReportOutputPath adalah file laporan hasil COBOL.
func ReportOutputPath() string {
	return filepath.Join(GetWorkDir(), "report_output.txt")
}

// WebDir mengembalikan folder berisi file statis frontend.
func WebDir() string {
	return filepath.Join(ProjectRoot(), "web")
}

// BinaryName menambahkan ekstensi .exe secara otomatis di Windows.
func BinaryName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

// ErrBinaryNotFound dikembalikan bila binary C/COBOL belum dikompilasi.
var ErrBinaryNotFound = errors.New("binary tidak ditemukan")

// isLinuxAMD64ELF rejects empty sentinels, non-Linux files, and binaries
// built for a different ELF class, byte order, or machine architecture.
func isLinuxAMD64ELF(data []byte) bool {
	return len(data) >= 20 &&
		bytes.Equal(data[:4], []byte{0x7f, 'E', 'L', 'F'}) &&
		data[4] == 2 && data[5] == 1 && data[18] == 0x3e && data[19] == 0
}

// writeEmbeddedBinaryAtomically extracts an embedded Linux executable into
// /tmp, sets mode 0755, and atomically replaces the stable runtime path.
func writeEmbeddedBinaryAtomically(data []byte, base string) (string, error) {
	if !isLinuxAMD64ELF(data) {
		return "", ErrBinaryNotFound
	}
	target := filepath.Join("/tmp", "pos-"+base)
	temp, err := os.CreateTemp("/tmp", "pos-"+base+"-")
	if err != nil {
		return "", err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := io.Copy(temp, bytes.NewReader(data)); err != nil {
		temp.Close()
		return "", err
	}
	if err := temp.Close(); err != nil {
		return "", err
	}
	if err := os.Chmod(tempPath, 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(tempPath, target); err != nil {
		return "", err
	}
	info, err := os.Stat(target)
	if err != nil {
		return "", err
	}
	if info.Mode().Perm()&0o111 == 0 {
		return "", errors.New("binary embedded tidak memiliki izin eksekusi")
	}
	return target, nil
}

// FindBinary mencari binary engine ("processor" atau "reporter").
// Urutan: env override (envVar); di Vercel, binary Linux amd64 lebih dulu;
// lalu bin/ dan folder sumber (c/ atau cobol/). Di lokal, binary native lebih dulu.
// Di Vercel, binary disalin ke /tmp dan diberi izin eksekusi.
func FindBinary(base, envVar, sourceDir string) (string, error) {
	root := ProjectRoot()
	name := BinaryName(base)

	candidates := []string{}
	override := os.Getenv(envVar)
	if override != "" && (!IsVercel() || fileExists(override)) {
		candidates = append(candidates, override)
	}
	if IsVercel() && runtime.GOOS == "linux" && runtime.GOARCH == "amd64" {
		if (override == "" || !fileExists(override)) {
			if embedded := embeddedLinuxBinary(base); isLinuxAMD64ELF(embedded) {
				return writeEmbeddedBinaryAtomically(embedded, base)
			}
		}
		// Cache belum terisi/valid? Coba artifact yang ikut dibundel di bin/.
		candidates = append(candidates, filepath.Join(root, "bin", base+"-linux-amd64"))
	}
	candidates = append(candidates,
		filepath.Join(root, "bin", name),
		filepath.Join(root, sourceDir, name),
	)
	if !IsVercel() && runtime.GOOS == "linux" && runtime.GOARCH == "amd64" {
		candidates = append(candidates, filepath.Join(root, "bin", base+"-linux-amd64"))
	}

	for _, candidate := range candidates {
		if !fileExists(candidate) {
			continue
		}
		if IsVercel() {
			return prepareOnVercel(candidate, base)
		}
		return candidate, nil
	}
	return "", ErrBinaryNotFound
}

// prepareOnVercel menyalin binary ke /tmp agar pasti punya izin eksekusi.
func prepareOnVercel(source, base string) (string, error) {
	target := filepath.Join("/tmp", "pos-"+base)
	in, err := os.Open(source)
	if err != nil {
		return "", err
	}
	defer in.Close()

	out, err := os.CreateTemp("/tmp", "pos-"+base+"-")
	if err != nil {
		return "", err
	}
	tempPath := out.Name()
	defer os.Remove(tempPath)
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return "", err
	}
	if err := out.Close(); err != nil {
		return "", err
	}
	if err := os.Chmod(tempPath, 0o755); err != nil {
		return "", err
	}
	// Atomic replacement avoids running a partially copied binary when two
	// serverless requests initialize the same engine concurrently.
	if err := os.Rename(tempPath, target); err != nil {
		return "", err
	}
	info, err := os.Stat(target)
	if err != nil {
		return "", err
	}
	if info.Mode().Perm()&0o111 == 0 {
		return "", errors.New("binary serverless tidak memiliki izin eksekusi")
	}
	return target, nil
}
