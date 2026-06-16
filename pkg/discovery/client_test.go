package discovery

import (
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	c := NewClient("cluster.local.", "127.0.0.1:5353", time.Second)
	if c.Zone != "agents.cluster.local" {
		t.Fatalf("zone: got %q", c.Zone)
	}
}

func TestClientLookup(t *testing.T) {
	fx := startDNSFixture(t, agentDNSHandler())
	client := NewClient("cluster.local", fx.addr, time.Second)

	rec, err := client.Lookup("db-reader")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if rec.URL != "https://mcp.example.com/v1/agents/db-reader" {
		t.Fatalf("url: got %q", rec.URL)
	}
	if rec.Proto != "mcp" || len(rec.Caps) != 2 {
		t.Fatalf("proto/caps: proto=%q caps=%v", rec.Proto, rec.Caps)
	}
	if rec.SRVHost != "mcp.example.com" || rec.SRVPort != 443 {
		t.Fatalf("srv: host=%q port=%d", rec.SRVHost, rec.SRVPort)
	}
}

func TestClientLookupValidation(t *testing.T) {
	fx := startDNSFixture(t, agentDNSHandler())
	client := NewClient("cluster.local", fx.addr, time.Second)

	if _, err := client.Lookup(""); err == nil {
		t.Fatal("expected error for empty capability")
	}
	if _, err := client.Lookup("missing"); err == nil {
		t.Fatal("expected error for missing record")
	}
}

func TestClientLookupWithoutSRV(t *testing.T) {
	fx := startDNSFixture(t, agentDNSHandler())
	client := NewClient("cluster.local", fx.addr, time.Second)

	rec, err := client.Lookup("no-srv")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if rec.URL != "https://example.com" {
		t.Fatalf("url: got %q", rec.URL)
	}
	if rec.SRVHost != "" || rec.SRVPort != 0 {
		t.Fatalf("expected empty srv, got host=%q port=%d", rec.SRVHost, rec.SRVPort)
	}
}
