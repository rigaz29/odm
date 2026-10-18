package browsers

import (
	"os"

	"golang.org/x/sys/windows/registry"
)

// Listed reports whether Installed can find the browsers here.
const Listed = true

// Installed lists the browsers Windows knows, from the entries browsers
// write for Default apps (Clients\StartMenuInternet): the user's own
// first, then the machine's, in both registry views.
func Installed() []Browser {
	type root struct {
		key    registry.Key
		access uint32
	}
	roots := []root{
		{registry.CURRENT_USER, 0},
		{registry.LOCAL_MACHINE, registry.WOW64_64KEY},
		{registry.LOCAL_MACHINE, registry.WOW64_32KEY},
	}
	const clients = `Software\Clients\StartMenuInternet`
	var list []Browser
	for _, r := range roots {
		k, err := registry.OpenKey(r.key, clients, registry.ENUMERATE_SUB_KEYS|r.access)
		if err != nil {
			continue
		}
		names, _ := k.ReadSubKeyNames(-1)
		k.Close()
		for _, n := range names {
			if b, ok := entry(r.key, clients+`\`+n, r.access); ok {
				list = append(list, b)
			}
		}
	}
	return unique(list)
}

// entry reads one browser's display name and command.
func entry(root registry.Key, path string, access uint32) (Browser, bool) {
	k, err := registry.OpenKey(root, path, registry.QUERY_VALUE|access)
	if err != nil {
		return Browser{}, false
	}
	name, _, _ := k.GetStringValue("")
	k.Close()
	c, err := registry.OpenKey(root, path+`\shell\open\command`, registry.QUERY_VALUE|access)
	if err != nil {
		return Browser{}, false
	}
	command, _, err := c.GetStringValue("")
	c.Close()
	if err != nil {
		return Browser{}, false
	}
	b, ok := identify(name, command)
	if !ok {
		return Browser{}, false
	}
	if _, err := os.Stat(b.Exe); err != nil {
		return Browser{}, false // uninstalled, the entry left behind
	}
	return b, true
}
