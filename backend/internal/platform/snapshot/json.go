package snapshot

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

var ErrNotFound = errors.New("snapshot not found")

func EnsureDir(dir string) error {
	if dir == "" {
		dir = "data"
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create snapshot directory: %w", err)
	}
	return nil
}

func LoadJSON(path string, target interface{}) (string, error) {
	source, err := loadFrom(path, target)
	if err == nil {
		return source, nil
	}
	if errors.Is(err, ErrNotFound) {
		return "", err
	}

	backupPath := path + ".bak"
	source, backupErr := loadFrom(backupPath, target)
	if backupErr == nil {
		return source, nil
	}
	return "", err
}

func SaveJSON(path string, payload interface{}) error {
	if path == "" {
		return nil
	}
	if err := EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}

	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("encode snapshot: %w", err)
	}

	tempPath := fmt.Sprintf("%s.%s.tmp", path, time.Now().UTC().Format("20060102150405000000000"))
	if err := writeSynced(tempPath, body); err != nil {
		return fmt.Errorf("write snapshot temp file: %w", err)
	}

	backupPath := path + ".bak"
	if _, err := os.Stat(path); err == nil {
		_ = os.Remove(backupPath)
		if err := os.Rename(path, backupPath); err != nil {
			_ = os.Remove(tempPath)
			return fmt.Errorf("backup previous snapshot: %w", err)
		}
	}

	if err := os.Rename(tempPath, path); err != nil {
		_ = restoreBackup(path, backupPath)
		_ = os.Remove(tempPath)
		return fmt.Errorf("replace snapshot: %w", err)
	}
	return nil
}

func loadFrom(path string, target interface{}) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("read snapshot: %w", err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		return "", fmt.Errorf("decode snapshot: %w", err)
	}
	return path, nil
}

func writeSynced(path string, body []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := file.Write(body); err != nil {
		return err
	}
	return file.Sync()
}

func restoreBackup(path string, backupPath string) error {
	source, err := os.Open(backupPath)
	if err != nil {
		return err
	}
	defer source.Close()

	target, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer target.Close()

	_, err = io.Copy(target, source)
	return err
}
