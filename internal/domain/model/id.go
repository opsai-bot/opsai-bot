package model

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// generateID creates a unique ID with timestamp prefix for sortability
func generateID() string {
	ts := time.Now().UTC().UnixMilli()
	random := make([]byte, 8)
	_, _ = rand.Read(random)

	buf := make([]byte, 0, 24)
	buf = append(buf, []byte(hex.EncodeToString(int64ToBytes(ts)))...)
	buf = append(buf, []byte(hex.EncodeToString(random))...)
	return string(buf)
}

func int64ToBytes(i int64) []byte {
	b := make([]byte, 8)
	for idx := 7; idx >= 0; idx-- {
		b[idx] = byte(i & 0xff)
		i >>= 8
	}
	return b
}
