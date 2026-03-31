package test

import (
	"bytes"
	"loki/internal/commands"
	"testing"
)

func TestHelp_PrintsCommands(t *testing.T) {
	output := CaptureOutput(func() {
		commands.Help()
	})

	header := "These are common loki commands used in various situations:"
	if !bytes.Contains([]byte(output), []byte(header)) {
		t.Errorf("Help output missing expected header: %s", output)
	}
}
