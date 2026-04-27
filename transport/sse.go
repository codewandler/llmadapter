package transport

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type SSEFrame struct {
	Event string
	Data  []byte
	ID    string
	Retry int
}

type SSEReader struct {
	r *bufio.Reader
}

func NewSSEReader(r io.Reader) *SSEReader {
	return &SSEReader{r: bufio.NewReader(r)}
}

func (r *SSEReader) Next() ([]byte, error) {
	var block bytes.Buffer
	var line bytes.Buffer

	for {
		b, err := r.r.ReadByte()
		if err != nil {
			if err == io.EOF && (line.Len() > 0 || block.Len() > 0) {
				if line.Len() > 0 {
					block.Write(line.Bytes())
				}
				return block.Bytes(), nil
			}
			return nil, err
		}

		if b != '\n' && b != '\r' {
			line.WriteByte(b)
			continue
		}
		if b == '\r' {
			if next, err := r.r.Peek(1); err == nil && next[0] == '\n' {
				if _, err := r.r.ReadByte(); err != nil {
					return nil, err
				}
			}
		}

		if line.Len() == 0 {
			if block.Len() == 0 {
				continue
			}
			return block.Bytes(), nil
		}
		if block.Len() > 0 {
			block.WriteByte('\n')
		}
		block.Write(line.Bytes())
		line.Reset()
	}
}

func ParseSSEFrame(raw []byte) (SSEFrame, error) {
	var frame SSEFrame
	var data []string
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		field, value, ok := strings.Cut(line, ":")
		if !ok {
			value = ""
		} else {
			value = strings.TrimPrefix(value, " ")
		}
		switch field {
		case "event":
			frame.Event = value
		case "data":
			data = append(data, value)
		case "id":
			frame.ID = value
		case "retry":
			retry, err := strconv.Atoi(value)
			if err != nil {
				return frame, fmt.Errorf("parse sse retry: %w", err)
			}
			frame.Retry = retry
		}
	}
	frame.Data = []byte(strings.Join(data, "\n"))
	return frame, nil
}
