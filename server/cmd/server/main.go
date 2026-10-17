package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"metro-buddy/server/internal/app"
	"metro-buddy/server/internal/config"
	"metro-buddy/server/internal/db"
	"metro-buddy/server/internal/logx"
	"metro-buddy/server/internal/redisx"
)

func main() {
	log := logx.New(os.Stdout, slog.LevelInfo)
	if err := run(log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	gdb, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	if err := db.Migrate(gdb); err != nil {
		return err
	}
	rdb, err := redisx.New(cfg.RedisURL)
	if err != nil {
		return err
	}
	defer rdb.Close()

	a, err := app.New(&app.Deps{Cfg: cfg, DB: gdb, Redis: rdb, Log: log})
	if err != nil {
		return err
	}
	e := a.Echo

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 1)
	go func() {
		log.Info("server listening", "port", cfg.Port, "env", cfg.Env)
		if err := e.Start(":" + cfg.Port); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return e.Shutdown(shutdownCtx)
}
