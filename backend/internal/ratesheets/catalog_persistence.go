package ratesheets

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

type State struct {
	Version int        `json:"version"`
	SavedAt string     `json:"savedAt"`
	Books   []RateBook `json:"books"`
}

type fileRepository struct {
	path string
}

var errInvalidRateBookSnapshot = errors.New("invalid shipping rate snapshot")

func newFileRepository(dataDir string) (Repository, error) {
	if dataDir == "" {
		dataDir = "data"
	}
	if err := snapshot.EnsureDir(dataDir); err != nil {
		return nil, err
	}
	return fileRepository{path: filepath.Join(dataDir, "shipping-rates.dev.json")}, nil
}

func (r fileRepository) LoadState() (State, string, error) {
	var state State
	loadedFrom, err := snapshot.LoadJSON(r.path, &state)
	if err != nil {
		return State{}, "", err
	}
	if state.Version != 1 {
		return State{}, "", errInvalidRateBookSnapshot
	}
	return state, loadedFrom, nil
}

func (r fileRepository) SaveState(state State) error {
	state.Version = 1
	state.SavedAt = time.Now().UTC().Format(time.RFC3339)
	return snapshot.SaveJSON(r.path, state)
}

func (s *Service) EnablePersistence(dataDir string) {
	repository, err := newFileRepository(dataDir)
	if err != nil {
		s.recordSystem("shipping_rates.persistence_error", "Could not create shipping rate repository: "+err.Error())
		return
	}
	s.EnableRepository(repository)
}

func (s *Service) EnableRepository(repository Repository) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if repository == nil {
		s.recordSystem("shipping_rates.persistence_error", "Shipping rate repository is nil.")
		return
	}
	s.repository = repository
	state, loadedFrom, err := repository.LoadState()
	if err == nil {
		if state.Books == nil {
			s.books = []RateBook{}
		} else {
			s.books = state.Books
		}
		s.recordSystem("shipping_rates.persistence_loaded", "Shipping rates loaded from "+loadedFrom+".")
		return
	}
	if !errors.Is(err, snapshot.ErrNotFound) {
		s.recordSystem("shipping_rates.persistence_ignored", "Shipping rate repository could not be loaded: "+err.Error())
	}
	s.persistLocked()
}

func (s *Service) persistLocked() {
	if s.repository == nil {
		return
	}
	if err := s.repository.SaveState(State{Books: s.books}); err != nil {
		s.recordSystem("shipping_rates.persistence_error", "Could not save shipping rates: "+err.Error())
	}
}

func (s *Service) recordSystem(action string, detail string) {
	if s.recorder != nil {
		s.recorder.Record("system", action, "shipping-rates", detail)
	}
}
