package core

import "encoding/hex"

func decodeHash(h string) []byte {
	b, _ := hex.DecodeString(h)
	return b
}
