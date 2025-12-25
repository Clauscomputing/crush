//go:build !openbsd

package security

// Init is a no-op on non-OpenBSD systems.
func Init(dataDir, workingDir string) error {
	return nil
}

// Shutdown is a no-op on non-OpenBSD systems.
func Shutdown() error {
	return nil
}

// IsInitialized always returns false on non-OpenBSD systems.
func IsInitialized() bool {
	return false
}
