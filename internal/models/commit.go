package models

import (
	"fmt"
	"time"
)

type Commit struct {
	Tree    string
	Message string
}

func (c *Commit) Serialize() []byte {
	body := fmt.Sprintf(
		"tree %s\n"+
			"author loki <loki@local> %d\n\n"+
			"%s\n",
		c.Tree,
		time.Now().Unix(),
		c.Message,
	)

	header := fmt.Sprintf("commit %d\x00", len(body))
	return append([]byte(header), []byte(body)...)
}
