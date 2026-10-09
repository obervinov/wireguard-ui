// Package sharedips keeps the shared client AllowedIPs list in sync with
// the public addresses of a DigitalOcean account.
package sharedips

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/labstack/gommon/log"
	"github.com/ngoduykhanh/wireguard-ui/store"
)

const doAPIURL = "https://api.digitalocean.com/v2"

// Syncer pulls droplet public IPv4 addresses and reserved IPs from the
// DigitalOcean API and stores them as /32 entries of the shared list.
type Syncer struct {
	db       store.IStore
	token    string
	interval time.Duration
	exclude  []string
	client   *http.Client
	baseURL  string
	mu       sync.Mutex
}

// NewSyncer returns nil when no token is configured, i.e. sync is disabled.
func NewSyncer(db store.IStore, token string, interval time.Duration, exclude []string) *Syncer {
	if token == "" {
		return nil
	}
	return &Syncer{
		db:       db,
		token:    token,
		interval: interval,
		exclude:  exclude,
		client:   &http.Client{Timeout: 30 * time.Second},
		baseURL:  doAPIURL,
	}
}

// Enabled reports whether a DigitalOcean token is configured.
func (s *Syncer) Enabled() bool {
	return s != nil
}

// Run syncs immediately and then on every interval until ctx is cancelled.
func (s *Syncer) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		if err := s.Sync(ctx); err != nil {
			log.Warnf("DigitalOcean shared AllowedIPs sync failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Sync fetches the current address list and saves it. On failure the previous
// synced list is kept and only the error is recorded.
func (s *Syncer) Sync(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ips, fetchErr := s.fetch(ctx)

	shared, err := s.db.GetSharedAllowedIPs()
	if err != nil {
		return fmt.Errorf("cannot read shared AllowedIPs: %w", err)
	}
	if fetchErr != nil {
		shared.SyncError = fetchErr.Error()
		_ = s.db.SaveSharedAllowedIPs(shared)
		return fetchErr
	}

	excluded := s.excludedIPs()
	synced := []string{}
	for _, ip := range ips {
		if excluded[ip] {
			continue
		}
		synced = append(synced, ip+"/32")
	}
	sort.Strings(synced)

	if strings.Join(synced, ",") != strings.Join(shared.SyncedIPs, ",") {
		log.Infof("Shared AllowedIPs synced from DigitalOcean: %v", synced)
		shared.UpdatedAt = time.Now().UTC()
	}
	shared.SyncedIPs = synced
	shared.SyncedAt = time.Now().UTC()
	shared.SyncError = ""
	return s.db.SaveSharedAllowedIPs(shared)
}

// excludedIPs returns the endpoint addresses plus configured exclusions.
// The endpoint must never be routed into the tunnel it is the endpoint of.
func (s *Syncer) excludedIPs() map[string]bool {
	excluded := map[string]bool{}
	for _, ip := range s.exclude {
		excluded[strings.TrimSuffix(strings.TrimSpace(ip), "/32")] = true
	}
	settings, err := s.db.GetGlobalSettings()
	if err != nil || settings.EndpointAddress == "" {
		return excluded
	}
	host := settings.EndpointAddress
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if ip := net.ParseIP(host); ip != nil {
		excluded[ip.String()] = true
		return excluded
	}
	addrs, err := net.LookupHost(host)
	if err != nil {
		log.Warnf("Cannot resolve endpoint %s to exclude it from shared AllowedIPs: %v", host, err)
		return excluded
	}
	for _, a := range addrs {
		excluded[a] = true
	}
	return excluded
}

func (s *Syncer) fetch(ctx context.Context) ([]string, error) {
	seen := map[string]bool{}
	result := []string{}
	add := func(ip string) {
		if ip != "" && !seen[ip] {
			seen[ip] = true
			result = append(result, ip)
		}
	}

	var droplets struct {
		Droplets []struct {
			Networks struct {
				V4 []struct {
					IPAddress string `json:"ip_address"`
					Type      string `json:"type"`
				} `json:"v4"`
			} `json:"networks"`
		} `json:"droplets"`
		Links links `json:"links"`
	}
	next := s.baseURL + "/droplets?per_page=200"
	for next != "" {
		droplets.Links = links{}
		if err := s.get(ctx, next, &droplets); err != nil {
			return nil, fmt.Errorf("list droplets: %w", err)
		}
		for _, d := range droplets.Droplets {
			for _, n := range d.Networks.V4 {
				if n.Type == "public" {
					add(n.IPAddress)
				}
			}
		}
		droplets.Droplets = nil
		next = droplets.Links.Pages.Next
	}

	var reserved struct {
		ReservedIPs []struct {
			IP string `json:"ip"`
		} `json:"reserved_ips"`
		Links links `json:"links"`
	}
	next = s.baseURL + "/reserved_ips?per_page=200"
	for next != "" {
		reserved.Links = links{}
		if err := s.get(ctx, next, &reserved); err != nil {
			return nil, fmt.Errorf("list reserved IPs: %w", err)
		}
		for _, r := range reserved.ReservedIPs {
			add(r.IP)
		}
		reserved.ReservedIPs = nil
		next = reserved.Links.Pages.Next
	}

	return result, nil
}

type links struct {
	Pages struct {
		Next string `json:"next"`
	} `json:"pages"`
}

func (s *Syncer) get(ctx context.Context, url string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("DigitalOcean API returned %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
