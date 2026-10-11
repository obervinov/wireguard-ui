package router

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ngoduykhanh/wireguard-ui/model"
)

func TestGlobalSettingsRendersSharedAllowedIPs(t *testing.T) {
	app := New(os.DirFS("../templates"), map[string]interface{}{"basePath": ""}, [64]byte{})

	for _, syncEnabled := range []bool{true, false} {
		var buf bytes.Buffer
		err := app.Renderer.Render(&buf, "global_settings.html", map[string]interface{}{
			"baseData":       model.BaseData{Active: "global-settings", CurrentUser: "admin", Admin: true},
			"globalSettings": model.GlobalSetting{EndpointAddress: "wg.example.com", DNSServers: []string{"1.1.1.1"}},
			"sharedAllowedIPs": model.SharedAllowedIPs{
				StaticIPs: []string{"192.168.88.0/24"},
				SyncedIPs: []string{"203.0.113.10/32"},
				SyncedAt:  time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC),
			},
			"sharedSyncEnabled": syncEnabled,
		}, nil)
		if err != nil {
			t.Fatalf("sync=%v: render failed: %v", syncEnabled, err)
		}
		out := buf.String()
		for _, want := range []string{`id="card_shared_allowed_ips"`, `addTag('192.168.88.0/24')`, `id="use_shared_allowed_ips" checked`} {
			if !strings.Contains(out, want) {
				t.Errorf("sync=%v: output misses %s", syncEnabled, want)
			}
		}
		if syncEnabled != strings.Contains(out, "203.0.113.10/32") {
			t.Errorf("sync=%v: synced list visibility is wrong", syncEnabled)
		}
	}
}

func TestClientsPageRenders(t *testing.T) {
	app := New(os.DirFS("../templates"), map[string]interface{}{"basePath": ""}, [64]byte{})
	for _, share := range []bool{true, false} {
		var buf bytes.Buffer
		err := app.Renderer.Render(&buf, "clients.html", map[string]interface{}{
			"baseData":       model.BaseData{Active: "", CurrentUser: "admin", Admin: true},
			"clientDataList": []model.ClientData{},
			"shareEnabled":   share,
		}, nil)
		if err != nil {
			t.Fatalf("render failed: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, `id="_use_shared_allowed_ips"`) {
			t.Error("edit modal misses the use-shared checkbox")
		}
		if !strings.Contains(out, `id="modal_share_client"`) {
			t.Error("share modal is missing")
		}
		if want := fmt.Sprintf("shareEnabled = %t;", share); !strings.Contains(out, want) {
			t.Errorf("share=%t: output misses %q", share, want)
		}
	}
}

func TestTwoFactorPagesRender(t *testing.T) {
	app := New(os.DirFS("../templates"), map[string]interface{}{"basePath": ""}, [64]byte{})
	for page, wants := range map[string][]string{
		"profile.html":        {`id="card_2fa"`, `id="btn_totp_setup"`, `id="btn_totp_enable"`, `id="btn_totp_disable"`, `/api/2fa/`},
		"login.html":          {`id="frm_totp"`, `id="totp_code"`, `/login/2fa`, `totp_required`},
		"users_settings.html": {`.btn-reset-2fa`, `/api/2fa/reset`},
	} {
		var buf bytes.Buffer
		err := app.Renderer.Render(&buf, page, map[string]interface{}{
			"baseData": model.BaseData{Active: "profile", CurrentUser: "admin", Admin: true},
		}, nil)
		if err != nil {
			t.Fatalf("%s: render failed: %v", page, err)
		}
		for _, want := range wants {
			if !strings.Contains(buf.String(), want) {
				t.Errorf("%s: output misses %s", page, want)
			}
		}
	}
}

func TestStatusPageRenders(t *testing.T) {
	type peer struct {
		Name, Email, PublicKey, Received, Transmitted, LastHandshake, Endpoint string
		ReceivedBytes, TransmitBytes                                            int64
		LastHandshakeTime                                                       time.Time
		Connected                                                               bool
		AllocatedIPs                                                            []string
	}
	type device struct {
		Name, Received, Transmitted string
		Online                      int
		Peers                       []peer
	}
	devices := []device{{Name: "wg0", Online: 1, Received: "1.5 GiB", Transmitted: "200.0 MiB", Peers: []peer{
		{Name: "laptop", Email: "alice@example.com", PublicKey: "J7Mgewxzu0maKFHQnxuNMEr14LPmzoOuXHrX+9hGq0I=", Received: "1.5 GiB", LastHandshake: "42s ago",
			LastHandshakeTime: time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC), Connected: true, AllocatedIPs: []string{"10.252.1.2/32", "10.252.2.2/32"}, Endpoint: "198.51.100.7:51820"},
		{PublicKey: "AJA4kQSME4SgBievqLYrftps+EQDzCmchsQSaPxw9H4=", LastHandshake: "never", AllocatedIPs: []string{"10.252.1.9/32"}},
	}}}
	app := New(os.DirFS("../templates"), map[string]interface{}{"basePath": ""}, [64]byte{})
	for _, admin := range []bool{true, false} {
		var buf bytes.Buffer
		err := app.Renderer.Render(&buf, "status.html", map[string]interface{}{
			"baseData": model.BaseData{Active: "status", CurrentUser: "admin", Admin: admin},
			"devices":  devices,
		}, nil)
		if err != nil {
			t.Fatalf("admin=%v: render failed: %v", admin, err)
		}
		out := buf.String()
		for _, want := range []string{"1 of 2 online", ">10.252.2.2/32<", "AJA4kQSM…", "42s ago", `title="2026-10-11 12:00:00 UTC"`} {
			if !strings.Contains(out, want) {
				t.Errorf("admin=%v: output misses %s", admin, want)
			}
		}
		if admin != strings.Contains(out, "198.51.100.7:51820") {
			t.Errorf("admin=%v: endpoint visibility is wrong", admin)
		}
	}
}
