package model

import (
	"time"
)

// SharedAllowedIPs is the AllowedIPs list rendered into every client config
// unless the client opts out with ExcludeSharedAllowedIPs.
// It lives in its own file (server/shared_allowed_ips.json) so that a sync does
// not change the server hash: the list is client-side only and never needs an
// "Apply Config" on the server.
type SharedAllowedIPs struct {
	StaticIPs []string  `json:"static_ips"`
	SyncedIPs []string  `json:"synced_ips"`
	SyncedAt  time.Time `json:"synced_at"`
	SyncError string    `json:"sync_error"`
	UpdatedAt time.Time `json:"updated_at"`
}

// All returns static and synced entries, de-duplicated, static first.
func (s SharedAllowedIPs) All() []string {
	return MergeIPLists(s.StaticIPs, s.SyncedIPs)
}

// MergeIPLists concatenates lists dropping empty and duplicate entries, keeping order.
func MergeIPLists(lists ...[]string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, list := range lists {
		for _, ip := range list {
			if ip == "" || seen[ip] {
				continue
			}
			seen[ip] = true
			result = append(result, ip)
		}
	}
	return result
}
