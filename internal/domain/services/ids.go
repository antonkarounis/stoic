package services

import (
	"crypto/rand"
	"encoding/hex"
	"io"
)

func newID() string {
	b := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		panic("crypto/rand is unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)
}

func newToken() string {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		panic("crypto/rand is unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)
}
