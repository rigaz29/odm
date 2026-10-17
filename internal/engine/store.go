package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Store persists download state as one JSON file per download. Writes are
// serialised: two saves of one download share a temporary file name, and
// interleaved writes to it could leave a state file that no longer parses,
// which drops that download from the list at the next start.
type Store struct {
	dir string
	mu  sync.Mutex
}

// NewStore prepares the state directory.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

func (s *Store) path(id string) string {
	return filepath.Join(s.dir, id+".json")
}

// Save writes the state atomically. The new file is flushed to the disk
// before it replaces the old one: after a power cut, a renamed file whose
// bytes never reached the disk would not parse either.
func (s *Store) Save(st State) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tmp := s.path(st.ID) + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp, s.path(st.ID))
}

// Delete removes the state file for a download.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := os.Remove(s.path(id))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Load returns every persisted download state.
func (s *Store) Load() ([]State, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	var out []State
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			continue
		}
		var st State
		if err := json.Unmarshal(data, &st); err != nil {
			continue
		}
		out = append(out, st)
	}
	return out, nil
}
