package router

import (
	"bytes"
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
	var buf bytes.Buffer
	err := app.Renderer.Render(&buf, "clients.html", map[string]interface{}{
		"baseData":       model.BaseData{Active: "", CurrentUser: "admin", Admin: true},
		"clientDataList": []model.ClientData{},
	}, nil)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	if !strings.Contains(buf.String(), `id="_use_shared_allowed_ips"`) {
		t.Error("edit modal misses the use-shared checkbox")
	}
}
