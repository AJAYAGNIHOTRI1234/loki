package commands

import (
	"fmt"

	"loki/internal/core"
)

func Add(files []string) {
	repo := core.OpenRepository()
	for _, f := range files {
		repo.AddFile(f)
	}
	fmt.Println("Files added to staging area")
}
