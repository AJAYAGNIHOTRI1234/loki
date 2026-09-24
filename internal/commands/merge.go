package commands

import (
	"fmt"
	"os"

	"loki/internal/config"
	"loki/internal/core"
	"loki/internal/utils"
)

func Merge(args []string) {
	if len(args) != 1 {
		fmt.Println("Usage: loki merge <branch-name>")
		return
	}

	cwd, _ := os.Getwd()
	repoRoot := config.FindRepoRoot(cwd)
	cfg := config.NewConfig()
	cfg.Load(repoRoot)
	author := cfg.Get("user.name")
	email := cfg.Get("user.email")
	if author == "" {
		author = "loki"
	}
	if email == "" {
		email = "loki@local"
	}

	repo := core.OpenRepository()
	message, err := repo.Merge(args[0], author, email)
	if err != nil {
		fmt.Println(utils.ColorText("error: "+err.Error(), "error"))
		return
	}

	if message != "" {
		fmt.Println(utils.ColorText(message, "success"))
	}
}
