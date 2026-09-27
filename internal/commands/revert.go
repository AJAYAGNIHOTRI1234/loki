package commands

import (
	"fmt"
	"os"

	"loki/internal/config"
	"loki/internal/core"
	"loki/internal/utils"
)

func Revert(args []string) {
	if len(args) != 1 {
		fmt.Println("Usage: loki revert <commit>")
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

	lokiDir := findLokiDir()
	commitHash, err := resolveCommit(lokiDir, args[0])
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	if commitHash == "" {
		fmt.Println("No commits yet")
		return
	}

	repo := core.OpenRepository()
	newHash, err := repo.Revert(commitHash, author, email)
	if err != nil {
		fmt.Println(utils.ColorText("error: "+err.Error(), "error"))
		return
	}

	fmt.Printf(utils.ColorText("Reverted commit: %s\n", "success"), newHash)
}
