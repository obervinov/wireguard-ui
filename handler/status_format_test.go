package handler

import (
	"testing"
	"time"
)

func TestHumanBytes(t *testing.T) {
	cases := map[int64]string{0: "0 B", 1023: "1023 B", 1024: "1.0 KiB", 1536: "1.5 KiB", 5 << 20: "5.0 MiB", 3 << 30: "3.0 GiB"}
	for in, want := range cases {
		if got := humanBytes(in); got != want {
			t.Errorf("humanBytes(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestHandshakeAgo(t *testing.T) {
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		t    time.Time
		want string
	}{
		{time.Time{}, "never"},
		{now.Add(-42 * time.Second), "42s ago"},
		{now.Add(-5 * time.Minute), "5 min ago"},
		{now.Add(-3 * time.Hour), "3 h ago"},
		{now.Add(-50 * time.Hour), "2 d ago"},
	}
	for _, c := range cases {
		if got := handshakeAgo(c.t, now); got != c.want {
			t.Errorf("handshakeAgo(%v) = %q, want %q", c.t, got, c.want)
		}
	}
}
