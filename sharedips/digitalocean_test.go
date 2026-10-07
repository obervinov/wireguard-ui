package sharedips

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/ngoduykhanh/wireguard-ui/model"
	"github.com/ngoduykhanh/wireguard-ui/store/jsondb"
)

func TestSyncCollectsDropletsAndReservedIPs(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.URL.Path == "/droplets" && r.URL.Query().Get("page") == "":
			w.Write([]byte(`{"droplets":[{"networks":{"v4":[{"ip_address":"10.0.0.2","type":"private"},{"ip_address":"203.0.113.10","type":"public"}]}}],
				"links":{"pages":{"next":"` + srv.URL + `/droplets?page=2"}}}`))
		case r.URL.Path == "/droplets":
			w.Write([]byte(`{"droplets":[{"networks":{"v4":[{"ip_address":"203.0.113.20","type":"public"},{"ip_address":"203.0.113.1","type":"public"}]}}],"links":{}}`))
		case r.URL.Path == "/reserved_ips":
			w.Write([]byte(`{"reserved_ips":[{"ip":"198.51.100.5"},{"ip":"203.0.113.10"}],"links":{}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	db, err := jsondb.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("WGUI_ENDPOINT_ADDRESS", "203.0.113.1:51820")
	if err := db.Init(); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveSharedAllowedIPs(model.SharedAllowedIPs{StaticIPs: []string{"192.168.88.0/24"}}); err != nil {
		t.Fatal(err)
	}

	s := NewSyncer(db, "token", 0, []string{"198.51.100.5/32"})
	s.baseURL = srv.URL
	if err := s.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}

	shared, _ := db.GetSharedAllowedIPs()
	// endpoint 203.0.113.1 and the configured exclusion are left out, private IPs are ignored
	want := []string{"203.0.113.10/32", "203.0.113.20/32"}
	if !reflect.DeepEqual(shared.SyncedIPs, want) {
		t.Fatalf("synced = %v, want %v", shared.SyncedIPs, want)
	}
	settings, _ := db.GetGlobalSettings()
	wantAll := []string{"192.168.88.0/24", "203.0.113.10/32", "203.0.113.20/32"}
	if !reflect.DeepEqual(settings.SharedAllowedIPs, wantAll) {
		t.Fatalf("global settings shared = %v, want %v", settings.SharedAllowedIPs, wantAll)
	}

	// a failing API keeps the previous list and records the error
	s.token = "bad"
	if err := s.Sync(context.Background()); err == nil {
		t.Fatal("expected error")
	}
	shared, _ = db.GetSharedAllowedIPs()
	if !reflect.DeepEqual(shared.SyncedIPs, want) || shared.SyncError == "" {
		t.Fatalf("after failure: synced = %v, error = %q", shared.SyncedIPs, shared.SyncError)
	}
}
