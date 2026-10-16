// Command appicon writes ODM's icon as a Windows .ico, for scripts/winres.sh.
package main

import (
	"fmt"
	"os"

	"odm/internal/gui"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: appicon <out.ico>")
		os.Exit(2)
	}
	if err := os.WriteFile(os.Args[1], gui.IconFile(), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
