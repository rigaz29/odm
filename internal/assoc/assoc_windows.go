package assoc

import (
	"errors"
	"net/url"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const supported = true

const (
	// appName names ODM in Default apps and in its settings page's address.
	appName     = "Ochi Download Manager"
	magnetProg  = "ODM.Magnet"
	torrentProg = "ODM.Torrent"
	classes     = `Software\Classes\`
	capsKey     = `Software\ODM\Capabilities`
	appsKey     = `Software\RegisteredApplications`
)

// hive is where the keys go: HKCU itself, or (in tests) a key below it.
type hive struct {
	base string
	// handled reports whether a program opens magnet: links already.
	handled func() bool
}

var std = hive{handled: func() bool {
	k, err := registry.OpenKey(registry.CLASSES_ROOT, `magnet\shell\open\command`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	k.Close()
	return true
}}

func command(exe string) string { return `"` + exe + `" "%1"` }

type value struct{ key, name, data string }

// set writes ODM's two ProgIDs, its capabilities and its entry in
// RegisteredApplications, which is what lists ODM in Default apps, and
// names ODM.Torrent among the programs for .torrent files. When no program
// opens magnet: links yet, it also makes the scheme known, opened by ODM:
// browsers do not hand a link of an unknown scheme to Windows at all.
func (h hive) set(exe string) error {
	cmd, icon := command(exe), exe+",0"
	scheme := !h.handled() || h.ownScheme()
	values := []value{
		{classes + magnetProg, "", "Magnet link"},
		{classes + magnetProg, "URL Protocol", ""},
		{classes + magnetProg + `\DefaultIcon`, "", icon},
		{classes + magnetProg + `\shell\open\command`, "", cmd},
		{classes + torrentProg, "", "Torrent file"},
		{classes + torrentProg + `\DefaultIcon`, "", icon},
		{classes + torrentProg + `\shell\open\command`, "", cmd},
		{classes + `.torrent\OpenWithProgids`, torrentProg, ""},
		{capsKey, "ApplicationName", appName},
		{capsKey, "ApplicationDescription", "Downloads files, videos and torrents."},
		{capsKey, "ApplicationIcon", icon},
		{capsKey + `\URLAssociations`, "magnet", magnetProg},
		{capsKey + `\FileAssociations`, ".torrent", torrentProg},
		{appsKey, appName, capsKey},
	}
	if scheme {
		values = append(values,
			value{classes + "magnet", "", "URL:Magnet link"},
			value{classes + "magnet", "URL Protocol", ""},
			value{classes + `magnet\shell\open\command`, "", cmd})
	}
	for _, v := range values {
		k, _, err := registry.CreateKey(registry.CURRENT_USER, h.base+v.key, registry.SET_VALUE)
		if err != nil {
			return err
		}
		err = k.SetStringValue(v.name, v.data)
		k.Close()
		if err != nil {
			return err
		}
	}
	changed()
	return nil
}

// unset removes what set wrote. The magnet: scheme goes only while ODM
// still opens it: another program may have taken it over since.
func (h hive) unset() error {
	var errs []error
	if h.ownScheme() {
		errs = append(errs, h.deleteTree(classes+"magnet"))
	}
	errs = append(errs,
		h.deleteTree(classes+magnetProg),
		h.deleteTree(classes+torrentProg),
		h.deleteValue(classes+`.torrent\OpenWithProgids`, torrentProg),
		h.deleteTree(capsKey),
		h.deleteValue(appsKey, appName))
	// The keys set created, when nothing else is in them now.
	h.deleteEmpty(classes + `.torrent\OpenWithProgids`)
	h.deleteEmpty(classes + `.torrent`)
	h.deleteEmpty(`Software\ODM`)
	changed()
	return errors.Join(errs...)
}

// exe is the program ODM.Torrent opens .torrent files with.
func (h hive) exe() string {
	cmd := h.commandOf(classes + torrentProg)
	if !strings.HasPrefix(cmd, `"`) {
		return ""
	}
	exe, _, ok := strings.Cut(cmd[1:], `"`)
	if !ok {
		return ""
	}
	return exe
}

// ownScheme reports whether the magnet: scheme in HKCU is the one set made:
// it runs what ODM.Magnet runs.
func (h hive) ownScheme() bool {
	mine := h.commandOf(classes + magnetProg)
	return mine != "" && h.commandOf(classes+"magnet") == mine
}

func (h hive) commandOf(key string) string {
	k, err := registry.OpenKey(registry.CURRENT_USER, h.base+key+`\shell\open\command`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	s, _, err := k.GetStringValue("")
	if err != nil {
		return ""
	}
	return s
}

func (h hive) deleteValue(key, name string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, h.base+key, registry.SET_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	defer k.Close()
	if err := k.DeleteValue(name); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	return nil
}

// deleteTree deletes key with everything below it.
func (h hive) deleteTree(key string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, h.base+key, registry.ENUMERATE_SUB_KEYS)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	subs, err := k.ReadSubKeyNames(-1)
	k.Close()
	if err != nil {
		return err
	}
	for _, s := range subs {
		if err := h.deleteTree(key + `\` + s); err != nil {
			return err
		}
	}
	if err := registry.DeleteKey(registry.CURRENT_USER, h.base+key); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	return nil
}

// deleteEmpty deletes key if it has no values and no subkeys.
func (h hive) deleteEmpty(key string) {
	k, err := registry.OpenKey(registry.CURRENT_USER, h.base+key, registry.QUERY_VALUE)
	if err != nil {
		return
	}
	info, err := k.Stat()
	k.Close()
	if err == nil && info.SubKeyCount == 0 && info.ValueCount == 0 {
		_ = registry.DeleteKey(registry.CURRENT_USER, h.base+key)
	}
}

var shChangeNotify = windows.NewLazySystemDLL("shell32.dll").NewProc("SHChangeNotify")

// changed tells Explorer that associations changed, as Windows asks of a
// program that registers them.
func changed() {
	const shcneAssocChanged, shcnfIDList = 0x08000000, 0
	if shChangeNotify.Find() == nil {
		shChangeNotify.Call(shcneAssocChanged, shcnfIDList, 0, 0)
	}
}

// openSettings opens Default apps at ODM's page (Windows 11; Windows 10
// opens Default apps itself).
func openSettings() error {
	uri := "ms-settings:defaultapps?registeredAppUser=" + url.PathEscape(appName)
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(uri)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, file, nil, nil, windows.SW_SHOWNORMAL)
}
