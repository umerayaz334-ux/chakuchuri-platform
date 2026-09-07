package files

import (
	"errors"
	"path/filepath"
	"time"

	"chakuchuri/backend/internal/platform/snapshot"
)

type Repository interface {
	LoadFiles() ([]FileRecord, string, error)
	SaveFiles([]FileRecord) error
	DeleteFile(FileRecord) error
}

// SQL repositories write just the new row; local JSON snapshots remain whole-file.
type recordWriter interface {
	SaveFile(FileRecord) error
}

type recordReader interface {
	GetFile(tenantID, id string) (FileRecord, bool, error)
	Check() error
}

func (s *Service) persistUploadLocked(record FileRecord) error {
	if repository, ok := s.repository.(recordWriter); ok {
		return repository.SaveFile(record)
	}
	return s.persistLocked()
}

type fileRepository struct {
	path string
}

type filesSnapshot struct {
	Version int          `json:"version"`
	SavedAt string       `json:"savedAt"`
	Files   []FileRecord `json:"files"`
}

var errInvalidFilesSnapshot = errors.New("invalid files snapshot")

func newFileRepository(dataDir string) (Repository, error) {
	if dataDir == "" {
		dataDir = "data"
	}
	if err := snapshot.EnsureDir(dataDir); err != nil {
		return nil, err
	}
	return fileRepository{path: filepath.Join(dataDir, "files.dev.json")}, nil
}

func (r fileRepository) LoadFiles() ([]FileRecord, string, error) {
	var next filesSnapshot
	loadedFrom, err := snapshot.LoadJSON(r.path, &next)
	if err != nil {
		return nil, "", err
	}
	if next.Version != 1 {
		return nil, "", errInvalidFilesSnapshot
	}
	return next.Files, loadedFrom, nil
}

func (r fileRepository) SaveFiles(records []FileRecord) error {
	return snapshot.SaveJSON(r.path, filesSnapshot{
		Version: 1,
		SavedAt: time.Now().UTC().Format(time.RFC3339),
		Files:   records,
	})
}

func (r fileRepository) DeleteFile(record FileRecord) error {
	records, _, err := r.LoadFiles()
	if err != nil {
		if errors.Is(err, snapshot.ErrNotFound) {
			return nil
		}
		return err
	}
	next := make([]FileRecord, 0, len(records))
	for _, row := range records {
		if row.ID != record.ID {
			next = append(next, row)
		}
	}
	return r.SaveFiles(next)
}

func (s *Service) EnablePersistence(dataDir string) {
	repository, err := newFileRepository(dataDir)
	if err != nil {
		s.recordSystem("files.persistence_error", "Could not create files repository: "+err.Error())
		return
	}
	s.EnableRepository(repository)
}

func (s *Service) EnableRepository(repository Repository) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.repository = repository
	if reader, ok := repository.(recordReader); ok {
		return reader.Check()
	}
	records, loadedFrom, err := repository.LoadFiles()
	if err == nil {
		s.records = records
		s.rebuildIndexLocked()
		s.recordSystem("files.persistence_loaded", "File records loaded from "+loadedFrom+".")
		return nil
	}
	if !errors.Is(err, snapshot.ErrNotFound) {
		s.recordSystem("files.persistence_ignored", "File repository could not be loaded: "+err.Error())
		return err
	}
	return s.persistLocked()
}

func (s *Service) persistLocked() error {
	if s.repository == nil {
		return nil
	}
	if err := s.repository.SaveFiles(s.records); err != nil {
		s.recordSystem("files.persistence_error", "Could not save file records: "+err.Error())
		return err
	}
	return nil
}

func (s *Service) rebuildIndexLocked() {
	s.byID = map[string]FileRecord{}
	for _, record := range s.records {
		s.byID[record.ID] = record
	}
}

func (s *Service) recordSystem(action string, detail string) {
	if s.recorder != nil {
		s.recorder.Record("system", action, "files", detail)
	}
}
