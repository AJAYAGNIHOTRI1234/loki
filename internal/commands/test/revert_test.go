package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"loki/internal/commands"
)

func TestRevert_CleanlyRevertsLatestCommit(t *testing.T) {
	oldCwd, _ := os.Getwd()
	defer os.Chdir(oldCwd)

	tempDir := t.TempDir()
	os.Chdir(tempDir)
	commands.Init()

	if err := os.WriteFile("note.txt", []byte("first version\n"), 0644); err != nil {
		t.Fatal(err)
	}
	commands.Add([]string{"note.txt"})
	commands.Commit([]string{"-m", "initial note"})

	if err := os.WriteFile("note.txt", []byte("second version\n"), 0644); err != nil {
		t.Fatal(err)
	}
	commands.Add([]string{"note.txt"})
	commands.Commit([]string{"-m", "update note"})
	targetCommit := readBranchCommit(t, "main")

	if got := readFileString(t, "note.txt"); got != "second version\n" {
		t.Fatalf("expected note.txt to match latest commit before revert, got %q", got)
	}

	output := CaptureOutput(func() {
		commands.Revert([]string{targetCommit})
	})

	if !strings.Contains(output, "Reverted commit:") {
		t.Fatalf("expected revert success output, got: %s", output)
	}

	if got := readFileString(t, "note.txt"); got != "first version\n" {
		t.Fatalf("expected note.txt to revert to first version, got %q", got)
	}

	revertCommit := readBranchCommit(t, "main")
	parents := readCommitParents(t, revertCommit)
	if len(parents) != 1 || parents[0] != targetCommit {
		t.Fatalf("expected revert commit parent to be %s, got %+v", targetCommit, parents)
	}

	body := readCommitBody(t, revertCommit)
	if !strings.Contains(body, "Revert: update note") {
		t.Fatalf("expected revert commit message to reference original commit, got: %q", body)
	}
}

func TestRevert_AddDeleteConflictLeavesFileUntouched(t *testing.T) {
	oldCwd, _ := os.Getwd()
	defer os.Chdir(oldCwd)

	tempDir := t.TempDir()
	os.Chdir(tempDir)
	commands.Init()

	if err := os.WriteFile("a.txt", []byte("hello\n"), 0644); err != nil {
		t.Fatal(err)
	}
	commands.Add([]string{"a.txt"})
	commands.Commit([]string{"-m", "first"})
	firstCommit := readBranchCommit(t, "main")

	if err := os.WriteFile("a.txt", []byte("hello world\n"), 0644); err != nil {
		t.Fatal(err)
	}
	commands.Add([]string{"a.txt"})
	commands.Commit([]string{"-m", "second"})

	beforeRevertCommit := readBranchCommit(t, "main")
	output := CaptureOutput(func() {
		commands.Revert([]string{firstCommit})
	})

	if !strings.Contains(output, "CONFLICT (modify/delete)") {
		t.Fatalf("expected revert success output, got: %s", output)
	}

	got := readFileString(t, "a.txt")
	if got != "hello world\n" {
		t.Fatalf("expected current file to be left untouched, got %q", got)
	}

	afterRevertCommit := readBranchCommit(t, "main")
	if afterRevertCommit != beforeRevertCommit {
		t.Fatalf("expected no new commit to be created, got %s want %s", afterRevertCommit, beforeRevertCommit)
	}

	if _, err := os.Stat(filepath.Join(".loki", "REVERT_HEAD")); err != nil {
		t.Fatalf("expected revert conflict state to be recorded: %v", err)
	}
}
