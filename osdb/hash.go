package osdb

import (
	"os"
	"io"
	"fmt"
	"bytes"
	"errors"
	"encoding/binary"
)

const (
	chunkSize = 65536 // 64k
)

func Hash(r io.ReaderAt, size int64) (string, error) {
	var hash uint64

	if size < chunkSize*2 {
		return "", errors.New("File is too small")
	}

	// Read head and tail blocks.
	buf := make([]byte, chunkSize*2)
	if _, err := r.ReadAt(buf[:chunkSize], 0); err != nil {
		return "", err
	}
	if _, err := r.ReadAt(buf[chunkSize:], size-chunkSize); err != nil {
		return "", err
	}

	// Convert to uint64, and sum.
	nums := make([]uint64, (chunkSize*2)/8)
	reader := bytes.NewReader(buf)
	if err := binary.Read(reader, binary.LittleEndian, &nums); err != nil {
		return "", err
	}
	for _, num := range nums {
		hash += num
	}

	return fmt.Sprintf("%016x", hash+uint64(size)), nil
}

func HashFile(file *os.File) (string, error) {
	stats, err := file.Stat()
	if err != nil {
		return "", err
	}

	return Hash(file, stats.Size())
}
