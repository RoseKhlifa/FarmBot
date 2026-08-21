package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func main() {
	if err := syncTree("web/dist", "assets/webdist"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func syncTree(source, destination string) error {
	if info, err := os.Stat(source); err != nil || !info.IsDir() {
		if err == nil {
			err = fmt.Errorf("source is not a directory")
		}
		return fmt.Errorf("web asset source %q: %w", source, err)
	}
	if err := os.RemoveAll(destination); err != nil {
		return fmt.Errorf("clear embedded web assets: %w", err)
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return fmt.Errorf("create embedded web assets: %w", err)
	}
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("unsupported web asset entry %q", path)
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = input.Close() }()
		output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(output, input)
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}
