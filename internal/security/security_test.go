package security

import (
	"os"
	"testing"
)

func TestInit(t *testing.T) {
	// Get valid directories for testing
	tmpDir := os.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	// On non-OpenBSD systems, Init is a no-op and should always succeed.
	// On OpenBSD, this test should only run once per process since
	// pledge/unveil cannot be re-initialized.
	err = Init(tmpDir, cwd)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Verify initialized state
	if !IsInitialized() {
		// On non-OpenBSD, IsInitialized returns false (stub behavior)
		// On OpenBSD, it should return true
		t.Log("IsInitialized returned false (expected on non-OpenBSD)")
	}
}

func TestInitDoubleCall(t *testing.T) {
	// Skip if already initialized (e.g., from TestInit running first)
	if IsInitialized() {
		t.Skip("security already initialized, cannot test double-init")
	}

	tmpDir := os.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	// First call should succeed
	if err := Init(tmpDir, cwd); err != nil {
		t.Fatalf("first Init failed: %v", err)
	}

	// Second call should fail on OpenBSD, succeed (no-op) on others
	err = Init(tmpDir, cwd)
	if IsInitialized() && err == nil {
		t.Error("expected error on double Init when initialized")
	}
}

func TestShutdown(t *testing.T) {
	// Shutdown should always succeed, even if not initialized
	err := Shutdown()
	if err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}
}

func TestIsInitialized(t *testing.T) {
	// Just verify it doesn't panic
	_ = IsInitialized()
}
