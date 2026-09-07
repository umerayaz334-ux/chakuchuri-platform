package directory

import (
	"errors"
	"path/filepath"
	"time"

	"chakuchuri/backend/internal/platform/snapshot"
)

type Repository interface {
	LoadState() (State, string, error)
	SaveState(State) error
}

type fileRepository struct {
	path string
}

var errInvalidSnapshot = errors.New("invalid directory snapshot")

func newFileRepository(dataDir string) (Repository, error) {
	if dataDir == "" {
		dataDir = "data"
	}
	if err := snapshot.EnsureDir(dataDir); err != nil {
		return nil, err
	}
	return fileRepository{path: filepath.Join(dataDir, "directory.dev.json")}, nil
}

func (r fileRepository) LoadState() (State, string, error) {
	var next State
	loadedFrom, err := snapshot.LoadJSON(r.path, &next)
	if err != nil {
		return State{}, "", err
	}
	if next.Version != 1 {
		return State{}, "", errInvalidSnapshot
	}
	if next.Categories == nil {
		next.Categories = []Category{}
	}
	if next.Listings == nil {
		next.Listings = []Listing{}
	}
	return next, loadedFrom, nil
}

func (r fileRepository) SaveState(state State) error {
	state.Version = 1
	if state.Categories == nil {
		state.Categories = []Category{}
	}
	if state.Listings == nil {
		state.Listings = []Listing{}
	}
	return snapshot.SaveJSON(r.path, struct {
		Version    int        `json:"version"`
		SavedAt    string     `json:"savedAt"`
		Categories []Category `json:"categories"`
		Listings   []Listing  `json:"listings"`
	}{
		Version:    1,
		SavedAt:    time.Now().UTC().Format(time.RFC3339),
		Categories: state.Categories,
		Listings:   state.Listings,
	})
}

func (s *Service) EnablePersistence(dataDir string) {
	repository, err := newFileRepository(dataDir)
	if err != nil {
		s.recordSystem("directory.persistence_error", "Could not create directory repository: "+err.Error())
		return
	}
	s.EnableRepository(repository)
}

func (s *Service) EnableRepository(repository Repository) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if repository == nil {
		s.recordSystem("directory.persistence_error", "Directory repository is nil.")
		return
	}
	s.repository = repository

	state, loadedFrom, err := repository.LoadState()
	if err == nil {
		if len(state.Categories) > 0 {
			s.categories = state.Categories
		}
		s.listings = state.Listings
		if len(s.listings) == 0 {
			s.listings = seedListings(s.categories)
			s.persistLocked()
			s.recordSystem("directory.seeded", "Seeded sample directory listings.")
		}
		s.recordSystem("directory.persistence_loaded", "Directory loaded from "+loadedFrom+".")
		return
	}
	if !errors.Is(err, snapshot.ErrNotFound) && !errors.Is(err, errInvalidSnapshot) {
		s.recordSystem("directory.persistence_ignored", "Directory repository could not be loaded: "+err.Error())
	}
	s.persistLocked()
}
