//go:build windows

package store

import "os"

// On Windows, file locking is a no-op for now.
// The mutex in Store still serializes writes within the same process.
func lockFile(_ *os.File) error {
	return nil
}

func unlockFile(_ *os.File) error {
	return nil
}
