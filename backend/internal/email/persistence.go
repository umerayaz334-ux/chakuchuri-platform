package email

import (
	"errors"
	"path/filepath"
	"strings"
	"time"

	"chakuchuri/backend/internal/platform/snapshot"
)

type Repository interface {
	LoadState() (State, string, error)
	SaveState(State) error
}

type State struct {
	Version    int              `json:"version"`
	SavedAt    string           `json:"savedAt"`
	Settings   SenderSettings   `json:"settings"`
	Templates  []Template       `json:"templates"`
	Rules      []AutomationRule `json:"rules"`
	Outbox     []OutboxItem     `json:"outbox"`
	Deliveries []Delivery       `json:"deliveries"`
}

type fileRepository struct {
	path string
}

var errInvalidEmailSnapshot = errors.New("invalid email snapshot")

func newFileRepository(dataDir string) (Repository, error) {
	if dataDir == "" {
		dataDir = "data"
	}
	if err := snapshot.EnsureDir(dataDir); err != nil {
		return nil, err
	}
	return fileRepository{path: filepath.Join(dataDir, "email.dev.json")}, nil
}

func (r fileRepository) LoadState() (State, string, error) {
	var state State
	loadedFrom, err := snapshot.LoadJSON(r.path, &state)
	if err != nil {
		return State{}, "", err
	}
	if state.Version != 1 {
		return State{}, "", errInvalidEmailSnapshot
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
		s.recordSystem("email.persistence_error", "email", "Could not create email repository: "+err.Error())
		return
	}
	s.EnableRepository(repository)
}

func (s *Service) EnableRepository(repository Repository) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if repository == nil {
		s.recordSystem("email.persistence_error", "email", "Email repository is nil.")
		return
	}
	s.repository = repository
	loadedFrom, err := s.loadLocked()
	if err == nil {
		s.recordSystem("email.persistence_loaded", "email", "Email state loaded from "+loadedFrom+".")
		return
	}
	if !errors.Is(err, snapshot.ErrNotFound) {
		s.recordSystem("email.persistence_ignored", "email", "Email repository could not be loaded: "+err.Error())
	}
	s.persistLocked()
}

func (s *Service) loadLocked() (string, error) {
	if s.repository == nil {
		return "", snapshot.ErrNotFound
	}
	state, loadedFrom, err := s.repository.LoadState()
	if err != nil {
		return "", err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	defaultSettings := s.settings
	if state.Settings.FromName == "" || state.Settings.FromEmail == "" {
		state.Settings = defaultSettings
	}
	if state.Settings.Provider == "" {
		state.Settings.Provider = "development"
	}
	if state.Settings.Mode == "" {
		state.Settings.Mode = "Development"
	}
	state.Settings.Configured = true
	state.Settings.PasswordConfigured = strings.TrimSpace(state.Settings.SMTPPassword) != ""

	s.settings = state.Settings
	s.templates = mergeTemplates(state.Templates, defaultTemplates(now))
	s.rules = mergeRules(state.Rules, defaultRules(now))
	if state.Outbox == nil {
		s.outbox = []OutboxItem{}
	} else {
		s.outbox = state.Outbox
	}
	if state.Deliveries == nil {
		s.deliveries = []Delivery{}
	} else {
		s.deliveries = state.Deliveries
	}
	s.rebuildMailerLocked()
	return loadedFrom, nil
}

func (s *Service) persistLocked() {
	if s.repository == nil {
		return
	}
	if err := s.repository.SaveState(State{
		Settings:   s.settings,
		Templates:  s.templates,
		Rules:      s.rules,
		Outbox:     trimOutbox(s.outbox, 500),
		Deliveries: trimDeliveries(s.deliveries, 1000),
	}); err != nil {
		s.recordSystem("email.persistence_error", "email", "Could not save email state: "+err.Error())
	}
}

func mergeTemplates(stored []Template, defaults []Template) []Template {
	byID := make(map[string]Template, len(stored)+len(defaults))
	for _, template := range defaults {
		byID[template.ID] = template
	}
	for _, template := range stored {
		if template.ID == "" || template.Key == "" || template.Language == "" {
			continue
		}
		template.Variables = variablesIn(template.Subject + "\n" + template.Body)
		if template.Version < 1 {
			template.Version = 1
		}
		if current, ok := byID[template.ID]; ok {
			// Refresh untouched system templates when defaults move forward.
			if strings.EqualFold(strings.TrimSpace(template.UpdatedBy), "system") && template.Version < current.Version {
				continue
			}
		}
		byID[template.ID] = template
	}
	result := make([]Template, 0, len(byID))
	for _, template := range byID {
		result = append(result, template)
	}
	return result
}

func mergeRules(stored []AutomationRule, defaults []AutomationRule) []AutomationRule {
	byID := make(map[string]AutomationRule, len(stored)+len(defaults))
	for _, rule := range defaults {
		byID[rule.ID] = rule
	}
	for _, rule := range stored {
		if rule.ID == "" || rule.Trigger == "" || rule.TemplateKey == "" {
			continue
		}
		byID[rule.ID] = rule
	}
	result := make([]AutomationRule, 0, len(byID))
	for _, rule := range byID {
		result = append(result, rule)
	}
	return result
}

func trimOutbox(values []OutboxItem, limit int) []OutboxItem {
	if values == nil {
		return []OutboxItem{}
	}
	if len(values) <= limit {
		return append([]OutboxItem{}, values...)
	}
	return append([]OutboxItem{}, values[:limit]...)
}

func trimDeliveries(values []Delivery, limit int) []Delivery {
	if values == nil {
		return []Delivery{}
	}
	if len(values) <= limit {
		return append([]Delivery{}, values...)
	}
	return append([]Delivery{}, values[:limit]...)
}
