package customers

import (
	"errors"
	"path/filepath"
	"time"

	"chakuchuri/backend/internal/platform/snapshot"
)

type Repository interface {
	LoadCustomers() ([]Customer, []DocumentRequest, string, error)
	SaveCustomers([]Customer, []DocumentRequest) error
}

type fileRepository struct {
	path string
}

type customersSnapshot struct {
	Version          int               `json:"version"`
	SavedAt          string            `json:"savedAt"`
	Customers        []Customer        `json:"customers"`
	DocumentRequests []DocumentRequest `json:"documentRequests,omitempty"`
}

var errInvalidCustomersSnapshot = errors.New("invalid customers snapshot")

func newFileRepository(dataDir string) (Repository, error) {
	if dataDir == "" {
		dataDir = "data"
	}
	if err := snapshot.EnsureDir(dataDir); err != nil {
		return nil, err
	}
	return fileRepository{path: filepath.Join(dataDir, "customers.dev.json")}, nil
}

func (r fileRepository) LoadCustomers() ([]Customer, []DocumentRequest, string, error) {
	var next customersSnapshot
	loadedFrom, err := snapshot.LoadJSON(r.path, &next)
	if err != nil {
		return nil, nil, "", err
	}
	if next.Version != 1 || len(next.Customers) == 0 {
		return nil, nil, "", errInvalidCustomersSnapshot
	}
	requests := next.DocumentRequests
	if requests == nil {
		requests = []DocumentRequest{}
	}
	return next.Customers, requests, loadedFrom, nil
}

func (r fileRepository) SaveCustomers(customers []Customer, requests []DocumentRequest) error {
	if requests == nil {
		requests = []DocumentRequest{}
	}
	safeRequests := append([]DocumentRequest(nil), requests...)
	for i := range safeRequests {
		if safeRequests[i].TokenHash == "" && safeRequests[i].Token != "" {
			safeRequests[i].TokenHash = hashRequestToken(safeRequests[i].Token)
		}
		safeRequests[i].Token = ""
	}
	return snapshot.SaveJSON(r.path, customersSnapshot{
		Version:          1,
		SavedAt:          time.Now().UTC().Format(time.RFC3339),
		Customers:        customers,
		DocumentRequests: safeRequests,
	})
}

func (s *Service) EnablePersistence(dataDir string) {
	repository, err := newFileRepository(dataDir)
	if err != nil {
		s.recordSystem("customers.persistence_error", "Could not create customers repository: "+err.Error())
		return
	}
	s.EnableRepository(repository)
}

func (s *Service) EnableRepository(repository Repository) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if repository == nil {
		s.recordSystem("customers.persistence_error", "Customer repository is nil.")
		return
	}
	s.repository = repository

	loadedFrom, err := s.loadLocked()
	if err == nil {
		s.recordSystem("customers.persistence_loaded", "Customers loaded from "+loadedFrom+".")
		return
	}
	if !errors.Is(err, snapshot.ErrNotFound) {
		s.recordSystem("customers.persistence_ignored", "Customers repository could not be loaded: "+err.Error())
	}
	s.persistLocked()
}

func (s *Service) loadLocked() (string, error) {
	if s.repository == nil {
		return "", snapshot.ErrNotFound
	}

	customers, requests, loadedFrom, err := s.repository.LoadCustomers()
	if err != nil {
		return "", err
	}
	for i := range customers {
		customers[i] = normalizeCustomer(customers[i])
	}
	s.customers = customers
	s.documentRequests = normalizeLoadedDocumentRequests(customers, requests)
	return loadedFrom, nil
}

func (s *Service) persistLocked() {
	if s.repository == nil {
		return
	}

	if err := s.repository.SaveCustomers(s.customers, s.documentRequests); err != nil {
		s.recordSystem("customers.persistence_error", "Could not save customers snapshot: "+err.Error())
	}
}

func (s *Service) recordSystem(action string, detail string) {
	if s.recorder != nil {
		s.recorder.Record("system", action, "customers", detail)
	}
}
