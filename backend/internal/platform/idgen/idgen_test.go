package idgen

import (
	"strings"
	"testing"
)

func TestNewProducesUniquePrefixedIDs(t *testing.T) {
	seen := map[string]bool{}
	for index := 0; index < 100; index++ {
		id := New("usr")
		if !strings.HasPrefix(id, "usr_") {
			t.Fatalf("identifier has wrong prefix: %s", id)
		}
		if seen[id] {
			t.Fatalf("duplicate identifier generated: %s", id)
		}
		seen[id] = true
	}
}
