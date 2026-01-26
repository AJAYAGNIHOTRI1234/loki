package commands

import (
	"fmt"
	"loki/internal/core"
)

func Status() {
	repo := core.OpenRepository()
	files := repo.Status()

	fmt.Println("Staged files:")
	if( len(files) == 0 ) {
		fmt.Println("nothing to commit")
		return;
	}
	for _, f := range files {
		fmt.Println(" ", f)
		}
}