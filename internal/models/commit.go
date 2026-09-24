package models

import (
	"fmt"
	"time"
)

type Commit struct {
	Tree    string
	Parents []string
	Message string
	Author  string
	Email   string
}

func (c *Commit) Serialize() []byte {
	body := fmt.Sprintf(
		"tree %s\n"+
			"%s"+
			"author %s <%s> %d\n\n"+
			"%s\n",
		c.Tree,
		formatParents(c.Parents),
		c.Author,
		c.Email,
		time.Now().Unix(),
		c.Message,
	)

	header := fmt.Sprintf("commit %d\x00", len(body))
	return append([]byte(header), []byte(body)...)
}

func formatParents(parents []string) string {
	if len(parents) == 0 {
		return ""
	}

	body := ""
	for _, parent := range parents {
		if parent == "" {
			continue
		}
		body += fmt.Sprintf("parent %s\n", parent)
	}
	return body
}
