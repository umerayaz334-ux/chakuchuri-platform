package idgen

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// New returns a sortable identifier with enough entropy for concurrent requests.
func New(prefix string) string {
	var entropy [6]byte
	if _, err := rand.Read(entropy[:]); err == nil {
		return fmt.Sprintf("%s_%d_%s", prefix, time.Now().UTC().UnixNano(), hex.EncodeToString(entropy[:]))
	}
	return fmt.Sprintf("%s_%d", prefix, time.Now().UTC().UnixNano())
}
