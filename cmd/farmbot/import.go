package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/RoseKhlifa/FarmBot/internal/config"
	"github.com/RoseKhlifa/FarmBot/internal/store"
)

// The importer is an additive command so the existing composition root can
// remain untouched while the Node data is moved into SQLite.
func init() {
	args := os.Args[1:]
	if hasHelpFlag(args) {
		printImportHelp(os.Stdout)
		os.Exit(0)
	}
	if !hasImportFlag(args) {
		return
	}
	if err := runJSONImport(args); err != nil {
		fmt.Fprintln(os.Stderr, "JSON import failed:", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func hasImportFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--import-json" || strings.HasPrefix(arg, "--import-json=") {
			return true
		}
	}
	return false
}

func hasHelpFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

func runJSONImport(args []string) error {
	sourceDir, conflict, dataDir, err := parseImportArgs(args)
	if err != nil {
		return err
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
	defer db.Close()

	report, err := store.NewJSONImporter(db).Import(context.Background(), sourceDir, store.JSONImportOptions{Conflict: conflict})
	printImportReport(report)
	return err
}

func parseImportArgs(args []string) (sourceDir string, conflict store.ImportConflict, dataDir string, err error) {
	conflict = store.ConflictSkip
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--import-json":
			if index+1 >= len(args) || strings.HasPrefix(args[index+1], "--") {
				return "", "", "", fmt.Errorf("--import-json requires a directory")
			}
			sourceDir = args[index+1]
			index++
		case strings.HasPrefix(arg, "--import-json="):
			sourceDir = strings.TrimPrefix(arg, "--import-json=")
		case arg == "--conflict":
			if index+1 >= len(args) {
				return "", "", "", fmt.Errorf("--conflict requires skip or overwrite")
			}
			conflict = store.ImportConflict(strings.ToLower(strings.TrimSpace(args[index+1])))
			index++
		case strings.HasPrefix(arg, "--conflict="):
			conflict = store.ImportConflict(strings.ToLower(strings.TrimSpace(strings.TrimPrefix(arg, "--conflict="))))
		case arg == "--data-dir":
			if index+1 >= len(args) || strings.HasPrefix(args[index+1], "--") {
				return "", "", "", fmt.Errorf("--data-dir requires a directory")
			}
			dataDir = args[index+1]
			index++
		case strings.HasPrefix(arg, "--data-dir="):
			dataDir = strings.TrimPrefix(arg, "--data-dir=")
		}
	}
	if strings.TrimSpace(sourceDir) == "" {
		return "", "", "", fmt.Errorf("--import-json requires a directory")
	}
	if conflict != store.ConflictSkip && conflict != store.ConflictOverwrite {
		return "", "", "", fmt.Errorf("invalid conflict strategy %q: use skip or overwrite", conflict)
	}
	return filepath.Clean(sourceDir), conflict, dataDir, nil
}

func printImportHelp(output interface{ Write([]byte) (int, error) }) {
	_, _ = output.Write([]byte("FarmBot JSON migration\n\n" +
		"  --import-json <dir>       import legacy core/data JSON into SQLite\n" +
		"  --conflict skip|overwrite  keep existing rows or replace them (default: skip)\n" +
		"  --data-dir <dir>          SQLite data directory override\n" +
		"  --help                    show this help\n"))
}

func printImportReport(report store.JSONImportReport) {
	fmt.Printf("JSON import source: %s\n", report.SourceDir)
	keys := make([]string, 0, len(report.Counts))
	for key := range report.Counts {
		keys = append(keys, key)
	}
	for key := range report.Skipped {
		found := false
		for _, existing := range keys {
			if existing == key {
				found = true
				break
			}
		}
		if !found {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Printf("  %-18s imported=%d skipped=%d\n", key, report.Counts[key], report.Skipped[key])
	}
}
