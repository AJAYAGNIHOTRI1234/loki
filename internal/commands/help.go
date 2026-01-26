package commands

import "fmt"

func Help() {
	fmt.Println(`Loki - a tiny VCS

Commands:
  init
  add <files>
  commit -m "message"
  status
  log`)
}
