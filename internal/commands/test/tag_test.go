package test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"loki/internal/commands"
	"loki/internal/models"
	"loki/internal/storage"
)

func setupTestRepo(t *testing.T) (string, func()) {
	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current dir: %v", err)
	}

	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to chdir to tempDir: %v", err)
	}

	commands.Init()

	cleanup := func() {
		os.Chdir(oldCwd)
	}
	return tempDir, cleanup
}

func commitSampleFile(t *testing.T, filename, content, commitMsg string) {
	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write file %s: %v", filename, err)
	}
	commands.Add([]string{filename})
	commands.Commit([]string{"-m", commitMsg})
}

func TestTag_ListEmpty(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	output := CaptureOutput(func() {
		commands.Tag([]string{})
	})

	if strings.TrimSpace(output) != "" {
		t.Errorf("Expected empty output for empty tags list, got: %s", output)
	}
}

func TestTag_NoCommits(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	output := CaptureOutput(func() {
		commands.Tag([]string{"v1.0"})
	})

	if !strings.Contains(output, "No commits yet") {
		t.Errorf("Expected 'No commits yet' message, got: %s", output)
	}

	// Verify tag file was not created
	if _, err := os.Stat(".loki/refs/tags/v1.0"); !os.IsNotExist(err) {
		t.Errorf("Tag file should not exist when no commits are made")
	}
}

func TestTag_Lightweight(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	commitSampleFile(t, "file1.txt", "hello world", "Initial commit")

	// Create lightweight tag
	output := CaptureOutput(func() {
		commands.Tag([]string{"v1.0"})
	})
	if strings.Contains(output, "error") || strings.Contains(output, "fatal") {
		t.Fatalf("Unexpected error creating tag: %s", output)
	}

	// Verify file exists in .loki/refs/tags/
	tagPath := filepath.Join(".loki", "refs", "tags", "v1.0")
	data, err := os.ReadFile(tagPath)
	if err != nil {
		t.Fatalf("Tag file %s does not exist: %v", tagPath, err)
	}

	tagHash := strings.TrimSpace(string(data))
	if len(tagHash) != 40 {
		t.Errorf("Expected 40-char commit hash in tag file, got: %s", tagHash)
	}

	// Verify HEAD hash matches tag hash
	headRefData, _ := os.ReadFile(".loki/refs/heads/main")
	expectedHash := strings.TrimSpace(string(headRefData))
	if tagHash != expectedHash {
		t.Errorf("Expected tag to point to HEAD commit %s, got: %s", expectedHash, tagHash)
	}

	// Verify loki tag lists v1.0
	listOutput := CaptureOutput(func() {
		commands.Tag([]string{})
	})
	if !strings.Contains(listOutput, "v1.0") {
		t.Errorf("Expected list output to contain v1.0, got: %s", listOutput)
	}
}

func TestTag_ListAlphabetical(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	commitSampleFile(t, "a.txt", "content a", "Commit a")

	commands.Tag([]string{"v2.0"})
	commands.Tag([]string{"v1.0"})
	commands.Tag([]string{"v0.1"})
	commands.Tag([]string{"beta"})

	output := CaptureOutput(func() {
		commands.Tag([]string{})
	})

	lines := strings.Split(strings.TrimSpace(output), "\n")
	expected := []string{"beta", "v0.1", "v1.0", "v2.0"}
	if len(lines) != len(expected) {
		t.Fatalf("Expected %d tags, got %d. Output: %v", len(expected), len(lines), lines)
	}
	for i, name := range expected {
		if lines[i] != name {
			t.Errorf("Index %d: expected tag %s, got %s", i, name, lines[i])
		}
	}
}

func TestTag_AlreadyExists(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	commitSampleFile(t, "test.txt", "version 1", "Commit 1")
	commands.Tag([]string{"v1.0"})

	// Try creating it again without -f
	output := CaptureOutput(func() {
		commands.Tag([]string{"v1.0"})
	})

	if !strings.Contains(output, "already exists") {
		t.Errorf("Expected 'already exists' error, got: %s", output)
	}

	// Try with -f
	commitSampleFile(t, "test.txt", "version 2", "Commit 2")
	outputForce := CaptureOutput(func() {
		commands.Tag([]string{"-f", "v1.0"})
	})
	if strings.Contains(outputForce, "already exists") {
		t.Errorf("Expected force tag to succeed, got: %s", outputForce)
	}

	headRefData, _ := os.ReadFile(".loki/refs/heads/main")
	newHash := strings.TrimSpace(string(headRefData))
	tagData, _ := os.ReadFile(".loki/refs/tags/v1.0")
	if strings.TrimSpace(string(tagData)) != newHash {
		t.Errorf("Expected tag to be updated to %s, got %s", newHash, string(tagData))
	}
}

