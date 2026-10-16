//go:build !windows

package fluent

import "os"

func systemLanguage() string {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

func windowsAccent() string { return "" }
