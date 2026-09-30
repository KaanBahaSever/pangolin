// Command pangolin is a local-first password keeper.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/awnumar/memguard"

	"github.com/KaanBahaSever/pangolin/internal/ui"
)

// vaultDir returns the directory holding the vault: $PANGOLIN_HOME if set,
// otherwise a Pangolin folder in the user's configuration directory.
func vaultDir() (string, error) {
	if dir := os.Getenv("PANGOLIN_HOME"); dir != "" {
		return filepath.Abs(dir)
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "Pangolin"), nil
}

func main() {
	// Wipe every locked buffer on the way out, including on Ctrl+C.
	memguard.CatchInterrupt()
	defer memguard.Purge()

	dir, err := vaultDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "pangolin:", err)
		memguard.SafeExit(1)
	}
	ui.Run(dir)
}
