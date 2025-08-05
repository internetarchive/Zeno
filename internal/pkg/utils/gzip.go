package utils

import (
	"bytes"
	"compress/gzip"
	"io"
)

func DecompressGzipped(r io.Reader) ([]byte, error) {
	reader, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, reader); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func DecompressGzippedBytes(data []byte) ([]byte, error) {
	return DecompressGzipped(bytes.NewReader(data))
}

func MustDecompressGzippedBytes(data []byte) []byte {
	result, err := DecompressGzippedBytes(data)
	if err != nil {
		panic(err)
	}
	return result
}
