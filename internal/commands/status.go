package commands

import (
	"fmt"
	"loki/internal/core"
)

func Status() {
	repo := core.OpenRepository()
	files := repo.Status()
 master
fmt.Println("Changes to be committed:")

if len(files) == 0 {
    fmt.Println("  (no files staged)")
    return
}

for _, f := range files {
    fmt.Println("  ", f)
}
 master
	for _, f := range files {
		fmt.Println(" ", f)
		}
}
