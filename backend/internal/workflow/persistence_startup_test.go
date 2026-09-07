package workflow

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"chakuchuri/backend/internal/platform/snapshot"
)

type startupRepository struct {
	state   State
	loadErr error
	saves   int
}

func (r *startupRepository) LoadState() (State, string, error) {
	return r.state, "test", r.loadErr
}

func (r *startupRepository) SaveState(State) error {
	r.saves++
	return nil
}

func TestWorkflowStartupDoesNotSaveAfterLoadFailure(t *testing.T) {
	for _, loadErr := range []error{errors.New("database unavailable"), errInvalidWorkflowSnapshot, snapshot.ErrNotFound} {
		t.Run(loadErr.Error(), func(t *testing.T) {
			repository := &startupRepository{loadErr: loadErr}
			if err := NewService(nil, nil).EnableRepository(repository); !errors.Is(err, loadErr) {
				t.Fatalf("expected load error, got %v", err)
			}
			if repository.saves != 0 {
				t.Fatal("load failure overwrote repository")
			}
		})
	}
}

func TestWorkflowStartupAcceptsEmptyRepositoryWithoutDemoData(t *testing.T) {
	service := NewService(nil, nil)
	repository := &startupRepository{state: State{Version: 1}}
	if err := service.EnableRepository(repository); err != nil {
		t.Fatal(err)
	}
	if len(service.quotations) != 0 || len(service.manufacturing) != 0 || repository.saves != 0 {
		t.Fatal("empty repository was replaced by demo state")
	}
}

func TestWorkflowStartupLocalBootstrapAndCorruption(t *testing.T) {
	dir := t.TempDir()
	if err := NewService(nil, nil).EnablePersistence(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "workflow.dev.json")); err != nil {
		t.Fatal(err)
	}
	brokenDir := t.TempDir()
	path := filepath.Join(brokenDir, "workflow.dev.json")
	original := []byte("{broken")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := NewService(nil, nil).EnablePersistence(brokenDir); err == nil {
		t.Fatal("corrupt file accepted")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(original) {
		t.Fatal("corrupt file was overwritten")
	}
}
