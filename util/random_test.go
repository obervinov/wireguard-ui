package util

import (
	"regexp"
	"testing"
)

func TestRandomString(t *testing.T) {
	valid := regexp.MustCompile(`^[a-zA-Z0-9]{32}$`)
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		s := RandomString(32)
		if !valid.MatchString(s) {
			t.Fatalf("RandomString(32) = %q, want 32 alphanumerics", s)
		}
		if seen[s] {
			t.Fatalf("RandomString(32) repeated %q", s)
		}
		seen[s] = true
	}
}
