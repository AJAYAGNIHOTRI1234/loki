package commands

import (
	"fmt"
	"os"
)

func Init() {
	dirs := []string{
		".loki",
		".loki/objects",
		".loki/refs",
	}

	for _, d := range dirs {
		_ = os.MkdirAll(d, 0755)
	}

	_ = os.WriteFile(".loki/HEAD", []byte("ref: refs/main"), 0644)
	fmt.Println("Initialized empty Loki repository")
}
