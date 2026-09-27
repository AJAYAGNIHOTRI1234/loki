package commands

import "fmt"

func Help() {
	fmt.Println(`loki - a simple Git-like version control tool

Usage:
  loki <command> [args]

Core commands:
  init                Create a new repository in the current directory
  add <files...>      Stage one or more files
  commit -m <msg>     Create a new commit from staged changes
  status              Show tracked, staged, modified, deleted, and untracked files
  diff [--staged]     Show differences between working tree and index
                      or between index and HEAD when --staged is used
  log                 Show the commit history
  checkout <target>   Switch to a branch, tag, or commit
  branch <name>       Create or list branches
  merge <branch>      Merge a branch into the current branch
  reset [target]      Reset HEAD / index / working tree
  rm <files...>       Remove files from the index and working tree
  revert <commit>     Create a new commit that undoes an earlier commit
  tag [options]       Create, list, delete, or annotate tags
  config              Read or write configuration values
  help                Show this help text`)
}
