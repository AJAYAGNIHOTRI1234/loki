package core

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"loki/internal/models"
	"loki/internal/storage"
	"loki/internal/utils"
	"os"
	"path/filepath"
	"strings"
)

type Repository struct {
	store  *storage.FileStorage
	index  *Index
	ignore *IgnoreMatcher
	root   string
}

func (r *Repository) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func OpenRepository() *Repository {
	cwd, err := os.Getwd()
	if err != nil {
		panic(utils.ColorText("Could not get current working directory", "error"))
	}
	repoRoot, ok := IsRepoInitialized(cwd + string(os.PathSeparator))
	if !ok {
		fmt.Fprintln(os.Stderr, utils.ColorText("fatal: not a loki repository (or any of the parent directories)", "error"))
		os.Exit(1)
	}
	return &Repository{
		store:  storage.NewFileStorage(filepath.Join(repoRoot, ".loki")),
		index:  LoadIndex(),
		ignore: LoadIgnore(repoRoot),
		root:   repoRoot,
	}
}

// Check for loki repo
func IsRepoInitialized(path string) (string, bool) {
	cur_path := path
	for {
		loki_check := filepath.Join(cur_path, ".loki")

		if info, err := os.Stat(loki_check); err == nil && info.IsDir() {
			return cur_path, true
		}

		parent := filepath.Dir(cur_path)

		if parent == cur_path {
			break
		}

		cur_path = parent
	}

	return "", false
}

func (r *Repository) IsIgnored(path string) bool {
	return r.ignore.Matches(path)
}

func (r *Repository) RemoveFile(path string) {
	r.index.Remove(path)
	r.index.Save()
}

// Detects and sets status: "new file", "modified", or "deleted"
func (r *Repository) AddFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	blob := &models.Blob{Content: data}
	hash := r.store.WriteObject(blob.Serialize())

	r.index.Add(path, hash)
	r.index.Save()

	return nil
}

// Helper: get last commit's tree (if any)
func (r *Repository) getLastCommitTree() *models.Tree {
	// Try to read HEAD ref
	headData, err := os.ReadFile(".loki/HEAD")
	if err != nil {
		return nil
	}
	ref := string(bytes.TrimSpace(headData))
	var commitHash string
	if len(ref) >= 5 && ref[:4] == "ref:" {
		refPath := ".loki/" + ref[5:]
		refHashData, err := os.ReadFile(refPath)
		if err != nil {
			return nil
		}
		commitHash = string(bytes.TrimSpace(refHashData))
	} else {
		commitHash = ref
	}
	// Read commit object
	objData, err := r.store.ReadObject(commitHash)
	if err != nil {
		return nil
	}
	// Parse commit to get tree hash
	idx := bytes.IndexByte(objData, 0)
	if idx >= 0 {
		objData = objData[idx+1:]
	}
	var treeHash string
	for _, line := range bytes.Split(objData, []byte("\n")) {
		if bytes.HasPrefix(line, []byte("tree ")) {
			treeHash = string(line[5:])
			break
		}
	}
	if treeHash == "" {
		return nil
	}
	// Read tree object
	treeData, err := r.store.ReadObject(treeHash)
	if err != nil {
		return nil
	}
	// Parse tree entries
	entries := []models.TreeEntry{}
	// Skip header ("tree <len>\0")
	idx = bytes.IndexByte(treeData, 0)
	if idx < 0 {
		return nil
	}
	treeContent := treeData[idx+1:]
	for len(treeContent) > 0 {
		// Format: mode name\0hash(20 bytes)
		sp := bytes.IndexByte(treeContent, ' ')
		if sp < 0 {
			break
		}
		mode := string(treeContent[:sp])
		treeContent = treeContent[sp+1:]
		nul := bytes.IndexByte(treeContent, 0)
		if nul < 0 {
			break
		}
		name := string(treeContent[:nul])
		if len(treeContent) < nul+21 {
			break
		}
		hash := treeContent[nul+1 : nul+21]
		entries = append(entries, models.TreeEntry{Mode: mode, Name: name, Hash: hash})
		treeContent = treeContent[nul+21:]
	}
	return &models.Tree{Entries: entries}
}

func parseObject(data []byte) (string, []byte, error) {
	idx := bytes.IndexByte(data, 0)
	if idx < 0 {
		return "", nil, fmt.Errorf("invalid object: missing header separator")
	}
	header := string(data[:idx])
	content := data[idx+1:]
	parts := strings.Split(header, " ")
	if len(parts) < 2 {
		return "", nil, fmt.Errorf("invalid object header format")
	}
	return parts[0], content, nil
}

// ReadBlob retrieves raw blob content by its hash.
func (r *Repository) ReadBlob(hash string) (string, error) {
	data, err := r.store.ReadObject(hash)
	if err != nil {
		return "", err
	}
	objType, content, err := parseObject(data)
	if err != nil {
		return "", err
	}
	if objType != "blob" {
		return "", fmt.Errorf("object is not a blob: %s", objType)
	}
	return string(content), nil
}

