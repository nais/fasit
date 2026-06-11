package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"
)

var cfg = DefaultConfig()

func init() {
	flag.StringVar(&cfg.BindAddress, "bind-address", cfg.BindAddress, "address to listen on")
	flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "which log level to output")
	flag.Var(targets{routes: &cfg.Routes}, "target", "host=upstream route, repeatable")
}

func main() {
	flag.Parse()
	log := newLogger(cfg.LogLevel)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := run(ctx, log); err != nil {
		log.With("err", err).Error("fatal")
		os.Exit(1)
	}
}

func run(ctx context.Context, log *slog.Logger) error {
	handler, err := newHandler(cfg.Routes, log)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              cfg.BindAddress,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.With("err", err).Error("shutdown")
		}
	}()

	log.With("addr", cfg.BindAddress).Info("oidcproxy serving")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
