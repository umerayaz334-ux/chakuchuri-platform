package workflow

import (
	"errors"
	"fmt"
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

type State struct {
	Version         int                  `json:"version"`
	SavedAt         string               `json:"savedAt"`
	Products        []Product            `json:"products"`
	Quotations      []Quotation          `json:"quotations"`
	Manufacturing   []ManufacturingOrder `json:"manufacturing"`
	RateSheets      []RateSheet          `json:"rateSheets"`
	Shipping        []ShippingRequest    `json:"shipping"`
	Payments        []Payment            `json:"payments"`
	Ledger          []LedgerEntry        `json:"ledger"`
	Conversations   []Conversation       `json:"conversations"`
	Calls           []CallRequest        `json:"calls"`
	CallSignals     []CallSignal         `json:"callSignals"`
	Notices         []CustomerNotice     `json:"notices"`
	Featured        []FeaturedProduct    `json:"featuredProducts"`
	Settings        PlatformSettings     `json:"settings"`
	LastSequenceDay string               `json:"lastSequenceDay"`
	Sequence        int                  `json:"sequence"`
}

var errInvalidWorkflowSnapshot = errors.New("invalid workflow snapshot")

func newFileRepository(dataDir string) (Repository, error) {
	if dataDir == "" {
		dataDir = "data"
	}
	if err := snapshot.EnsureDir(dataDir); err != nil {
		return nil, err
	}
	return fileRepository{path: filepath.Join(dataDir, "workflow.dev.json")}, nil
}

func (r fileRepository) LoadState() (State, string, error) {
	var next State
	loadedFrom, err := snapshot.LoadJSON(r.path, &next)
	if err != nil {
		return State{}, "", err
	}
	if next.Version != 1 {
		return State{}, "", errInvalidWorkflowSnapshot
	}
	return next, loadedFrom, nil
}

func (r fileRepository) SaveState(state State) error {
	state.Version = 1
	state.SavedAt = time.Now().UTC().Format(time.RFC3339)
	return snapshot.SaveJSON(r.path, state)
}

func (s *Service) EnablePersistence(dataDir string) error {
	repository, err := newFileRepository(dataDir)
	if err != nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.recordSystem("workflow.persistence_error", "Could not create workflow repository: "+err.Error())
		return fmt.Errorf("create workflow repository: %w", err)
	}
	return s.EnableRepository(repository)
}

func (s *Service) EnableRepository(repository Repository) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if repository == nil {
		return errors.New("workflow repository is required")
	}
	s.repository = repository

	loadedFrom, err := s.loadLocked()
	if err == nil {
		s.recordSystem("workflow.persistence_loaded", "Workflow state loaded from "+loadedFrom+".")
		return nil
	}
	// Only a missing local development file may bootstrap demo data.
	if _, local := repository.(fileRepository); local && errors.Is(err, snapshot.ErrNotFound) {
		return s.saveStateLocked()
	}
	s.recordSystem("workflow.persistence_error", "Workflow repository could not be loaded: "+err.Error())
	return fmt.Errorf("load workflow repository: %w", err)
}

func (s *Service) loadLocked() (string, error) {
	if s.repository == nil {
		return "", snapshot.ErrNotFound
	}

	state, loadedFrom, err := s.repository.LoadState()
	if err != nil {
		return "", err
	}
	s.products = state.Products
	s.quotations = state.Quotations
	s.manufacturing = state.Manufacturing
	s.rateSheets = state.RateSheets
	s.shipping = state.Shipping
	s.payments = state.Payments
	s.ledger = mergedLedgerEntries(state.Ledger, state.Manufacturing, state.Shipping, state.Payments)
	s.conversations = make([]Conversation, 0, len(state.Conversations))
	for _, conversation := range state.Conversations {
		s.conversations = append(s.conversations, normalizeConversation(conversation))
	}
	s.calls = append([]CallRequest(nil), state.Calls...)
	s.callSignals = append([]CallSignal(nil), state.CallSignals...)
	s.notices = append([]CustomerNotice(nil), state.Notices...)
	s.featured = append([]FeaturedProduct(nil), state.Featured...)
	if state.Settings.ThemePreset == "" {
		s.settings = defaultPlatformSettings()
	} else if settings, settingsErr := normalizePlatformSettings(state.Settings); settingsErr == nil {
		s.settings = settings
	} else {
		s.settings = defaultPlatformSettings()
		s.recordSystem("workflow.settings_invalid", "Stored platform settings were invalid; defaults were restored.")
	}
	for index := range s.calls {
		s.calls[index] = normalizeCall(s.calls[index])
	}
	s.expireRingingCallsLocked(time.Now().UTC())
	s.lastSequenceDay = state.LastSequenceDay
	s.sequence = state.Sequence
	return loadedFrom, nil
}

func (s *Service) persistLocked() {
	_ = s.saveStateLocked()
}

var errPersistence = errors.New("workflow storage unavailable")

func (s *Service) saveStateLocked() error {
	if s.repository == nil {
		return nil
	}

	if err := s.repository.SaveState(State{
		Products:        s.products,
		Quotations:      s.quotations,
		Manufacturing:   s.manufacturing,
		RateSheets:      s.rateSheets,
		Shipping:        s.shipping,
		Payments:        s.payments,
		Ledger:          append([]LedgerEntry(nil), s.ledger...),
		Conversations:   s.conversations,
		Calls:           s.calls,
		CallSignals:     trimCallSignals(s.callSignals, 500),
		Notices:         s.notices,
		Featured:        s.featured,
		Settings:        copyPlatformSettings(s.settings),
		LastSequenceDay: s.lastSequenceDay,
		Sequence:        s.sequence,
	}); err != nil {
		s.recordSystem("workflow.persistence_error", "Could not save workflow state: "+err.Error())
		return fmt.Errorf("%w: %w", errPersistence, err)
	}
	return nil
}

// App Home mutations publish only after saving, with memory restored on failure.
// Callers must hold s.mu and pass new slices rather than editing existing ones.
func (s *Service) commitAppHomeLocked(notices []CustomerNotice, featured []FeaturedProduct) error {
	oldNotices, oldFeatured := s.notices, s.featured
	s.notices, s.featured = notices, featured
	if err := s.saveStateLocked(); err != nil {
		s.notices, s.featured = oldNotices, oldFeatured
		return err
	}
	return nil
}

func (s *Service) recordSystem(action string, detail string) {
	if s.recorder != nil {
		s.recorder.Record("system", action, "workflow", detail)
	}
}
