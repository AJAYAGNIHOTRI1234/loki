package storage

import (
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
)

func (fs *FileStorage) WriteObject(data []byte) string {
	hash := sha1.Sum(data)
	hashStr := hex.EncodeToString(hash[:])

	dir := filepath.Join(fs.root, "objects", hashStr[:2])
	file := filepath.Join(dir, hashStr[2:])

	_ = os.MkdirAll(dir, 0755)

	f, _ := os.Create(file)
	defer f.Close()

	w := zlib.NewWriter(f)
	defer w.Close()
	w.Write(data)

	return hashStr
}
