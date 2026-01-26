package main

import (
	"fmt"
	"os"

	"loki/internal/commands"
)

func main() {
	if len(os.Args) < 2 {
		commands.Help()
		return
	}

	switch os.Args[1] {
	case "init":
		commands.Init()
	case "add":
		commands.Add(os.Args[2:])
	case "commit":
		commands.Commit(os.Args[2:])
	case "status":
		commands.Status()
	case "log":
		commands.Log()
	case "help":
		commands.Help()
	default:
		fmt.Println("Unknown command:", os.Args[1])
		commands.Help()
	}
}
