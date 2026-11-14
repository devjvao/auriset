package config

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
)

// Path resolves an application setting within the current user's config directory.
func Path(name string) string {
	dir, err := os.UserConfigDir()
	if err != nil {
		slog.Warn("could not locate user configuration directory", "error", err)
		return ""
	}
	return filepath.Join(dir, "Kuba Equalizer", name)
}

// WriteFile replaces a setting after writing and flushing its new contents.
func WriteFile(path string, data []byte) error {
	if path == "" {
		return errors.New("user configuration directory is unavailable")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".setting-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(file.Name()) }()
	_, err = file.Write(data)
	if err == nil {
		err = file.Sync()
	}
	if err = errors.Join(err, file.Close()); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
