package app

import (
	"os"
	"path/filepath"
	"testing"
)

func setTestHome(t *testing.T) string {
	t.Helper()
	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, 0755); err != nil {
		t.Fatalf("failed to create test home: %v", err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	return home
}

func TestMactopConfigDirUsesXDGConfigHome(t *testing.T) {
	setTestHome(t)
	configHome := filepath.Join(t.TempDir(), "xdg-config")
	t.Setenv("XDG_CONFIG_HOME", configHome)

	got := mactopConfigDir()
	want := filepath.Join(configHome, "mactop")
	if got != want {
		t.Fatalf("mactopConfigDir() = %q, want %q", got, want)
	}
}

func TestMactopConfigDirFallsBackToLegacyDir(t *testing.T) {
	home := setTestHome(t)

	got := mactopConfigDir()
	want := filepath.Join(home, ".mactop")
	if got != want {
		t.Fatalf("mactopConfigDir() = %q, want %q", got, want)
	}
}

func TestMactopConfigDirIgnoresRelativeXDGConfigHome(t *testing.T) {
	home := setTestHome(t)
	t.Setenv("XDG_CONFIG_HOME", "relative-config")

	got := mactopConfigDir()
	want := filepath.Join(home, ".mactop")
	if got != want {
		t.Fatalf("mactopConfigDir() = %q, want %q", got, want)
	}
}

func TestMactopStateDirUsesXDGStateHome(t *testing.T) {
	setTestHome(t)
	stateHome := filepath.Join(t.TempDir(), "xdg-state")
	t.Setenv("XDG_STATE_HOME", stateHome)

	got := mactopStateDir()
	want := filepath.Join(stateHome, "mactop")
	if got != want {
		t.Fatalf("mactopStateDir() = %q, want %q", got, want)
	}
}

func TestMactopStateDirFallsBackToLegacyDir(t *testing.T) {
	home := setTestHome(t)

	got := mactopStateDir()
	want := filepath.Join(home, ".mactop")
	if got != want {
		t.Fatalf("mactopStateDir() = %q, want %q", got, want)
	}
}
