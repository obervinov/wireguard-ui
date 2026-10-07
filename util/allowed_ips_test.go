package util

import (
	"reflect"
	"strings"
	"testing"

	"github.com/ngoduykhanh/wireguard-ui/model"
)

func TestEffectiveAllowedIPs(t *testing.T) {
	setting := model.GlobalSetting{SharedAllowedIPs: []string{"10.0.0.0/24", "203.0.113.10/32"}}
	client := model.Client{AllowedIPs: []string{"10.252.1.0/24", "10.0.0.0/24"}}

	got := EffectiveAllowedIPs(client, setting)
	want := []string{"10.252.1.0/24", "10.0.0.0/24", "203.0.113.10/32"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("included: got %v, want %v", got, want)
	}

	client.ExcludeSharedAllowedIPs = true
	got = EffectiveAllowedIPs(client, setting)
	want = []string{"10.252.1.0/24", "10.0.0.0/24"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("excluded: got %v, want %v", got, want)
	}
}

func TestBuildClientConfigUsesSharedAllowedIPs(t *testing.T) {
	server := model.Server{Interface: &model.ServerInterface{ListenPort: 51820}, KeyPair: &model.ServerKeypair{PublicKey: "pub"}}
	setting := model.GlobalSetting{EndpointAddress: "wg.example.com", SharedAllowedIPs: []string{"203.0.113.10/32"}}
	client := model.Client{AllocatedIPs: []string{"10.252.1.2/32"}}

	cfg := BuildClientConfig(client, server, setting)
	if !strings.Contains(cfg, "AllowedIPs = 203.0.113.10/32\n") {
		t.Fatalf("config missing shared AllowedIPs:\n%s", cfg)
	}
}
