package config

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Paths describes writable application data and read-only resources. Embedded
// is optional: callers can pass an embed.FS (or any fs.FS) and OpenResource
// will use a disk file first, falling back to that filesystem.
type Paths struct {
	DataDir     string
	ResourceDir string
	Embedded    fs.FS
}

// ResolvePaths resolves paths using FARM_DATA_DIR and the executable location.
// FARM_RESOURCE_DIR is an optional disk override for resources that otherwise
// come from the embedded asset filesystem.
func ResolvePaths() Paths {
	executable, err := os.Executable()
	if err != nil {
		executable = ""
	}
	return ResolvePathsForExecutable(executable, nil)
}

// ResolvePathsForExecutable is deterministic and is intended for tests and
// launchers that already know their executable path. An empty path falls back
// to the current working directory.
func ResolvePathsForExecutable(executable string, embedded fs.FS) Paths {
	root := executableDir(executable)
	dataDir := os.Getenv("FARM_DATA_DIR")
	if dataDir == "" {
		dataDir = filepath.Join(root, "data")
	} else {
		dataDir = absoluteClean(dataDir)
	}
	resourceDir := os.Getenv("FARM_RESOURCE_DIR")
	if resourceDir == "" {
		resourceDir = root
	} else {
		resourceDir = absoluteClean(resourceDir)
	}
	return Paths{DataDir: dataDir, ResourceDir: resourceDir, Embedded: embedded}
}

// NewPaths creates a path set explicitly. Empty data/resource paths are filled
// from ResolvePaths, which makes it convenient to provide only an embedded FS.
func NewPaths(dataDir, resourceDir string, embedded fs.FS) Paths {
	paths := ResolvePathsForExecutable("", embedded)
	if dataDir != "" {
		paths.DataDir = absoluteClean(dataDir)
	}
	if resourceDir != "" {
		paths.ResourceDir = absoluteClean(resourceDir)
	}
	return paths
}

// DataFile returns a filename below the writable data directory.
func (p Paths) DataFile(filename string) string {
	return filepath.Join(p.DataDir, filename)
}

// ResourcePath returns the disk override path for a resource. For embedded
// resources, use OpenResource because there is no meaningful disk path.
func (p Paths) ResourcePath(segments ...string) string {
	parts := append([]string{p.ResourceDir}, segments...)
	return filepath.Join(parts...)
}

// EnsureDataDir creates the writable data directory if needed.
func (p Paths) EnsureDataDir() error {
	return os.MkdirAll(p.DataDir, 0o755)
}

// OpenResource opens a resource from the disk override when present, then
// falls back to the embedded filesystem. Names are slash-separated and may
// not escape the resource root.
func (p Paths) OpenResource(name string) (fs.File, error) {
	cleanName, err := cleanResourceName(name)
	if err != nil {
		return nil, err
	}
	diskPath := filepath.Join(p.ResourceDir, filepath.FromSlash(cleanName))
	if file, openErr := os.Open(diskPath); openErr == nil {
		return file, nil
	} else if !os.IsNotExist(openErr) {
		return nil, openErr
	}
	if p.Embedded != nil {
		return p.Embedded.Open(cleanName)
	}
	return nil, os.ErrNotExist
}

// GetDataDir mirrors the legacy runtime-paths helper.
func GetDataDir() string { return ResolvePaths().DataDir }

// GetResourcePath mirrors the legacy runtime-paths helper.
func GetResourcePath(segments ...string) string { return ResolvePaths().ResourcePath(segments...) }

// GetDataFile mirrors the legacy runtime-paths helper.
func GetDataFile(filename string) string { return ResolvePaths().DataFile(filename) }

// EnsureDataDir mirrors the legacy runtime-paths helper.
func EnsureDataDir() (string, error) {
	paths := ResolvePaths()
	return paths.DataDir, paths.EnsureDataDir()
}

// GetShareFilePath preserves the old placement: an explicit FARM_DATA_DIR
// stores share.txt inside that directory, while the default stores it beside
// the executable.
func GetShareFilePath() string {
	paths := ResolvePaths()
	if os.Getenv("FARM_DATA_DIR") != "" {
		return paths.DataFile("share.txt")
	}
	return filepath.Join(filepath.Dir(paths.DataDir), "share.txt")
}

func executableDir(executable string) string {
	if executable == "" {
		cwd, err := os.Getwd()
		if err == nil {
			return cwd
		}
		return "."
	}
	abs := absoluteClean(executable)
	return filepath.Dir(abs)
}

func absoluteClean(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(abs)
}

func cleanResourceName(name string) (string, error) {
	name = filepath.ToSlash(strings.TrimSpace(name))
	name = strings.TrimPrefix(name, "./")
	if name == "" || name == "." || strings.HasPrefix(name, "/") || filepath.VolumeName(name) != "" {
		return "", fs.ErrInvalid
	}
	clean := pathCleanSlash(name)
	if clean == "" || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fs.ErrInvalid
	}
	return clean, nil
}

func pathCleanSlash(name string) string {
	parts := strings.Split(name, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part {
		case "", ".":
			continue
		case "..":
			if len(out) > 0 {
				out = out[:len(out)-1]
			} else {
				out = append(out, part)
			}
		default:
			out = append(out, part)
		}
	}
	return strings.Join(out, "/")
}
