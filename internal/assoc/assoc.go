// Package assoc makes ODM one of the programs Windows offers for magnet
// links and .torrent files. It registers ODM as able to open them, for the
// current user only; which program opens them stays the user's choice,
// made in Windows (Settings → Default apps, or its "How do you want to
// open this?" question). Turned off, it removes what it wrote. Other
// systems have nothing to register.
package assoc

// Supported reports whether this system has associations ODM registers.
func Supported() bool { return supported }

// Set registers exe as able to open magnet links and .torrent files, or
// removes that registration.
func Set(enable bool, exe string) error {
	if enable {
		return std.set(exe)
	}
	return std.unset()
}

// Exe is the program registered, "" when none is.
func Exe() string { return std.exe() }

// OpenSettings opens the system's page for choosing the programs that open
// magnet links and .torrent files.
func OpenSettings() error { return openSettings() }
