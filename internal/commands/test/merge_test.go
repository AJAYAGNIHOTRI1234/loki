package test

import (
	"bytes"
	"compress/zlib"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"loki/internal/commands"
)

func readBranchCommit(t *testing.T, branch string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(".loki", "refs", "heads", branch))
	if err != nil {
		t.Fatalf("failed to read branch %s: %v", branch, err)
	}
	return strings.TrimSpace(string(data))
}

func readCommitParents(t *testing.T, commitHash string) []string {
	t.Helper()
	data := readCommitBody(t, commitHash)
	parents := []string{}
	for _, line := range strings.Split(data, "\n") {
		if strings.HasPrefix(line, "parent ") {
			parents = append(parents, strings.TrimSpace(line[7:]))
		}
	}
	return parents
}

func readCommitBody(t *testing.T, commitHash string) string {
	t.Helper()
	objPath := filepath.Join(".loki", "objects", commitHash[:2], commitHash[2:])
	file, err := os.Open(objPath)
	if err != nil {
		t.Fatalf("failed to open object %s: %v", objPath, err)
	}
	defer file.Close()

	reader, err := zlib.NewReader(file)
	if err != nil {
		t.Fatalf("failed to decompress object %s: %v", objPath, err)
	}
	defer reader.Close()

	contents, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read object %s: %v", objPath, err)
	}
	idx := bytes.IndexByte(contents, 0)
	if idx < 0 {
		t.Fatalf("invalid object format for %s", commitHash)
	}
	return string(contents[idx+1:])
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return string(data)
}

func TestMerge_CreatesMergeCommit(t *testing.T) {
	oldCwd, _ := os.Getwd()
	defer os.Chdir(oldCwd)

	tempDir := t.TempDir()
	os.Chdir(tempDir)
	commands.Init()

	if err := os.WriteFile("base.txt", []byte("base\n"), 0644); err != nil {
		t.Fatal(err)
	}
	commands.Add([]string{"base.txt"})
	commands.Commit([]string{"-m", "base"})

	commands.Branch([]string{"feature"})

	if err := os.WriteFile("main.txt", []byte("main line\n"), 0644); err != nil {
		t.Fatal(err)
	}
	commands.Add([]string{"main.txt"})
	commands.Commit([]string{"-m", "main work"})
	mainCommit := readBranchCommit(t, "main")

	commands.Checkout([]string{"feature"})
	if err := os.WriteFile("feature.txt", []byte("feature line\n"), 0644); err != nil {
		t.Fatal(err)
	}
	commands.Add([]string{"feature.txt"})
	commands.Commit([]string{"-m", "feature work"})
	featureCommit := readBranchCommit(t, "feature")

	commands.Checkout([]string{"main"})
	output := CaptureOutput(func() {
		commands.Merge([]string{"feature"})
	})

	if !strings.Contains(output, "Merged feature into current branch") {
		t.Fatalf("expected merge success message, got: %s", output)
	}

	mergeCommit := readBranchCommit(t, "main")
	parents := readCommitParents(t, mergeCommit)
	if len(parents) != 2 {
		t.Fatalf("expected merge commit to have 2 parents, got %d", len(parents))
	}

	if !(parents[0] == mainCommit && parents[1] == featureCommit) && !(parents[0] == featureCommit && parents[1] == mainCommit) {
		t.Fatalf("unexpected merge parents: %+v", parents)
	}

	if got := readFileString(t, "main.txt"); got != "main line\n" {
		t.Fatalf("expected main.txt to remain from current branch, got %q", got)
	}
	if got := readFileString(t, "feature.txt"); got != "feature line\n" {
		t.Fatalf("expected feature.txt to be merged in, got %q", got)
	}
	if got := readFileString(t, "base.txt"); got != "base\n" {
		t.Fatalf("expected base.txt to remain present, got %q", got)
	}
}

func TestMerge_ConflictRequiresResolution(t *testing.T) {
	oldCwd, _ := os.Getwd()
	defer os.Chdir(oldCwd)

	tempDir := t.TempDir()
	os.Chdir(tempDir)
	commands.Init()

	if err := os.WriteFile("conflict.txt", []byte("base\n"), 0644); err != nil {
		t.Fatal(err)
	}
	commands.Add([]string{"conflict.txt"})
	commands.Commit([]string{"-m", "base"})
	commands.Branch([]string{"feature"})

	if err := os.WriteFile("conflict.txt", []byte("main change\n"), 0644); err != nil {
		t.Fatal(err)
	}
	commands.Add([]string{"conflict.txt"})
	commands.Commit([]string{"-m", "main change"})
	mainCommit := readBranchCommit(t, "main")

	commands.Checkout([]string{"feature"})
	if err := os.WriteFile("conflict.txt", []byte("feature change\n"), 0644); err != nil {
		t.Fatal(err)
	}
	commands.Add([]string{"conflict.txt"})
	commands.Commit([]string{"-m", "feature change"})
	featureCommit := readBranchCommit(t, "feature")

	commands.Checkout([]string{"main"})
	output := CaptureOutput(func() {
		commands.Merge([]string{"feature"})
	})

	if !strings.Contains(output, "Automatic merge failed; fix conflicts and commit the result") {
		t.Fatalf("expected conflict message, got: %s", output)
	}

	mergedText := readFileString(t, "conflict.txt")
	if !strings.Contains(mergedText, "<<<<<<< HEAD") || !strings.Contains(mergedText, "=======") || !strings.Contains(mergedText, ">>>>>>> feature") {
		t.Fatalf("expected conflict markers in file, got: %q", mergedText)
	}

	if _, err := os.Stat(filepath.Join(".loki", "MERGE_HEAD")); err != nil {
		t.Fatalf("expected MERGE_HEAD to exist after conflict merge: %v", err)
	}

	if err := os.WriteFile("conflict.txt", []byte("resolved\n"), 0644); err != nil {
		t.Fatal(err)
	}
	commands.Add([]string{"conflict.txt"})
	commitOutput := CaptureOutput(func() {
		commands.Commit([]string{"-m", "merge resolved"})
	})
	if !strings.Contains(commitOutput, "Committed:") {
		t.Fatalf("expected merge resolution commit output, got: %s", commitOutput)
	}

	mergeCommit := readBranchCommit(t, "main")
	parents := readCommitParents(t, mergeCommit)
	if len(parents) != 2 {
		t.Fatalf("expected merge resolution commit to have 2 parents, got %d", len(parents))
	}
	if !(parents[0] == mainCommit && parents[1] == featureCommit) && !(parents[0] == featureCommit && parents[1] == mainCommit) {
		t.Fatalf("unexpected merge parents after conflict resolution: %+v", parents)
	}

	if _, err := os.Stat(filepath.Join(".loki", "MERGE_HEAD")); !os.IsNotExist(err) {
		t.Fatalf("expected MERGE_HEAD to be cleared after merge commit, got: %v", err)
	}
}
