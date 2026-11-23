package handler

import (
	"context"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"pos-system/go/database"
	"pos-system/go/router"
)

var (
	initOnce sync.Once
	initErr  error
	app      http.Handler
)

// Handler adalah entry point HTTP Serverless Function Vercel.
func Handler(w http.ResponseWriter, r *http.Request) {
	initOnce.Do(func() {
		// .env lokal opsional; di Vercel nilai datang dari Project Settings.
		_ = godotenv.Load()
		if len(os.Getenv("JWT_SECRET")) < 32 {
			initErr = context.DeadlineExceeded // tidak bocorkan konfigurasi rahasia
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := database.Init(ctx); err != nil {
			initErr = err
			return
		}
		app = router.New()
	})
	if initErr != nil || app == nil {
		http.Error(w, "POS belum siap. Periksa konfigurasi deployment.", http.StatusServiceUnavailable)
		return
	}
	app.ServeHTTP(w, r)
}
