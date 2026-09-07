package auth

import (
	"errors"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"chakuchuri/backend/internal/platform/snapshot"
)

type Repository interface {
	LoadUsers() ([]UserRecord, string, error)
	SaveUsers([]UserRecord) error
	DeleteUser(User) error
}

type fileRepository struct {
	path string
}

type UserRecord struct {
	User         User   `json:"user"`
	Salt         string `json:"salt"`
	PasswordHash string `json:"passwordHash"`
}

type authSnapshot struct {
	Version int          `json:"version"`
	SavedAt string       `json:"savedAt"`
	Users   []UserRecord `json:"users"`
}

var errInvalidAuthSnapshot = errors.New("invalid auth snapshot")

func newFileRepository(dataDir string) (Repository, error) {
	if dataDir == "" {
		dataDir = "data"
	}
	if err := snapshot.EnsureDir(dataDir); err != nil {
		return nil, err
	}
	return fileRepository{path: filepath.Join(dataDir, "auth.users.dev.json")}, nil
}

func (r fileRepository) LoadUsers() ([]UserRecord, string, error) {
	var next authSnapshot
	loadedFrom, err := snapshot.LoadJSON(r.path, &next)
	if err != nil {
		return nil, "", err
	}
	if next.Version != 1 {
		return nil, "", errInvalidAuthSnapshot
	}
	return next.Users, loadedFrom, nil
}

func (r fileRepository) SaveUsers(users []UserRecord) error {
	return snapshot.SaveJSON(r.path, authSnapshot{
		Version: 1,
		SavedAt: time.Now().UTC().Format(time.RFC3339),
		Users:   users,
	})
}

func (r fileRepository) DeleteUser(User) error {
	return nil
}

func (s *Service) EnablePersistence(dataDir string) {
	repository, err := newFileRepository(dataDir)
	if err != nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.recordSystem("auth.persistence_error", "Could not create auth repository: "+err.Error())
		return
	}
	s.EnableRepository(repository)
}

func (s *Service) EnableRepository(repository Repository) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.repository = repository

	loadedFrom, err := s.loadLocked()
	if err == nil {
		s.recordSystem("auth.persistence_loaded", "Auth users loaded from "+loadedFrom+".")
		return
	}
	if !errors.Is(err, snapshot.ErrNotFound) {
		s.recordSystem("auth.persistence_ignored", "Auth repository could not be loaded: "+err.Error())
	}
	s.persistLocked()
}

func (s *Service) loadLocked() (string, error) {
	if s.repository == nil {
		return "", snapshot.ErrNotFound
	}

	rows, loadedFrom, err := s.repository.LoadUsers()
	if err != nil {
		return "", err
	}

	users := map[string]storedUser{}
	for _, row := range rows {
		key := strings.ToLower(strings.TrimSpace(row.User.Email))
		if key == "" {
			continue
		}
		row.User.Email = key
		users[key] = storedUser{User: normalizeLoadedUser(row.User), salt: row.Salt, passwordHash: row.PasswordHash}
	}
	if len(users) == 0 {
		return "", errInvalidAuthSnapshot
	}
	s.users = users
	return loadedFrom, nil
}

func (s *Service) persistLocked() {
	if s.repository == nil {
		return
	}

	emails := make([]string, 0, len(s.users))
	for email := range s.users {
		emails = append(emails, email)
	}
	sort.Strings(emails)

	rows := make([]UserRecord, 0, len(emails))
	for _, email := range emails {
		row := s.users[email]
		rows = append(rows, UserRecord{User: row.User, Salt: row.salt, PasswordHash: row.passwordHash})
	}

	if err := s.repository.SaveUsers(rows); err != nil {
		s.recordSystem("auth.persistence_error", "Could not save auth users: "+err.Error())
	}
}

func (s *Service) recordSystem(action string, detail string) {
	if s.recorder != nil {
		s.recorder.Record("system", action, "auth", detail)
	}
}
