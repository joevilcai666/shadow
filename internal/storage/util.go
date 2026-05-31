package storage

import (
	"crypto/rand"
	"fmt"
)

// NewID generates a random 32-character hex ID.
func NewID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}