// GetHeadTreeEntries retrieves the file path to hex hash mappings for the HEAD commit.
func (r *Repository) GetHeadTreeEntries() (map[string]string, error) {
	tree := r.getLastCommitTree()
	entries := make(map[string]string)
	if tree == nil {
		return entries, nil
	}
	for _, entry := range tree.Entries {
		entries[entry.Name] = hex.EncodeToString(entry.Hash)
	}
	return entries, nil
}

// GetIndexEntries retrieves the current index entries.
func (r *Repository) GetIndexEntries() map[string]string {
	entries := make(map[string]string)
	for k, v := range r.index.Entries {
		entries[k] = v
	}
	return entries
}

func (r *Repository) Commit(message, author, email string) string {
	// Write the tree and get its hash
	treeHash := r.index.WriteTree(r.store)
	parents := []string{}
	if headCommit, err := r.resolveHeadCommitHash(); err == nil && headCommit != "" {
		parents = append(parents, headCommit)
	}
	if mergeHead, err := r.readMergeHead(); err == nil && mergeHead != "" {
		if err := r.ensureIndexMatchesWorkingTree(); err != nil {
			fmt.Println(utils.ColorText("error: "+err.Error(), "error"))
			return ""
		}
		if len(parents) == 0 {
			parents = []string{mergeHead}
		} else if parents[0] != mergeHead {
			parents = append(parents, mergeHead)
		}
	}

	// Create a proper Commit object using the model
	commitModel := &models.Commit{
		Tree:    treeHash,
		Parents: parents,
		Message: message,
		Author:  author,
		Email:   email,
	}

	// Serialize and write using the standard WriteObject (Git-style)
	commitHash := r.store.WriteObject(commitModel.Serialize())

	// Update the log
	f, _ := os.OpenFile(filepath.Join(r.store.GiveRoot(), "commits.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()
	f.WriteString(commitHash + " " + message + " " + author + " <" + email + ">\n")

	// update branch ref so getLastCommitTree works
	headData, err := os.ReadFile(".loki/HEAD")
	if err == nil {
		ref := string(bytes.TrimSpace(headData))
		if len(ref) >= 5 && ref[:4] == "ref:" {
			refPath := ".loki/" + ref[5:]
			os.MkdirAll(filepath.Dir(refPath), 0755)
			os.WriteFile(refPath, []byte(commitHash+"\n"), 0644)
		} else {
			os.WriteFile(".loki/HEAD", []byte(commitHash+"\n"), 0644)
		}
	}
	_ = r.clearMergeState()
	_ = r.clearRevertState()

	return commitHash
}

func (r *Repository) Revert(commitHash, author, email string) (string, error) {
	targetCommit, err := r.readCommitData(commitHash)
	if err != nil {
		return "", err
	}

	if len(targetCommit.parents) > 1 {
		return "", fmt.Errorf("reverting merge commits is not supported")
	}

	parentFiles := map[string]string{}
	if len(targetCommit.parents) == 1 {
		parentFiles, err = r.readCommitFileMap(targetCommit.parents[0])
		if err != nil {
			return "", err
		}
	}

	targetFiles, err := r.readCommitFileMap(commitHash)
	if err != nil {
		return "", err
	}

	currentFiles, err := r.currentTrackedFiles()
	if err != nil {
		return "", err
	}

	if err := r.ensureCleanForCheckout(); err != nil {
		return "", err
	}
	if err := r.ensureIndexMatchesWorkingTree(); err != nil {
		return "", err
	}

	paths := map[string]struct{}{}
	for path := range parentFiles {
		paths[path] = struct{}{}
	}
	for path := range targetFiles {
		paths[path] = struct{}{}
	}

	conflictPaths := make([]string, 0)
	appliedChanges := false
	for path := range paths {
		parentHash, parentOK := parentFiles[path]
		targetHash, targetOK := targetFiles[path]
		currentHash, currentOK := currentFiles[path]

		if parentOK == targetOK && parentHash == targetHash {
			continue
		}

		if currentOK == targetOK && currentHash == targetHash {
			if parentOK {
				blobData, err := r.store.ReadObject(parentHash)
				if err != nil {
					return "", fmt.Errorf("failed to read blob %s: %v", parentHash, err)
				}
				content := objectBody(blobData)
				osPath := filepath.FromSlash(path)
				if err := os.MkdirAll(filepath.Dir(osPath), 0755); err != nil {
					return "", fmt.Errorf("failed to create directory for %s: %v", path, err)
				}
				if err := os.WriteFile(osPath, content, 0644); err != nil {
					return "", fmt.Errorf("failed to write file %s: %v", path, err)
				}
				r.index.Add(path, parentHash)
			} else {
				_ = os.Remove(filepath.FromSlash(path))
				r.index.Remove(path)
			}
			appliedChanges = true
			continue
		}

		if currentOK == parentOK && (!currentOK || currentHash == parentHash) {
			continue
		}
		if !parentOK {
			conflictPaths = append(conflictPaths, path)
			continue
		}

		currentContent := ""
		if currentOK {
			currentBlob, err := r.store.ReadObject(currentHash)
			if err != nil {
				return "", fmt.Errorf("failed to read blob %s: %v", currentHash, err)
			}
			currentContent = string(objectBody(currentBlob))
		}

		parentBlob, err := r.store.ReadObject(parentHash)
		if err != nil {
			return "", fmt.Errorf("failed to read blob %s: %v", parentHash, err)
		}
		parentContent := string(objectBody(parentBlob))

		conflictContent := makeRevertConflictMarker(currentContent, parentContent)
		osPath := filepath.FromSlash(path)
		if err := os.MkdirAll(filepath.Dir(osPath), 0755); err != nil {
			return "", fmt.Errorf("failed to create directory for %s: %v", path, err)
		}
		if err := os.WriteFile(osPath, []byte(conflictContent), 0644); err != nil {
			return "", fmt.Errorf("failed to write file %s: %v", path, err)
		}
		appliedChanges = true
		conflictPaths = append(conflictPaths, path)
	}

	if appliedChanges {
		r.index.Save()
	}

	if len(conflictPaths) > 0 {
		if err := r.writeRevertState(commitHash, conflictPaths); err != nil {
			return "", err
		}
		return "", fmt.Errorf("CONFLICT (modify/delete): %s deleted in revert and modified in HEAD. Version HEAD left in tree.", strings.Join(conflictPaths, ", "))
	}

	if !appliedChanges {
		return "", fmt.Errorf("nothing to revert")
	}

	message := "Revert: " + targetCommit.message
	if message == "Revert: " {
		message = "Revert commit"
	}
	_ = r.clearRevertState()
	return r.Commit(message, author, email), nil
}

func makeRevertConflictMarker(currentContent, parentContent string) string {
	return fmt.Sprintf("<<<<<<< CURRENT\n%s=======\n%s>>>>>>> REVERT\n", currentContent, parentContent)
}

func (r *Repository) writeRevertState(commitHash string, conflicts []string) error {
	if err := os.WriteFile(".loki/REVERT_HEAD", []byte(commitHash+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to write revert state: %v", err)
	}
	if err := os.WriteFile(".loki/REVERT_CONFLICTS", []byte(strings.Join(conflicts, "\n")+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to write revert state: %v", err)
	}
	return nil
}

func (r *Repository) clearRevertState() error {
	_ = os.Remove(".loki/REVERT_HEAD")
	_ = os.Remove(".loki/REVERT_CONFLICTS")
	return nil
}

func (r *Repository) Status() []FileStatus {
	headFiles := map[string]string{}
	headCommit, err := r.resolveHeadCommitHash()
	if err == nil && headCommit != "" {
		if headTreeHash, err := r.commitTreeHash(headCommit); err == nil && headTreeHash != "" {
			_ = r.collectTreeFiles(headTreeHash, "", headFiles)
		}
	}

	indexFiles := r.index.Entries

	wdFiles := map[string]string{}
	filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path == ".loki" || r.IsIgnored(path) {
				return filepath.SkipDir
			}
			return nil
		}
		if r.IsIgnored(path) {
			return nil
		}

		path = filepath.ToSlash(path)
		wdFiles[path] = ""
		return nil
	})

	var results []FileStatus

	// 1. Index vs HEAD (Staged changes)
	for path, indexHash := range indexFiles {
		headHash, inHead := headFiles[path]
		if !inHead {
			results = append(results, FileStatus{Name: path, Status: "staged (added)"})
		} else if indexHash != headHash {
			results = append(results, FileStatus{Name: path, Status: "staged (modified)"})
		}
	}
	for path := range headFiles {
		if _, inIndex := indexFiles[path]; !inIndex {
			results = append(results, FileStatus{Name: path, Status: "staged (deleted)"})
		}
	}

	// 2. WD vs Index (Unstaged changes)
	for path := range indexFiles {
		if _, inWd := wdFiles[path]; !inWd {
			results = append(results, FileStatus{Name: path, Status: "deleted"})
		} else {
			content, err := os.ReadFile(path)
			if err == nil {
				wdHash := blobHashForContent(content)
				if wdHash != indexFiles[path] {
					results = append(results, FileStatus{Name: path, Status: "modified"})
				}
			}
		}
	}

	// 3. Untracked files (In WD, but not in index)
	for path := range wdFiles {
		if _, inIndex := indexFiles[path]; !inIndex {
			results = append(results, FileStatus{Name: path, Status: "untracked"})
		}
	}

	return results
}

func (r *Repository) PrintLog() {
	logs, err := os.ReadFile(filepath.Join(r.store.GiveRoot(), "commits.log"))
	if err != nil {
		fmt.Println(utils.ColorText("No commit found.", "error"))
		return
	}
	lines := strings.Split(string(logs), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, " ")
		if len(parts) >= 4 {
			hash := parts[0]
			email := parts[len(parts)-1]
			author := parts[len(parts)-2]
			msg := strings.Join(parts[1:len(parts)-2], " ")
			fmt.Printf(utils.ColorText("Commit: %s\nMessage: %s\nAuthor: %s\n%s\n\n", "info"), hash, msg, author, email)
		} else {
			p := strings.SplitN(line, " ", 2)
			if len(p) >= 2 {
				fmt.Printf(utils.ColorText("Commit: %s\n%s\n\n", "info"), p[0], p[1])
			}
		}
	}
}

