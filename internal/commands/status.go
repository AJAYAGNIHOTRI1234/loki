package commands

import (
	"fmt"
	"loki/internal/core"
)

func Status() {
	repo := core.OpenRepository()
	files := repo.Status()

	fmt.Println("Files staged for commit :" )

	for _, f := range files {
		fmt.Println(" ", f)
	}
}
