package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStorageAndWebResolveBesideConfig(t *testing.T) {
	dir := t.TempDir()
	for _, key := range []string{"SI_STORAGE_PATH", "SI_WEB_PATH"} {
		t.Setenv(key, "")
	}
	c, err := Load(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if c.StoragePath != filepath.Join(dir, "storage") || c.WebPath != filepath.Join(dir, "web") {
		t.Fatalf("paths depend on working directory: %+v", c)
	}
	if err = os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("storage_path: data\nweb_path: assets\n"), 0600); err != nil {
		t.Fatal(err)
	}
	c, err = Load(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if c.StoragePath != filepath.Join(dir, "data") || c.WebPath != filepath.Join(dir, "assets") {
		t.Fatal(c)
	}
	absolute := filepath.Join(t.TempDir(), "storage")
	t.Setenv("SI_STORAGE_PATH", absolute)
	c, err = Load(filepath.Join(dir, "config.yaml"))
	if err != nil || c.StoragePath != absolute {
		t.Fatalf("absolute override: %v %+v", err, c)
	}
}
