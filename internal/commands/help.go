package commands

import "fmt"

func Help() {
	fmt.Println(`These are common loki commands used in various situations:
  ------------------
  init              Create an empty loki repository or reinitialize an existing one
  add               Add file contents to the index
                      - <files>   Files to add to the staging area
  commit            Record changes to the repository
                      - -m <msg>    Commit message
 	merge             Merge a branch into the current branch
  status            Show the working tree status
  log               Show commit logs
  tag               Create, list, or delete tags`)
}
