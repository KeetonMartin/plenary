package plenary

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type EventStore interface {
	Append(Event) error
	ListByPlenary(plenaryID string) ([]Event, error)
	ListAll() ([]Event, error)
}

type JSONLStore struct {
	Path string
}

func NewJSONLStore(path string) *JSONLStore {
	return &JSONLStore{Path: path}
}

func (s *JSONLStore) ensureFile() error {
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(s.Path, os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	return f.Close()
}

func (s *JSONLStore) Append(evt Event) error {
	if err := s.ensureFile(); err != nil {
		return err
	}
	f, err := os.OpenFile(s.Path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}
	return nil
}

func (s *JSONLStore) ListByPlenary(plenaryID string) ([]Event, error) {
	events, err := s.ListAll()
	if err != nil {
		return nil, err
	}
	out := make([]Event, 0, len(events))
	for _, evt := range events {
		if evt.PlenaryID == plenaryID {
			out = append(out, evt)
		}
	}
	return out, nil
}

func (s *JSONLStore) ListAll() ([]Event, error) {
	if err := s.ensureFile(); err != nil {
		return nil, err
	}
	f, err := os.Open(s.Path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var events []Event
	line := 0
	for scanner.Scan() {
		line++
		txt := scanner.Bytes()
		if len(txt) == 0 {
			continue
		}
		var evt Event
		if err := json.Unmarshal(txt, &evt); err != nil {
			return nil, fmt.Errorf("decode line %d: %w", line, err)
		}
		events = append(events, evt)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

