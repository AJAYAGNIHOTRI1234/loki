package commands

import "loki/internal/core"

func Log() {
	repo := core.OpenRepository()
	repo.PrintLog()
}
