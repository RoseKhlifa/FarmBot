package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/app"
	"github.com/RoseKhlifa/FarmBot/internal/backup"
	"github.com/RoseKhlifa/FarmBot/internal/config"
	"github.com/RoseKhlifa/FarmBot/internal/store"
)

// version is overridden by release builds with -ldflags.
var version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "backup":
			if err := runBackupCommand(os.Args[2:]); err != nil {
				fmt.Fprintln(os.Stderr, "backup failed:", err)
				os.Exit(1)
			}
			return
		case "export":
			if err := runExportCommand(os.Args[2:]); err != nil {
				fmt.Fprintln(os.Stderr, "export failed:", err)
				os.Exit(1)
			}
			return
		}
	}
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

func runBackupCommand(args []string) error {
	flags := flag.NewFlagSet("backup", flag.ContinueOnError)
	dataDir, timestamp := "", ""
	keep := flags.Int("keep", 0, "number of snapshots to retain (0 disables pruning)")
	flags.StringVar(&dataDir, "data-dir", "", "SQLite data directory")
	flags.StringVar(&timestamp, "timestamp", "", "external snapshot timestamp")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(timestamp) == "" {
		return fmt.Errorf("--timestamp is required")
	}
	cfg := config.Load()
	if dataDir != "" {
		cfg.DataDir = filepath.Clean(dataDir)
		cfg.Paths = config.NewPaths(cfg.DataDir, cfg.ResourceDir, nil)
	}
	db, err := store.Open(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	dir := filepath.Join(cfg.DataDir, "backups")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	destination := filepath.Join(dir, "farmbot-"+filepath.Base(timestamp)+".db")
	if err := (backup.Snapshotter{DB: db}).Snapshot(context.Background(), destination); err != nil {
		return err
	}
	if *keep > 0 {
		if err := backup.PruneSnapshots(dir, *keep); err != nil {
			return err
		}
	}
	fmt.Println(destination)
	return nil
}

func runExportCommand(args []string) error {
	flags := flag.NewFlagSet("export", flag.ContinueOnError)
	dataDir, accountID, output := "", "", ""
	flags.StringVar(&dataDir, "data-dir", "", "SQLite data directory")
	flags.StringVar(&accountID, "account-id", "", "account ID")
	flags.StringVar(&output, "output", "", "JSON output file")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(accountID) == "" || strings.TrimSpace(output) == "" {
		return fmt.Errorf("--account-id and --output are required")
	}
	cfg := config.Load()
	if dataDir != "" {
		cfg.DataDir = filepath.Clean(dataDir)
		cfg.Paths = config.NewPaths(cfg.DataDir, cfg.ResourceDir, nil)
	}
	db, err := store.Open(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	data, err := backup.ExportAccount(context.Background(), db, accountID)
	if err != nil {
		return err
	}
	if err := os.WriteFile(output, append(data, '\n'), 0o600); err != nil {
		return err
	}
	fmt.Println(output)
	return nil
}
