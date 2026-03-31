package test

import (
	"loki/internal/commands"
	"os"
	"testing"
)

func TestInit(t *testing.T) {
	tempDir := t.TempDir()
	os.Chdir(tempDir)
	commands.Init()

	files := []string{".loki", ".loki/objects", ".loki/refs", ".loki/HEAD", ".loki/config"}
	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("Expected file or directory %s to exist, but got error: %v", f, err)
		}
	}
}
