package revanced

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/gofrs/flock"
)

// Phase represents the current stage of the revanced build pipeline.
type Phase string

const (
	PhaseIdle        Phase = "idle"
	PhaseResolving   Phase = "resolving"
	PhaseAwaitingAPK Phase = "awaiting_apks"
	PhaseBuilding    Phase = "building"
)

// RequiredAPK describes an APK that the resolver determined is needed.
type RequiredAPK struct {
	PackageName string `json:"package_name"`
	AppName     string `json:"app_name"`
	Version     string `json:"version"`
	Received    bool   `json:"received"`
}

// State is the persistent state of the revanced pipeline.
type State struct {
	Phase        Phase         `json:"phase"`
	RequiredAPKs []RequiredAPK `json:"required_apks,omitempty"`
	ChatID       int64         `json:"chat_id,omitempty"`
	StartedAt    time.Time     `json:"started_at,omitempty"`
	Error        string        `json:"error,omitempty"`
}

// StateStore provides atomic read/write of pipeline state backed by a JSON
// file protected with a file lock.
type StateStore struct {
	path string
	mu   sync.Mutex
}

// NewStateStore creates a store that reads/writes at the given path.
func NewStateStore(path string) *StateStore {
	return &StateStore{path: path}
}

// Load reads the current state from disk.  If the file does not exist an
// idle state is returned.
func (s *StateStore) Load() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fl := flock.New(s.path + ".lock")
	if err := fl.Lock(); err != nil {
		return State{}, fmt.Errorf("lock state file: %w", err)
	}
	defer fl.Unlock()

	return s.readUnsafe()
}

// Save atomically writes the state to disk.
func (s *StateStore) Save(st State) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	fl := flock.New(s.path + ".lock")
	if err := fl.Lock(); err != nil {
		return fmt.Errorf("lock state file: %w", err)
	}
	defer fl.Unlock()

	return s.writeUnsafe(st)
}

// Update applies fn to the current state and writes back the result.
func (s *StateStore) Update(fn func(*State) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	fl := flock.New(s.path + ".lock")
	if err := fl.Lock(); err != nil {
		return fmt.Errorf("lock state file: %w", err)
	}
	defer fl.Unlock()

	st, err := s.readUnsafe()
	if err != nil {
		return err
	}
	if err := fn(&st); err != nil {
		return err
	}
	return s.writeUnsafe(st)
}

func (s *StateStore) readUnsafe() (State, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return State{Phase: PhaseIdle}, nil
		}
		return State{}, fmt.Errorf("read state: %w", err)
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return State{}, fmt.Errorf("parse state: %w", err)
	}
	if st.Phase == "" {
		st.Phase = PhaseIdle
	}
	return st, nil
}

func (s *StateStore) writeUnsafe(st State) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	if err := os.WriteFile(s.path, data, 0644); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	return nil
}
