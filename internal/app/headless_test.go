package app

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestHeadlessIntegration(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping integration test in CI environment")
	}

	projectRoot := filepath.Join("..", "..")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	buildCmd := exec.CommandContext(ctx, "go", "build", "-o", "mactop_test_binary", ".")
	buildCmd.Dir = projectRoot
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build binary: %v\nOutput: %s", err, out)
	}
	defer os.Remove(filepath.Join(projectRoot, "mactop_test_binary"))

	cmd := exec.CommandContext(ctx, "./mactop_test_binary", "--headless", "--count", "1")
	cmd.Dir = projectRoot

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("headless command failed: %v\nstderr: %s", err, stderr.String())
	}

	output := stdout.Bytes()
	if len(output) == 0 {
		t.Fatal("Expected non-empty output from headless mode")
	}

	var result []HeadlessOutput
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, string(output))
	}

	if len(result) != 1 {
		t.Errorf("Expected 1 sample, got %d", len(result))
	}

	sample := result[0]
	if sample.Timestamp == "" {
		t.Error("Expected non-empty timestamp")
	}
	if sample.SystemInfo.Name == "" {
		t.Error("Expected non-empty system name")
	}
	if sample.SystemInfo.CoreCount == 0 {
		t.Error("Expected non-zero core count")
	}
}
