package wailsapp

import (
	"errors"
	"io"
)

const maxFetchResponseBytes = 8 << 20

var errFetchResponseTooLarge = errors.New("fetch response exceeds 8 MiB")

func readFetchResponse(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxFetchResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxFetchResponseBytes {
		return nil, errFetchResponseTooLarge
	}
	return data, nil
}
