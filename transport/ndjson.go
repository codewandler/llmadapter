package transport

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
)

const defaultNDJSONMaxLineBytes = 1024 * 1024

type NDJSONReader struct {
	scanner *bufio.Scanner
}

func NewNDJSONReader(r io.Reader) *NDJSONReader {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), defaultNDJSONMaxLineBytes)
	return &NDJSONReader{scanner: scanner}
}

func (r *NDJSONReader) Next() ([]byte, error) {
	for r.scanner.Scan() {
		line := bytes.TrimSpace(r.scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		out := append([]byte(nil), line...)
		return out, nil
	}
	if err := r.scanner.Err(); err != nil {
		return nil, fmt.Errorf("read ndjson: %w", err)
	}
	return nil, io.EOF
}
