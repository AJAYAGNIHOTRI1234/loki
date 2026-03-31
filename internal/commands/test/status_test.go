package test

import (
	"bytes"
	"loki/internal/commands"
	"os"
	"testing"
)

func TestStatus_NoFilesStaged(t *testing.T) {
	tempDir := t.TempDir()
	os.Chdir(tempDir)
	commands.Init()

	output := CaptureOutput(func() {
		commands.Status()
	})

	if !bytes.Contains([]byte(output), []byte("No files staged to commit")) {
		t.Errorf("Expected warning for no files staged, got: %s", output)
	}
}
