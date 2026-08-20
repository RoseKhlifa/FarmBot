package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/app"
	"github.com/RoseKhlifa/FarmBot/internal/config"
)

// version is overridden by release builds with -ldflags.
var version = "dev"

func main() {
	log.Printf("farmbot %s starting", version)
	application, err := app.New(config.Load())
	if err != nil {
		log.Printf("farmbot startup failed: %v", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := application.Run(ctx); err != nil {
		log.Printf("farmbot server stopped: %v", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := application.Shutdown(shutdownCtx); err != nil {
		log.Printf("farmbot shutdown failed: %v", err)
	}
}
