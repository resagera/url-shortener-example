package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"shortener/internal/cache"
	"shortener/internal/config"
	"shortener/internal/logger"
	sqliterepo "shortener/internal/repo/sqlite"
	service "shortener/internal/service/shortener"
	httphandler "shortener/internal/web"
)

func main() {
	cfg := config.LoadConfig()

	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o755); err != nil {
		log.Fatalf("create db dir: %v", err)
	}

	db, err := sqliterepo.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	asyncH := logger.NewAsyncHandler(slog.NewTextHandler(os.Stdout, nil), 100)
	lg := slog.New(asyncH)

	repo := sqliterepo.New(db)

	if err := repo.Migrate(context.Background()); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	c := cache.NewURLCache(100_000)
	svc := service.NewURLService(repo, c, lg)

	h := httphandler.NewHandler(svc, lg)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	srv := &http.Server{
		Addr:    cfg.ServerPort,
		Handler: mux,
	}

	go func() {
		log.Printf("Listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("server shutdown: %v", err)
	}
}
