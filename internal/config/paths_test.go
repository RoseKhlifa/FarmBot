package config

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	testingfst "testing/fstest"
)

func TestResolvePathsUsesExecutableDirectoryByDefault(t *testing.T) {
	t.Setenv("FARM_DATA_DIR", "")
	t.Setenv("FARM_RESOURCE_DIR", "")
	executable := filepath.Join(t.TempDir(), "farmbot")
	root := filepath.Dir(executable)
	paths := ResolvePathsForExecutable(executable, nil)
	if paths.DataDir != filepath.Join(root, "data") {
		t.Fatalf("DataDir = %q, want %q", paths.DataDir, filepath.Join(root, "data"))
	}
	if paths.ResourceDir != root {
		t.Fatalf("ResourceDir = %q, want %q", paths.ResourceDir, root)
	}
}

func TestResolvePathsEnvironmentOverrides(t *testing.T) {
	t.Setenv("FARM_DATA_DIR", `relative/data`)
	t.Setenv("FARM_RESOURCE_DIR", `relative/resources`)
	paths := ResolvePathsForExecutable(`/opt/farmbot/farmbot`, nil)
	if !isAbsolute(paths.DataDir) || !isAbsolute(paths.ResourceDir) {
		t.Fatalf("overrides should be absolute: %#v", paths)
	}
	if paths.DataFile("users.json") != join(paths.DataDir, "users.json") {
		t.Fatalf("DataFile did not join below DataDir")
	}
}

func TestOpenResourcePrefersDiskAndFallsBackToEmbedded(t *testing.T) {
	disk := t.TempDir()
	if err := writeFile(join(disk, "config", "disk.txt"), "disk"); err != nil {
		t.Fatal(err)
	}
	embedded := testingfst.MapFS{
		"config/embedded.txt": &testingfst.MapFile{Data: []byte("embedded")},
		"config/disk.txt":     &testingfst.MapFile{Data: []byte("embedded fallback")},
	}
	paths := NewPaths(t.TempDir(), disk, embedded)
	file, err := paths.OpenResource("config/disk.txt")
	if err != nil {
		t.Fatal(err)
	}
	data, err := readOpened(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "disk" {
		t.Fatalf("disk override = %q, want disk", data)
	}
	_ = file.Close()

	file, err = paths.OpenResource("config/embedded.txt")
	if err != nil {
		t.Fatal(err)
	}
	data, err = readOpened(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "embedded" {
		t.Fatalf("embedded resource = %q, want embedded", data)
	}
	_ = file.Close()
}

func TestOpenResourceRejectsTraversal(t *testing.T) {
	paths := NewPaths(t.TempDir(), t.TempDir(), testingfst.MapFS{})
	for _, name := range []string{"../secret", "a/../"} {
		if _, err := paths.OpenResource(name); err != fs.ErrInvalid {
			t.Fatalf("traversal %q error = %v, want fs.ErrInvalid", name, err)
		}
	}
}

func readOpened(file fs.File) ([]byte, error) {
	defer file.Close()
	return io.ReadAll(file)
}

func writeFile(filename, contents string) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		return err
	}
	return os.WriteFile(filename, []byte(contents), 0o644)
}

func join(parts ...string) string { return filepath.Join(parts...) }

func isAbsolute(path string) bool { return filepath.IsAbs(path) }
