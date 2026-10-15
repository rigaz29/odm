//go:build windows

package quarantine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows/registry"
)

// zoneInfoOff reports whether policy tells Windows not to keep where files
// came from (SaveZoneInformation = 1), in which case nothing is written.
func zoneInfoOff() bool {
	for _, root := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		k, err := registry.OpenKey(root, `Software\Microsoft\Windows\CurrentVersion\Policies\Attachments`, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		v, _, err := k.GetIntegerValue("SaveZoneInformation")
		k.Close()
		if err == nil && v == 1 {
			return true
		}
	}
	return false
}

func sample(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("hello from the internet\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMarkWritesTheInternetZone(t *testing.T) {
	if zoneInfoOff() {
		t.Skip("policy turns zone information off")
	}
	path := sample(t)
	if err := Mark(path, "https://user:secret@example.com/files/notes.txt", "https://example.com/page"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path + ":Zone.Identifier")
	if err != nil {
		t.Fatalf("no Zone.Identifier stream: %v", err)
	}
	if !strings.Contains(string(data), "ZoneId=3") {
		t.Fatalf("stream %q does not name the internet zone", data)
	}
	if strings.Contains(string(data), "secret") {
		t.Fatalf("the password went into the stream: %q", data)
	}
	if got, _ := os.ReadFile(path); string(got) != "hello from the internet\r\n" {
		t.Fatal("the file itself changed")
	}
}

func TestWriteZoneIdentifierFallback(t *testing.T) {
	path := sample(t)
	if err := writeZoneIdentifier(path, "https://example.com/notes.txt", "https://example.com/"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path + ":Zone.Identifier")
	if err != nil {
		t.Fatal(err)
	}
	want := "[ZoneTransfer]\r\nZoneId=3\r\nReferrerUrl=https://example.com/\r\nHostUrl=https://example.com/notes.txt\r\n"
	if string(data) != want {
		t.Fatalf("stream %q, want %q", data, want)
	}
}

// The file goes through Attachment Services itself, not the fallback.
func TestAttachmentServicesRuns(t *testing.T) {
	if zoneInfoOff() {
		t.Skip("policy turns zone information off")
	}
	path := sample(t)
	if err := attachmentServices(path, "https://example.com/notes.txt", ""); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path + ":Zone.Identifier")
	if err != nil {
		t.Fatalf("no Zone.Identifier stream: %v", err)
	}
	t.Logf("Attachment Services wrote %q", data)
}
