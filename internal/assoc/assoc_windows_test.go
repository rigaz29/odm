package assoc

import (
	"errors"
	"testing"

	"golang.org/x/sys/windows/registry"
)

// testHive is a hive below HKCU\Software\ODM-assoc-test, removed after the
// test; handled says whether a program opens magnet: links already.
func testHive(t *testing.T, handled bool) hive {
	t.Helper()
	h := hive{base: `Software\ODM-assoc-test\`, handled: func() bool { return handled }}
	root := hive{}
	t.Cleanup(func() { _ = root.deleteTree(`Software\ODM-assoc-test`) })
	return h
}

func (h hive) has(key string) bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, h.base+key, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	k.Close()
	return true
}

func (h hive) str(key, name string) string {
	k, err := registry.OpenKey(registry.CURRENT_USER, h.base+key, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	s, _, _ := k.GetStringValue(name)
	return s
}

func TestSetAndUnset(t *testing.T) {
	h := testHive(t, false)
	exe := `C:\Program Files\ODM\ODM.exe`
	if err := h.set(exe); err != nil {
		t.Fatal(err)
	}
	if got := h.exe(); got != exe {
		t.Fatalf("exe %q", got)
	}
	cmd := `"C:\Program Files\ODM\ODM.exe" "%1"`
	for _, key := range []string{classes + magnetProg, classes + torrentProg, classes + "magnet"} {
		if got := h.commandOf(key); got != cmd {
			t.Fatalf("%s runs %q", key, got)
		}
	}
	if h.str(appsKey, appName) != capsKey || h.str(capsKey+`\URLAssociations`, "magnet") != magnetProg ||
		h.str(capsKey+`\FileAssociations`, ".torrent") != torrentProg {
		t.Fatal("ODM is not listed in Default apps")
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, h.base+classes+`.torrent\OpenWithProgids`, registry.QUERY_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = k.GetStringValue(torrentProg)
	k.Close()
	if err != nil {
		t.Fatalf("ODM.Torrent is not offered for .torrent files: %v", err)
	}

	// A moved ODM registers its new place, the scheme included.
	moved := `D:\ODM\ODM.exe`
	if err := h.set(moved); err != nil {
		t.Fatal(err)
	}
	if h.exe() != moved || h.commandOf(classes+"magnet") != command(moved) {
		t.Fatalf("after a move: %q, scheme %q", h.exe(), h.commandOf(classes+"magnet"))
	}

	if err := h.unset(); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{classes + magnetProg, classes + torrentProg, classes + "magnet", classes + ".torrent", `Software\ODM`} {
		if h.has(key) {
			t.Fatalf("%s left behind", key)
		}
	}
	if h.exe() != "" {
		t.Fatal("still registered")
	}
	if k, err := registry.OpenKey(registry.CURRENT_USER, h.base+appsKey, registry.QUERY_VALUE); err == nil {
		_, _, err := k.GetStringValue(appName)
		k.Close()
		if !errors.Is(err, registry.ErrNotExist) {
			t.Fatalf("still in RegisteredApplications: %v", err)
		}
	}
	// Nothing to remove is no error.
	if err := h.unset(); err != nil {
		t.Fatal(err)
	}
}

// A magnet: scheme another program registered is not ODM's to change.
func TestSchemeOfAnotherProgram(t *testing.T) {
	h := testHive(t, true)
	other := `"C:\qBittorrent\qbittorrent.exe" "%1"`
	k, _, err := registry.CreateKey(registry.CURRENT_USER, h.base+classes+`magnet\shell\open\command`, registry.SET_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	k.SetStringValue("", other)
	k.Close()

	if err := h.set(`C:\ODM\ODM.exe`); err != nil {
		t.Fatal(err)
	}
	if got := h.commandOf(classes + "magnet"); got != other {
		t.Fatalf("scheme taken over: %q", got)
	}
	if err := h.unset(); err != nil {
		t.Fatal(err)
	}
	if got := h.commandOf(classes + "magnet"); got != other {
		t.Fatalf("scheme removed: %q", got)
	}
	// .torrent keeps the keys another program has in it.
	k, _, err = registry.CreateKey(registry.CURRENT_USER, h.base+classes+`.torrent`, registry.SET_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	k.SetStringValue("", "qBittorrent.File.Torrent")
	k.Close()
	if err := h.set(`C:\ODM\ODM.exe`); err != nil {
		t.Fatal(err)
	}
	if err := h.unset(); err != nil {
		t.Fatal(err)
	}
	if h.str(classes+".torrent", "") != "qBittorrent.File.Torrent" {
		t.Fatal(".torrent's own program removed")
	}
}
