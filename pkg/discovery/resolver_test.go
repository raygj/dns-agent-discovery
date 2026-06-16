package discovery

import (
	"testing"
	"time"
)

func TestNewResolverDefaults(t *testing.T) {
	r := NewResolver("", 0)
	if r.Server != "127.0.0.1:53" {
		t.Fatalf("server: got %q", r.Server)
	}
	if r.Timeout != 2*time.Second {
		t.Fatalf("timeout: got %v", r.Timeout)
	}
}

func TestResolverLookupTXTAndSRV(t *testing.T) {
	fx := startDNSFixture(t, agentDNSHandler())
	r := NewResolver(fx.addr, time.Second)

	txts, err := r.LookupTXT("db-reader.agents.cluster.local")
	if err != nil {
		t.Fatalf("LookupTXT: %v", err)
	}
	if len(txts) != 3 {
		t.Fatalf("txt count: got %d", len(txts))
	}

	host, port, pri, weight, err := r.LookupSRV("db-reader.agents.cluster.local")
	if err != nil {
		t.Fatalf("LookupSRV: %v", err)
	}
	if host != "mcp.example.com" || port != 443 || pri != 0 || weight != 5 {
		t.Fatalf("srv: host=%q port=%d pri=%d weight=%d", host, port, pri, weight)
	}
}

func TestResolverLookupErrors(t *testing.T) {
	fx := startDNSFixture(t, agentDNSHandler())
	r := NewResolver(fx.addr, time.Second)

	if _, err := r.LookupTXT("missing.agents.cluster.local"); err == nil {
		t.Fatal("expected NXDOMAIN for missing name")
	}

	if _, err := r.LookupTXT("empty-answer.agents.cluster.local"); err == nil {
		t.Fatal("expected error for empty txt answer")
	}

	if _, _, _, _, err := r.LookupSRV("no-srv.agents.cluster.local"); err == nil {
		t.Fatal("expected error for missing srv")
	}

	if _, err := r.LookupTXT("db-reader.agents.cluster.local"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolverPing(t *testing.T) {
	fx := startDNSFixture(t, agentDNSHandler())
	r := NewResolver(fx.addr, time.Second)
	if err := r.Ping(); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestResolverUnreachableServer(t *testing.T) {
	r := NewResolver("127.0.0.1:1", 100*time.Millisecond)
	if _, err := r.LookupTXT("db-reader.agents.cluster.local"); err == nil {
		t.Fatal("expected exchange error")
	}
}
