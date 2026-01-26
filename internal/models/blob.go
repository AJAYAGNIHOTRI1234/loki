package models

import (
	"fmt"
)

type Blob struct {
	Content []byte
}

func (b *Blob) Serialize() []byte {
	header := fmt.Sprintf("blob %d\x00", len(b.Content))
	return append([]byte(header), b.Content...)
}
