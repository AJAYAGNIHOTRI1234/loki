package core

import (
	"os"

	"encoding/json"
	"loki/internal/models"
)

type Index struct {
	FilesList []string
}

func LoadIndex() *Index {
	data, err := os.ReadFile(".loki/index")
	if err != nil {
		return &Index{}
	}
	var idx Index
	_ = json.Unmarshal(data, &idx)
	return &idx
}

func (i *Index) Add(file string) {
	i.FilesList = append(i.FilesList, file)
}

func (i *Index) Save() {
	data, _ := json.Marshal(i)
	_ = os.WriteFile(".loki/index", data, 0644)
}

func (i *Index) Files() []string {
	return i.FilesList
}

func (i *Index) Clear() {
	i.FilesList = []string{}
	i.Save()
}

type ObjectStore interface {
	WriteObject(data []byte) string
}

func (i *Index) WriteTree(store ObjectStore) string {
	var entries []models.TreeEntry

	for _, file := range i.FilesList {
		data, _ := os.ReadFile(file)
		blob := &models.Blob{Content: data}
		blobHash := store.WriteObject(blob.Serialize())

		entries = append(entries, models.TreeEntry{
			Mode: "100644",
			Name: file,
			Hash: decodeHash(blobHash),
		})
	}

	tree := &models.Tree{Entries: entries}
	return store.WriteObject(tree.Serialize())
}
