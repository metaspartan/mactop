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

func TestSaveAndLoadConfigUseXDGConfigHome(t *testing.T) {
	origConfig := currentConfig
	defer func() { currentConfig = origConfig }()

	setTestHome(t)
	configHome := filepath.Join(t.TempDir(), "xdg-config")
	t.Setenv("XDG_CONFIG_HOME", configHome)

	currentConfig = AppConfig{
		DefaultLayout: "compact",
		Theme:         "green",
		Background:    "clear",
		Interval:      1500,
		SortReverse:   true,
	}
	saveConfig()

	configPath := filepath.Join(configHome, "mactop", "config.json")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected config at %s: %v", configPath, err)
	}

	currentConfig = AppConfig{}
	loadConfig()

	if currentConfig.DefaultLayout != "compact" {
		t.Fatalf("DefaultLayout = %q, want compact", currentConfig.DefaultLayout)
	}
	if currentConfig.Theme != "green" {
		t.Fatalf("Theme = %q, want green", currentConfig.Theme)
	}
	if currentConfig.Interval != 1500 {
		t.Fatalf("Interval = %d, want 1500", currentConfig.Interval)
	}
	if !currentConfig.SortReverse {
		t.Fatal("SortReverse = false, want true")
	}
}

func TestLoadThemeFileUsesXDGConfigHome(t *testing.T) {
	setTestHome(t)
	configHome := filepath.Join(t.TempDir(), "xdg-config")
	t.Setenv("XDG_CONFIG_HOME", configHome)

	themeDir := filepath.Join(configHome, "mactop")
	if err := os.MkdirAll(themeDir, 0755); err != nil {
		t.Fatalf("failed to create theme dir: %v", err)
	}
	themePath := filepath.Join(themeDir, "theme.json")
	if err := os.WriteFile(themePath, []byte(`{"foreground":"#9580FF","background":"#22212C"}`), 0644); err != nil {
		t.Fatalf("failed to write theme file: %v", err)
	}

	theme := loadThemeFile()
	if theme == nil {
		t.Fatal("loadThemeFile() returned nil")
	}
	if theme.Foreground != "#9580FF" {
		t.Fatalf("Foreground = %q, want #9580FF", theme.Foreground)
	}
	if theme.Background != "#22212C" {
		t.Fatalf("Background = %q, want #22212C", theme.Background)
	}
}

func TestSetupLogfileUsesXDGStateHome(t *testing.T) {
	setTestHome(t)
	stateHome := filepath.Join(t.TempDir(), "xdg-state")
	t.Setenv("XDG_STATE_HOME", stateHome)

	logfile, err := setupLogfile()
	if err != nil {
		t.Fatalf("setupLogfile() error = %v", err)
	}
	defer logfile.Close()

	want := filepath.Join(stateHome, "mactop", "mactop.log")
	if logfile.Name() != want {
		t.Fatalf("logfile.Name() = %q, want %q", logfile.Name(), want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("expected log file at %s: %v", want, err)
	}
}

func TestSetupLogfileFallsBackToLegacyDir(t *testing.T) {
	home := setTestHome(t)

	logfile, err := setupLogfile()
	if err != nil {
		t.Fatalf("setupLogfile() error = %v", err)
	}
	defer logfile.Close()

	want := filepath.Join(home, ".mactop", "mactop.log")
	if logfile.Name() != want {
		t.Fatalf("logfile.Name() = %q, want %q", logfile.Name(), want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("expected log file at %s: %v", want, err)
	}
}
