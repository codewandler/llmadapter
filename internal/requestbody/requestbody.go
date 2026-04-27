package requestbody

import (
	"errors"
	"fmt"
	"io"
)

const MaxBytes int64 = 10 << 20

var ErrTooLarge = errors.New("request body too large")

func Read(r io.Reader) ([]byte, error) {
	limited := io.LimitReader(r, MaxBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > MaxBytes {
		return nil, fmt.Errorf("%w: max %d bytes", ErrTooLarge, MaxBytes)
	}
	return body, nil
}