func TestTag_Delete(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	commitSampleFile(t, "file.txt", "sample", "Commit")
	commands.Tag([]string{"v1.0"})

	// Delete existing tag
	output := CaptureOutput(func() {
		commands.Tag([]string{"-d", "v1.0"})
	})
	if !strings.Contains(output, "Deleted tag v1.0") {
		t.Errorf("Expected 'Deleted tag v1.0', got: %s", output)
	}

	if _, err := os.Stat(".loki/refs/tags/v1.0"); !os.IsNotExist(err) {
		t.Errorf("Tag file should have been removed")
	}

	// Delete non-existent tag
	outputNonExistent := CaptureOutput(func() {
		commands.Tag([]string{"-d", "nonexistent"})
	})
	if !strings.Contains(outputNonExistent, "Error deleting tag nonexistent") {
		t.Errorf("Expected error deleting nonexistent tag, got: %s", outputNonExistent)
	}
}

func TestTag_Annotated(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	commitSampleFile(t, "release.txt", "Release Notes 1.0", "Release commit")

	headRefData, _ := os.ReadFile(".loki/refs/heads/main")
	commitHash := strings.TrimSpace(string(headRefData))

	output := CaptureOutput(func() {
		commands.Tag([]string{"-a", "v1.0.0", "-m", "First production release"})
	})
	if strings.Contains(output, "error") || strings.Contains(output, "fatal") {
		t.Fatalf("Unexpected error creating annotated tag: %s", output)
	}

	// Verify tag file contains tag object hash
	tagPath := filepath.Join(".loki", "refs", "tags", "v1.0.0")
	tagData, err := os.ReadFile(tagPath)
	if err != nil {
		t.Fatalf("Tag ref file not created: %v", err)
	}
	tagObjHash := strings.TrimSpace(string(tagData))

	// Verify tag object hash is NOT equal to commit hash
	if tagObjHash == commitHash {
		t.Errorf("Annotated tag should store tag object hash, not raw commit hash")
	}

	// Verify tag object exists in .loki/objects/
	store := storage.NewFileStorage(".loki")
	objRaw, err := store.ReadObject(tagObjHash)
	if err != nil {
		t.Fatalf("Tag object not found in store: %v", err)
	}

	if !bytes.HasPrefix(objRaw, []byte("tag ")) {
		t.Fatalf("Object does not have tag header: %s", string(objRaw))
	}

	tagModel, err := models.ParseTag(objRaw)
	if err != nil {
		t.Fatalf("Failed to parse tag object: %v", err)
	}

	if tagModel.Object != commitHash {
		t.Errorf("Expected target object %s, got %s", commitHash, tagModel.Object)
	}
	if tagModel.Name != "v1.0.0" {
		t.Errorf("Expected tag name v1.0.0, got %s", tagModel.Name)
	}
	if tagModel.Message != "First production release" {
		t.Errorf("Expected message 'First production release', got %s", tagModel.Message)
	}
}

func TestTag_Checkout(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	commitSampleFile(t, "app.txt", "v1 code", "Commit 1")
	commands.Tag([]string{"v1.0"})

	commitSampleFile(t, "app.txt", "v2 code", "Commit 2")
	commands.Tag([]string{"-a", "v2.0", "-m", "Version 2 release"})

	// Checkout lightweight tag
	outputCheckout1 := CaptureOutput(func() {
		commands.Checkout([]string{"v1.0"})
	})
	if !strings.Contains(outputCheckout1, "Successfully checked out v1.0") {
		t.Errorf("Failed to checkout lightweight tag: %s", outputCheckout1)
	}

	content1, _ := os.ReadFile("app.txt")
	if string(content1) != "v1 code" {
		t.Errorf("Expected 'v1 code' after checkout v1.0, got: %s", string(content1))
	}

	// Checkout annotated tag
	outputCheckout2 := CaptureOutput(func() {
		commands.Checkout([]string{"v2.0"})
	})
	if !strings.Contains(outputCheckout2, "Successfully checked out v2.0") {
		t.Errorf("Failed to checkout annotated tag: %s", outputCheckout2)
	}

	content2, _ := os.ReadFile("app.txt")
	if string(content2) != "v2 code" {
		t.Errorf("Expected 'v2 code' after checkout v2.0, got: %s", string(content2))
	}
}

func TestTag_SpecificCommit(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	commitSampleFile(t, "a.txt", "commit 1", "Commit 1")
	head1, _ := os.ReadFile(".loki/refs/heads/main")
	commit1Hash := strings.TrimSpace(string(head1))

	commitSampleFile(t, "a.txt", "commit 2", "Commit 2")

	// Tag commit1 explicitly while HEAD is on commit 2
	commands.Tag([]string{"v1.0", commit1Hash})

	tagData, _ := os.ReadFile(".loki/refs/tags/v1.0")
	if strings.TrimSpace(string(tagData)) != commit1Hash {
		t.Errorf("Expected tag to point to commit1 %s, got: %s", commit1Hash, string(tagData))
	}
}
