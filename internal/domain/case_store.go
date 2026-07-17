package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// CaseStore keeps consultation records outside process memory. It is deliberately
// small so that a SQL-backed implementation can replace it without changing the
// product state machine.
type CaseStore interface {
	Load() ([]*ConsultationCase, error)
	Save([]*ConsultationCase) error
}

type FileCaseStore struct {
	path string
}

func NewFileCaseStore(dataDir string) (*FileCaseStore, error) {
	if dataDir == "" {
		return nil, errors.New("data directory is required")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	return &FileCaseStore{path: filepath.Join(dataDir, "consultation-cases.json")}, nil
}

func (s *FileCaseStore) Load() ([]*ConsultationCase, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read cases: %w", err)
	}
	var cases []*ConsultationCase
	if err := json.Unmarshal(data, &cases); err != nil {
		return nil, fmt.Errorf("decode cases: %w", err)
	}
	return cases, nil
}

func (s *FileCaseStore) Save(cases []*ConsultationCase) error {
	data, err := json.MarshalIndent(cases, "", "  ")
	if err != nil {
		return fmt.Errorf("encode cases: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".cases-*.tmp")
	if err != nil {
		return fmt.Errorf("create case temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write cases: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("protect cases: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close cases: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("replace cases: %w", err)
	}
	return nil
}
