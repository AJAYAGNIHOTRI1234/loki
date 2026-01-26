package core

import (
	"fmt"
	"loki/internal/models"
	"loki/internal/storage"
	"os"
)

type Repository struct {
	store *storage.FileStorage
	index *Index
}

func OpenRepository() *Repository {
	return &Repository{
		store: storage.NewFileStorage(".loki"),
		index: LoadIndex(),
	}
}

func (r *Repository) AddFile(path string) {
	r.index.Add(path)
	r.index.Save()
}

func (r *Repository) Commit(message string) string {
	treeHash := r.index.WriteTree(r.store)
	commit := &models.Commit{
		Tree:    treeHash,
		Message: message,
	}
	commitHash := r.store.WriteObject(commit.Serialize())
	// Optionally update HEAD and log (not implemented here)
	r.index.Clear()
	return commitHash
}

func (r *Repository) Status() []string {
	return r.index.Files()
}

func (r *Repository) PrintLog() {
	logs, _ := os.ReadFile(".loki/commits.log")
	fmt.Println(string(logs))
}