func (r *Repository) Checkout(target string) error {
	commitHash, headContent, err := r.resolveCheckoutTarget(target)
	if err != nil {
		return err
	}

	if err := r.ensureCleanForCheckout(); err != nil {
		return err
	}

	if err := r.applyCommitToWorkingTree(commitHash); err != nil {
		return err
	}

	if err := os.WriteFile(".loki/HEAD", []byte(headContent+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to update HEAD: %v", err)
	}

	return nil
}

func objectBody(data []byte) []byte {
	idx := bytes.IndexByte(data, 0)
	if idx >= 0 {
		return data[idx+1:]
	}
	return data
}

func normalizeRepoPath(path string) string {
	cleaned := filepath.Clean(path)
	if cleaned == "." {
		return ""
	}
	return filepath.ToSlash(cleaned)
}

func blobHashForContent(data []byte) string {
	blob := (&models.Blob{Content: data}).Serialize()
	sum := sha1.Sum(blob)
	return hex.EncodeToString(sum[:])
}

func sameFileMap(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func (r *Repository) resolveHeadCommitHash() (string, error) {
	headData, err := os.ReadFile(".loki/HEAD")
	if err != nil {
		return "", err
	}
	ref := strings.TrimSpace(string(headData))
	if strings.HasPrefix(ref, "ref: ") {
		refPath := filepath.Join(".loki", strings.TrimPrefix(ref, "ref: "))
		refHashData, err := os.ReadFile(refPath)
		if err != nil {
			if os.IsNotExist(err) {
				return "", nil
			}
			return "", err
		}
		return strings.TrimSpace(string(refHashData)), nil
	}
	return ref, nil
}

type commitData struct {
	tree    string
	parents []string
	message string
}

func (r *Repository) readCommitData(commitHash string) (*commitData, error) {
	objData, err := r.store.ReadObject(commitHash)
	if err != nil {
		return nil, fmt.Errorf("target commit %s not found", commitHash)
	}
	body := objectBody(objData)
	data := &commitData{parents: []string{}}
	parts := bytes.SplitN(body, []byte("\n\n"), 2)
	for _, line := range bytes.Split(parts[0], []byte("\n")) {
		if bytes.HasPrefix(line, []byte("tree ")) {
			data.tree = strings.TrimSpace(string(line[5:]))
			continue
		}
		if bytes.HasPrefix(line, []byte("parent ")) {
			data.parents = append(data.parents, strings.TrimSpace(string(line[7:])))
			continue
		}
	}
	if len(parts) == 2 {
		data.message = strings.TrimSpace(string(parts[1]))
	}
	if data.tree == "" {
		return nil, fmt.Errorf("commit %s has no tree", commitHash)
	}
	return data, nil
}

func (r *Repository) commitTreeHash(commitHash string) (string, error) {
	data, err := r.readCommitData(commitHash)
	if err != nil {
		return "", err
	}
	return data.tree, nil
}

func (r *Repository) commitParents(commitHash string) ([]string, error) {
	data, err := r.readCommitData(commitHash)
	if err != nil {
		return nil, err
	}
	parents := make([]string, 0, len(data.parents))
	for _, parent := range data.parents {
		if parent != "" {
			parents = append(parents, parent)
		}
	}
	return parents, nil
}

func (r *Repository) currentTrackedFiles() (map[string]string, error) {
	tracked := map[string]string{}
	headCommit, err := r.resolveHeadCommitHash()
	if err != nil {
		return nil, err
	}
	if headCommit == "" {
		return tracked, nil
	}
	headTreeHash, err := r.commitTreeHash(headCommit)
	if err != nil {
		return nil, err
	}
	if err := r.collectTreeFiles(headTreeHash, "", tracked); err != nil {
		return nil, err
	}
	return tracked, nil
}

func (r *Repository) readMergeHead() (string, error) {
	data, err := os.ReadFile(".loki/MERGE_HEAD")
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (r *Repository) readMergeConflicts() ([]string, error) {
	data, err := os.ReadFile(".loki/MERGE_CONFLICTS")
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	conflicts := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			conflicts = append(conflicts, line)
		}
	}
	return conflicts, nil
}

func (r *Repository) clearMergeState() error {
	_ = os.Remove(".loki/MERGE_HEAD")
	_ = os.Remove(".loki/MERGE_CONFLICTS")
	return nil
}

func (r *Repository) ensureIndexMatchesWorkingTree() error {
	for path, indexHash := range r.index.Entries {
		content, err := os.ReadFile(filepath.FromSlash(path))
		if err != nil {
			return fmt.Errorf("working tree has unresolved changes for %s", path)
		}
		if blobHashForContent(content) != indexHash {
			return fmt.Errorf("working tree has unresolved changes for %s", path)
		}
	}
	return nil
}

func (r *Repository) replaceWorkingTreeWithFiles(target map[string]string) error {
	currentTracked, err := r.currentTrackedFiles()
	if err != nil {
		return err
	}

	for path := range currentTracked {
		if _, ok := target[path]; !ok {
			_ = os.Remove(filepath.FromSlash(path))
		}
	}

	r.index.Entries = make(map[string]string)
	for path, blobHash := range target {
		blobData, err := r.store.ReadObject(blobHash)
		if err != nil {
			return fmt.Errorf("failed to read blob %s: %v", blobHash, err)
		}
		content := objectBody(blobData)

		osPath := filepath.FromSlash(path)
		if err := os.MkdirAll(filepath.Dir(osPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %v", path, err)
		}
		if err := os.WriteFile(osPath, content, 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %v", path, err)
		}
		r.index.Add(path, blobHash)
	}
	r.index.Save()

	return nil
}

func (r *Repository) applyMergeResult(cleanFiles map[string]string, conflictMarkers map[string]string) error {
	currentTracked, err := r.currentTrackedFiles()
	if err != nil {
		return err
	}

	newIndex := make(map[string]string, len(r.index.Entries))
	for path, hash := range r.index.Entries {
		newIndex[path] = hash
	}

	for path := range currentTracked {
		if _, keep := cleanFiles[path]; keep {
			continue
		}
		if _, conflicted := conflictMarkers[path]; conflicted {
			continue
		}
		_ = os.Remove(filepath.FromSlash(path))
		delete(newIndex, path)
	}

	for path, blobHash := range cleanFiles {
		blobData, err := r.store.ReadObject(blobHash)
		if err != nil {
			return fmt.Errorf("failed to read blob %s: %v", blobHash, err)
		}
		content := objectBody(blobData)

		osPath := filepath.FromSlash(path)
		if err := os.MkdirAll(filepath.Dir(osPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %v", path, err)
		}
		if err := os.WriteFile(osPath, content, 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %v", path, err)
		}
		newIndex[path] = blobHash
	}

	for path, content := range conflictMarkers {
		osPath := filepath.FromSlash(path)
		if err := os.MkdirAll(filepath.Dir(osPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %v", path, err)
		}
		if err := os.WriteFile(osPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %v", path, err)
		}
	}

	r.index.Entries = newIndex
	r.index.Save()
	return nil
}

func (r *Repository) resolveBranchCommitHash(branch string) (string, error) {
	refPath := filepath.Join(".loki", "refs", "heads", branch)
	if _, err := os.Stat(refPath); err != nil {
		if os.IsNotExist(err) {
			tagPath := filepath.Join(".loki", "refs", "tags", branch)
			if tagHashData, err := os.ReadFile(tagPath); err == nil {
				refHash := strings.TrimSpace(string(tagHashData))
				if objData, err := r.store.ReadObject(refHash); err == nil && bytes.HasPrefix(objData, []byte("tag ")) {
					if tagObj, err := models.ParseTag(objData); err == nil && tagObj.Object != "" {
						return tagObj.Object, nil
					}
				}
				if refHash != "" {
					return refHash, nil
				}
			}
			return "", fmt.Errorf("branch %s not found", branch)
		}
		return "", fmt.Errorf("failed to read branch %s: %v", branch, err)
	}
	hashData, err := os.ReadFile(refPath)
	if err != nil {
		return "", fmt.Errorf("failed to read branch %s: %v", branch, err)
	}
	commitHash := strings.TrimSpace(string(hashData))
	if commitHash == "" {
		return "", fmt.Errorf("branch %s has no commit", branch)
	}
	return commitHash, nil
}

func (r *Repository) isAncestor(ancestor, descendant string) (bool, error) {
	if ancestor == "" {
		return descendant == "", nil
	}
	if ancestor == descendant {
		return true, nil
	}
	if descendant == "" {
		return false, nil
	}

	visited := map[string]bool{}
	queue := []string{descendant}
	for len(queue) > 0 {
		commitHash := queue[0]
		queue = queue[1:]
		if visited[commitHash] {
			continue
		}
		visited[commitHash] = true
		if commitHash == ancestor {
			return true, nil
		}
		parents, err := r.commitParents(commitHash)
		if err != nil {
			return false, err
		}
		queue = append(queue, parents...)
	}

	return false, nil
}

func (r *Repository) ancestorDistances(start string) (map[string]int, error) {
	distances := map[string]int{}
	if start == "" {
		return distances, nil
	}

	queue := []string{start}
	distances[start] = 0

	for len(queue) > 0 {
		commitHash := queue[0]
		queue = queue[1:]
		parents, err := r.commitParents(commitHash)
		if err != nil {
			return nil, err
		}
		for _, parent := range parents {
			if parent == "" {
				continue
			}
			if _, ok := distances[parent]; ok {
				continue
			}
			distances[parent] = distances[commitHash] + 1
			queue = append(queue, parent)
		}
	}

	return distances, nil
}

func (r *Repository) mergeBase(left, right string) (string, error) {
	leftDistances, err := r.ancestorDistances(left)
	if err != nil {
		return "", err
	}
	if right == "" {
		return "", fmt.Errorf("no common ancestor found")
	}

	type queueItem struct {
		commit string
		dist   int
	}

	bestCommit := ""
	bestScore := int(^uint(0) >> 1)
	visited := map[string]bool{}
	queue := []queueItem{{commit: right, dist: 0}}

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]
		if visited[item.commit] {
			continue
		}
		visited[item.commit] = true
		if leftDist, ok := leftDistances[item.commit]; ok {
			score := leftDist + item.dist
			if score < bestScore {
				bestScore = score
				bestCommit = item.commit
			}
		}
		parents, err := r.commitParents(item.commit)
		if err != nil {
			return "", err
		}
		for _, parent := range parents {
			if parent == "" {
				continue
			}
			queue = append(queue, queueItem{commit: parent, dist: item.dist + 1})
		}
	}

	if bestCommit == "" {
		return "", fmt.Errorf("no common ancestor found")
	}
	return bestCommit, nil
}

func (r *Repository) mergeTreeMaps(base, current, source map[string]string, branch string) (map[string]string, map[string]string, error) {
	merged := map[string]string{}
	conflicts := map[string]string{}
	paths := map[string]struct{}{}
	for path := range base {
		paths[path] = struct{}{}
	}
	for path := range current {
		paths[path] = struct{}{}
	}
	for path := range source {
		paths[path] = struct{}{}
	}

	for path := range paths {
		baseHash, baseOK := base[path]
		currentHash, currentOK := current[path]
		sourceHash, sourceOK := source[path]

		switch {
		case currentOK && sourceOK && currentHash == sourceHash:
			merged[path] = currentHash
		case !baseOK && currentOK && !sourceOK:
			merged[path] = currentHash
		case !baseOK && !currentOK && sourceOK:
			merged[path] = sourceHash
		case !baseOK && currentOK && sourceOK:
			conflicts[path] = r.makeConflictMarker(path, currentHash, sourceHash, branch)
		case baseOK && currentOK && !sourceOK:
			if currentHash == baseHash {
				continue
			}
			conflicts[path] = r.makeConflictMarker(path, currentHash, "", branch)
		case baseOK && !currentOK && sourceOK:
			if sourceHash == baseHash {
				continue
			}
			conflicts[path] = r.makeConflictMarker(path, "", sourceHash, branch)
		case baseOK && currentOK && sourceOK:
			currentChanged := currentHash != baseHash
			sourceChanged := sourceHash != baseHash
			switch {
			case !currentChanged && sourceChanged:
				merged[path] = sourceHash
			case currentChanged && !sourceChanged:
				merged[path] = currentHash
			case !currentChanged && !sourceChanged:
				merged[path] = currentHash
			case currentHash == sourceHash:
				merged[path] = currentHash
			default:
				conflicts[path] = r.makeConflictMarker(path, currentHash, sourceHash, branch)
			}
		case baseOK && !currentOK && !sourceOK:
			continue
		case !baseOK && currentOK && !sourceOK:
			merged[path] = currentHash
		case !baseOK && !currentOK && sourceOK:
			merged[path] = sourceHash
		default:
			continue
		}
	}

	return merged, conflicts, nil
}

func (r *Repository) makeConflictMarker(path, currentHash, sourceHash, branch string) string {
	currentContent := ""
	sourceContent := ""
	if currentHash != "" {
		if content, err := r.ReadBlob(currentHash); err == nil {
			currentContent = content
		}
	}
	if sourceHash != "" {
		if content, err := r.ReadBlob(sourceHash); err == nil {
			sourceContent = content
		}
	}
	return fmt.Sprintf("<<<<<<< HEAD\n%s=======\n%s>>>>>>> %s\n", currentContent, sourceContent, branch)
}

func (r *Repository) createCommit(message, author, email string, parents []string) string {
	treeHash := r.index.WriteTree(r.store)
	commitModel := &models.Commit{
		Tree:    treeHash,
		Parents: parents,
		Message: message,
		Author:  author,
		Email:   email,
	}

	commitHash := r.store.WriteObject(commitModel.Serialize())
	f, _ := os.OpenFile(filepath.Join(r.store.GiveRoot(), "commits.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()
	f.WriteString(commitHash + " " + message + " " + author + " <" + email + ">\n")

	headData, err := os.ReadFile(".loki/HEAD")
	if err == nil {
		ref := string(bytes.TrimSpace(headData))
		if len(ref) >= 5 && ref[:4] == "ref:" {
			refPath := ".loki/" + ref[5:]
			os.MkdirAll(filepath.Dir(refPath), 0755)
			os.WriteFile(refPath, []byte(commitHash+"\n"), 0644)
		} else {
			os.WriteFile(".loki/HEAD", []byte(commitHash+"\n"), 0644)
		}
	}

	_ = r.clearMergeState()

	return commitHash
}

func (r *Repository) applyCommitToWorkingTree(commitHash string) error {
	targetFiles, err := r.readCommitFileMap(commitHash)
	if err != nil {
		return err
	}
	return r.replaceWorkingTreeWithFiles(targetFiles)
}

func (r *Repository) readCommitFileMap(commitHash string) (map[string]string, error) {
	treeHash, err := r.commitTreeHash(commitHash)
	if err != nil {
		return nil, err
	}
	targetFiles := map[string]string{}
	if err := r.collectTreeFiles(treeHash, "", targetFiles); err != nil {
		return nil, err
	}
	return targetFiles, nil
}

func (r *Repository) MergeFastForward(branch string) (string, error) {
	sourceCommit, err := r.resolveBranchCommitHash(branch)
	if err != nil {
		return "", err
	}

	currentCommit, err := r.resolveHeadCommitHash()
	if err != nil {
		return "", fmt.Errorf("failed to resolve HEAD: %v", err)
	}

	if currentCommit == sourceCommit {
		return fmt.Sprintf("Already up to date with %s", branch), nil
	}

	isFastForward, err := r.isAncestor(currentCommit, sourceCommit)
	if err != nil {
		return "", err
	}
	if !isFastForward {
		return "", fmt.Errorf("cannot fast-forward merge %s into current branch", branch)
	}

	if err := r.ensureCleanForCheckout(); err != nil {
		return "", err
	}

	if err := r.applyCommitToWorkingTree(sourceCommit); err != nil {
		return "", err
	}

	if err := r.updateHeadOrRef(sourceCommit); err != nil {
		return "", fmt.Errorf("failed to update branch pointer: %v", err)
	}

	return fmt.Sprintf("Fast-forwarded current branch to %s", branch), nil
}

func (r *Repository) Merge(branch, author, email string) (string, error) {
	sourceCommit, err := r.resolveBranchCommitHash(branch)
	if err != nil {
		return "", err
	}

	currentCommit, err := r.resolveHeadCommitHash()
	if err != nil {
		return "", fmt.Errorf("failed to resolve HEAD: %v", err)
	}

	if currentCommit == sourceCommit {
		return fmt.Sprintf("Already up to date with %s", branch), nil
	}

	isCurrentAncestor, err := r.isAncestor(currentCommit, sourceCommit)
	if err != nil {
		return "", err
	}
	if isCurrentAncestor {
		return r.MergeFastForward(branch)
	}

	isSourceAncestor, err := r.isAncestor(sourceCommit, currentCommit)
	if err != nil {
		return "", err
	}
	if isSourceAncestor {
		return fmt.Sprintf("Already up to date with %s", branch), nil
	}

	if err := r.ensureCleanForCheckout(); err != nil {
		return "", err
	}

	baseCommit, err := r.mergeBase(currentCommit, sourceCommit)
	if err != nil {
		return "", err
	}

	baseFiles, err := r.readCommitFileMap(baseCommit)
	if err != nil {
		return "", err
	}
	currentFiles, err := r.readCommitFileMap(currentCommit)
	if err != nil {
		return "", err
	}
	sourceFiles, err := r.readCommitFileMap(sourceCommit)
	if err != nil {
		return "", err
	}

	mergedFiles, conflicts, err := r.mergeTreeMaps(baseFiles, currentFiles, sourceFiles, branch)
	if err != nil {
		return "", err
	}

	if len(conflicts) == 0 {
		if err := r.replaceWorkingTreeWithFiles(mergedFiles); err != nil {
			return "", err
		}
		parents := []string{currentCommit, sourceCommit}
		_ = r.createCommit("Merge branch '"+branch+"'", author, email, parents)
		return fmt.Sprintf("Merged %s into current branch", branch), nil
	}

	if err := r.applyMergeResult(mergedFiles, conflicts); err != nil {
		return "", err
	}
	if err := os.WriteFile(".loki/MERGE_HEAD", []byte(sourceCommit+"\n"), 0644); err != nil {
		return "", fmt.Errorf("failed to write merge state: %v", err)
	}
	conflictList := make([]string, 0, len(conflicts))
	for path := range conflicts {
		conflictList = append(conflictList, path)
	}
	if err := os.WriteFile(".loki/MERGE_CONFLICTS", []byte(strings.Join(conflictList, "\n")+"\n"), 0644); err != nil {
		return "", fmt.Errorf("failed to write merge state: %v", err)
	}

	return "Automatic merge failed; fix conflicts and commit the result", nil
}

func (r *Repository) resolveCheckoutTarget(target string) (string, string, error) {
	refPath := filepath.Join(".loki", "refs", "heads", target)
	if _, err := os.Stat(refPath); err == nil {
		hashData, err := os.ReadFile(refPath)
		if err != nil {
			return "", "", fmt.Errorf("failed to read branch file: %v", err)
		}
		commitHash := strings.TrimSpace(string(hashData))
		if commitHash == "" {
			return "", "", fmt.Errorf("branch %s has no commit", target)
		}
		return commitHash, "ref: refs/heads/" + target, nil
	}

	tagPath := filepath.Join(".loki", "refs", "tags", target)
	if _, err := os.Stat(tagPath); err == nil {
		hashData, err := os.ReadFile(tagPath)
		if err != nil {
			return "", "", fmt.Errorf("failed to read tag file: %v", err)
		}
		refHash := strings.TrimSpace(string(hashData))
		if refHash == "" {
			return "", "", fmt.Errorf("tag %s has no commit", target)
		}
		objData, err := r.store.ReadObject(refHash)
		if err == nil && bytes.HasPrefix(objData, []byte("tag ")) {
			tagObj, err := models.ParseTag(objData)
			if err == nil && tagObj.Object != "" {
				return tagObj.Object, tagObj.Object, nil
			}
		}
		return refHash, refHash, nil
	}

	return target, target, nil
}

func (r *Repository) collectTreeFiles(treeHash, prefix string, out map[string]string) error {
	treeData, err := r.store.ReadObject(treeHash)
	if err != nil {
		return fmt.Errorf("failed to read tree object %s: %v", treeHash, err)
	}
	body := objectBody(treeData)
	for len(body) > 0 {
		sp := bytes.IndexByte(body, ' ')
		if sp < 0 {
			break
		}
		mode := string(body[:sp])
		body = body[sp+1:]

		nul := bytes.IndexByte(body, 0)
		if nul < 0 {
			break
		}
		name := string(body[:nul])
		if len(body) < nul+21 {
			break
		}
		hashBytes := body[nul+1 : nul+21]
		hashHex := hex.EncodeToString(hashBytes)
		body = body[nul+21:]

		joined := name
		if prefix != "" {
			joined = prefix + "/" + name
		}
		joined = normalizeRepoPath(joined)

		if mode == "40000" {
			if err := r.collectTreeFiles(hashHex, joined, out); err != nil {
				return err
			}
			continue
		}

		if joined != "" {
			out[joined] = hashHex
		}
	}
	return nil
}

func (r *Repository) ensureCleanForCheckout() error {
	headFiles := map[string]string{}
	headCommit, err := r.resolveHeadCommitHash()
	if err != nil {
		return fmt.Errorf("failed to resolve HEAD: %v", err)
	}
	if headCommit != "" {
		headTreeHash, err := r.commitTreeHash(headCommit)
		if err != nil {
			return err
		}
		if err := r.collectTreeFiles(headTreeHash, "", headFiles); err != nil {
			return err
		}
	}

	for path, expectedHash := range headFiles {
		content, err := os.ReadFile(filepath.FromSlash(path))
		if err != nil {
			return fmt.Errorf("working directory is not clean, missing tracked file: %s", path)
		}
		if blobHashForContent(content) != expectedHash {
			return fmt.Errorf("working directory is not clean, unstaged changes in: %s", path)
		}
	}

	return nil
}

func (r *Repository) updateHeadOrRef(commitHash string) error {
	headData, err := os.ReadFile(".loki/HEAD")
	if err != nil {
		return err
	}
	ref := strings.TrimSpace(string(headData))
	if strings.HasPrefix(ref, "ref: ") {
		refPath := filepath.Join(".loki", strings.TrimPrefix(ref, "ref: "))
		return os.WriteFile(refPath, []byte(commitHash+"\n"), 0644)
	}
	return os.WriteFile(".loki/HEAD", []byte(commitHash+"\n"), 0644)
}

func (r *Repository) Reset(target string, mode string) error {
	if target == "" {
		target = "HEAD"
	}

	var commitHash string
	if target == "HEAD" {
		var err error
		commitHash, err = r.resolveHeadCommitHash()
		if err != nil || commitHash == "" {
			return fmt.Errorf("could not resolve HEAD")
		}
	} else {
		hash, _, err := r.resolveCheckoutTarget(target)
		if err != nil {
			return err
		}
		commitHash = hash
	}

	treeHash, err := r.commitTreeHash(commitHash)
	if err != nil {
		return err
	}

	// 1. Update branch pointer
	if err := r.updateHeadOrRef(commitHash); err != nil {
		return fmt.Errorf("failed to update branch pointer: %v", err)
	}

	if mode == "soft" {
		return nil
	}

	// 2. Mixed: Update index to match target tree
	targetFiles := map[string]string{}
	if err := r.collectTreeFiles(treeHash, "", targetFiles); err != nil {
		return err
	}

	if mode == "mixed" {
		r.index.Entries = make(map[string]string)
		for path, blobHash := range targetFiles {
			r.index.Add(path, blobHash)
		}
		r.index.Save()
		return nil
	}

	// 3. Hard: Update index and overwrite working directory
	if mode == "hard" {
		currentTracked := map[string]string{}
		if headCommit, err := r.resolveHeadCommitHash(); err == nil && headCommit != "" {
			if headTreeHash, err := r.commitTreeHash(headCommit); err == nil && headTreeHash != "" {
				_ = r.collectTreeFiles(headTreeHash, "", currentTracked)
			}
		}

		for path := range currentTracked {
			if strings.HasPrefix(path, ".loki/") || path == ".loki" {
				continue
			}
			_ = os.Remove(filepath.FromSlash(path))
		}

		r.index.Entries = make(map[string]string)
		for path, blobHash := range targetFiles {
			blobData, err := r.store.ReadObject(blobHash)
			if err != nil {
				return fmt.Errorf("failed to read blob %s: %v", blobHash, err)
			}
			content := objectBody(blobData)

			osPath := filepath.FromSlash(path)
			if err := os.MkdirAll(filepath.Dir(osPath), 0755); err != nil {
				return fmt.Errorf("failed to create directory for %s: %v", path, err)
			}
			if err := os.WriteFile(osPath, content, 0644); err != nil {
				return fmt.Errorf("failed to write file %s: %v", path, err)
			}
			r.index.Add(path, blobHash)
		}
		r.index.Save()
	}

	return nil
}
