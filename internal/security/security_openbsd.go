//go:build openbsd

package security

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sys/unix"
)

var (
	mu          sync.Mutex
	initialized = false
)

// allPromises contains all pledge promises needed for the application lifetime.
// On OpenBSD, pledge can only drop promises, never add them. So we must
// request everything upfront.
const allPromises = "stdio rpath wpath cpath dpath flock fattr proc exec inet dns unix tty getpw"

// shutdownPromises contains minimal promises needed during shutdown.
// Only used after all cleanup requiring filesystem/network is complete.
const shutdownPromises = "stdio"

// Init initializes OpenBSD security sandboxing with pledge and unveil.
// This must be called once during startup with all paths that will be needed.
// After this call, only the specified paths will be accessible.
func Init(dataDir, workingDir string) error {
	mu.Lock()
	defer mu.Unlock()

	if initialized {
		return fmt.Errorf("security already initialized")
	}

	// Step 1: Set up unveil BEFORE calling UnveilBlock
	if err := setupUnveil(dataDir, workingDir); err != nil {
		return fmt.Errorf("unveil setup failed: %w", err)
	}

	// Step 2: Lock unveil - no more paths can be added after this
	if err := unix.UnveilBlock(); err != nil {
		return fmt.Errorf("unveil lock failed: %w", err)
	}

	// Step 3: Set pledge with all needed promises
	// We request everything upfront since pledge can only become more restrictive
	if err := unix.PledgePromises(allPromises); err != nil {
		return fmt.Errorf("pledge failed: %w", err)
	}

	initialized = true
	slog.Debug("openbsd security: sandbox active",
		"promises", allPromises,
		"data_dir", dataDir,
		"working_dir", workingDir,
	)

	return nil
}

// setupUnveil configures filesystem visibility before locking.
func setupUnveil(dataDir, workingDir string) error {
	// System paths needed for basic operation (read-only)
	systemPaths := []struct {
		path  string
		perms string
	}{
		// Libraries and binaries for exec
		{"/usr/lib", "r"},
		{"/usr/local/lib", "r"},
		{"/usr/bin", "rx"},
		{"/usr/local/bin", "rx"},
		{"/bin", "rx"},
		{"/sbin", "r"},

		// SSL certificates for HTTPS
		{"/etc/ssl/cert.pem", "r"},
		{"/etc/ssl/certs", "r"},

		// DNS and hostname resolution
		{"/etc/resolv.conf", "r"},
		{"/etc/hosts", "r"},

		// User database (needed by os.UserHomeDir, etc.)
		{"/etc/passwd", "r"},

		// Timezone data
		{"/usr/share/zoneinfo", "r"},
		{"/etc/localtime", "r"},

		// Device access for terminal
		{"/dev/null", "rw"},
		{"/dev/tty", "rw"},
		{"/dev/urandom", "r"},
	}

	unveilCount := 0
	for _, sp := range systemPaths {
		if err := unix.Unveil(sp.path, sp.perms); err != nil {
			// Log but don't fail - path may not exist on all systems
			slog.Debug("unveil skipped", "path", sp.path, "error", err)
		} else {
			unveilCount++
		}
	}

	// Data directory - full access for database, logs, config
	if dataDir != "" {
		absDataDir, err := filepath.Abs(dataDir)
		if err != nil {
			return fmt.Errorf("resolving data dir: %w", err)
		}
		if err := unix.Unveil(absDataDir, "rwc"); err != nil {
			return fmt.Errorf("unveil data dir %s: %w", absDataDir, err)
		}
		unveilCount++
	}

	// Working directory (project) - full access for code editing
	if workingDir != "" {
		absWorkDir, err := filepath.Abs(workingDir)
		if err != nil {
			return fmt.Errorf("resolving working dir: %w", err)
		}
		if err := unix.Unveil(absWorkDir, "rwcx"); err != nil {
			return fmt.Errorf("unveil working dir %s: %w", absWorkDir, err)
		}
		unveilCount++
	}

	// User home directories for config files
	if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
		// .crush directory if different from dataDir
		crushDir := filepath.Join(homeDir, ".crush")
		if absDataDir, _ := filepath.Abs(dataDir); absDataDir != crushDir {
			if err := unix.Unveil(crushDir, "rwc"); err != nil {
				slog.Debug("unveil home .crush skipped", "error", err)
			}
		}

		// XDG config and data directories (read/write for app config)
		xdgPaths := []string{
			filepath.Join(homeDir, ".config"),
			filepath.Join(homeDir, ".local", "share"),
		}
		for _, xp := range xdgPaths {
			if err := unix.Unveil(xp, "rwc"); err != nil {
				slog.Debug("unveil xdg path skipped", "path", xp, "error", err)
			}
		}

		// Git config (read-only)
		if err := unix.Unveil(filepath.Join(homeDir, ".gitconfig"), "r"); err != nil {
			slog.Debug("unveil gitconfig skipped", "error", err)
		}

		// Go toolchain directories from environment (needed for shell commands running go build, etc.)
		if gopath := os.Getenv("GOPATH"); gopath != "" {
			if err := unix.Unveil(gopath, "rwc"); err != nil {
				slog.Debug("unveil GOPATH skipped", "path", gopath, "error", err)
			}
		}
		if gocache := os.Getenv("GOCACHE"); gocache != "" {
			if err := unix.Unveil(gocache, "rwc"); err != nil {
				slog.Debug("unveil GOCACHE skipped", "path", gocache, "error", err)
			}
		}
	}

	// Temp directory - use user-specific subdirectory for safety
	// Create it if it doesn't exist
	tmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("crush-%d", os.Getuid()))
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		slog.Debug("failed to create temp dir", "path", tmpDir, "error", err)
		// Fall back to system tmp with more restricted access
		tmpDir = os.TempDir()
	}
	if err := unix.Unveil(tmpDir, "rwc"); err != nil {
		slog.Debug("unveil temp dir skipped", "path", tmpDir, "error", err)
	} else {
		unveilCount++
	}

	slog.Debug("unveil setup complete", "total_paths", unveilCount)
	return nil
}

// Shutdown tightens permissions to minimum for graceful exit.
// Call this only after all cleanup operations are complete.
func Shutdown() error {
	mu.Lock()
	defer mu.Unlock()

	if !initialized {
		return nil
	}

	// Reduce to minimal promises - stdio only
	if err := unix.PledgePromises(shutdownPromises); err != nil {
		// Log but don't fail - we're shutting down anyway
		slog.Debug("pledge shutdown failed", "error", err)
		return err
	}

	slog.Debug("openbsd security: shutdown promises active")
	return nil
}

// IsInitialized returns whether security sandboxing has been set up.
func IsInitialized() bool {
	mu.Lock()
	defer mu.Unlock()
	return initialized
}
