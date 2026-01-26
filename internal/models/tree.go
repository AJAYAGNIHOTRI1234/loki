package models

import (
	"bytes"
	"fmt"
)

type TreeEntry struct {
	Mode string
	Name string
	Hash []byte
}

type Tree struct {
	Entries []TreeEntry
}

func (t *Tree) Serialize() []byte {
	var buf bytes.Buffer
	for _, e := range t.Entries {
		fmt.Fprintf(&buf, "%s %s\x00", e.Mode, e.Name)
		buf.Write(e.Hash)
	}
	header := fmt.Sprintf("tree %d\x00", buf.Len())
	return append([]byte(header), buf.Bytes()...)
}
