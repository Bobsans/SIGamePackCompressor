package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"sigamepackcompressor/internal/config"
	"sigamepackcompressor/internal/server"
)

var version = "dev"

func main() {
	executable, err := os.Executable()
	if err != nil {
		slog.Error("executable path", "error", err)
		os.Exit(1)
	}
	filename := flag.String("config", filepath.Join(filepath.Dir(executable), "config.yaml"), "Configuration file")
	printVersion := flag.Bool("version", false, "Print build version")
	flag.Parse()
	if *printVersion {
		fmt.Println(version)
		return
	}
	cfg, err := config.Load(*filename)
	if err != nil {
		slog.Error("configuration", "error", err)
		os.Exit(1)
	}
	cfg.BuildVersion = version
	s, err := server.New(cfg)
	if err != nil {
		slog.Error("initialize", "error", err)
		os.Exit(1)
	}
	defer s.Close()
	httpServer := &http.Server{Addr: cfg.Listen, Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdown)
	}()
	slog.Info("listening", "address", cfg.Listen, "storage", cfg.StoragePath)
	if err = httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("serve", "error", err)
		os.Exit(1)
	}
}
