package commands

import (
	"fmt"

	"loki/internal/core"
)

func Commit(args []string) {
	msg := "default commit"
	if len(args) >= 2 && args[0] == "-m" {
		msg = args[1]
	}

	repo := core.OpenRepository()
	hash := repo.Commit(msg)
	fmt.Println("Committed:", hash)
}
