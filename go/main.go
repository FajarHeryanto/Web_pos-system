package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"pos-system/go/database"
	"pos-system/go/router"
)

func main() {
	_ = godotenv.Load()
	if len(os.Getenv("JWT_SECRET")) < 32 {
		log.Fatal("JWT_SECRET wajib berisi minimal 32 karakter acak. Lihat .env.example")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	if err := database.Init(ctx); err != nil {
		cancel()
		log.Fatalf("Inisialisasi Supabase gagal: %v", err)
	}
	cancel()
	defer database.Close()

	addr := strings.TrimSpace(os.Getenv("PORT"))
	if addr == "" {
		addr = "8080"
	}
	if !strings.HasPrefix(addr, ":") {
		addr = ":" + addr
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           router.New(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("POS berjalan di http://localhost%s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server gagal: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Shutdown server: %v", err)
	}
}
