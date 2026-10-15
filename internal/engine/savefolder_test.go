package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveFolderRemembersTheLastChoice(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	def, other := t.TempDir(), t.TempDir()
	m := NewManager(Settings{Dir: def}, path)
	if got := m.Settings().SaveFolder(); got != def {
		t.Fatalf("fresh install offers %q, want the default %q", got, def)
	}

	m.NoteSaveFolder(other)
	if got := m.Settings().SaveFolder(); got != other {
		t.Fatalf("offers %q after choosing %q", got, other)
	}
	if saved := LoadSettings(path, def); saved.LastDir != other {
		t.Fatalf("settings.json last_dir = %q", saved.LastDir)
	}

	// A folder that is gone (a removed drive) falls back to the default.
	if err := os.RemoveAll(other); err != nil {
		t.Fatal(err)
	}
	if got := m.Settings().SaveFolder(); got != def {
		t.Fatalf("offers the missing %q, want the default", got)
	}
}

func TestSaveFolderCanBeTurnedOff(t *testing.T) {
	def, other := t.TempDir(), t.TempDir()
	m := NewManager(Settings{Dir: def, NoRememberDir: true}, "")
	m.NoteSaveFolder(other)
	st := m.Settings()
	if st.LastDir != "" || st.SaveFolder() != def {
		t.Fatalf("turned off, still remembered: last %q, offered %q", st.LastDir, st.SaveFolder())
	}
}

func TestNoteSaveFolderSkipsUnchangedFolders(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	def := t.TempDir()
	m := NewManager(Settings{Dir: def}, path)
	m.NoteSaveFolder(def) // what is offered already: nothing to write
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("settings written for an unchanged folder: %v", err)
	}
}
