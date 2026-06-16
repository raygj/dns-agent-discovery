package registration

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/server/v3/embed"
)

func startTestEtcd(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	clientLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen client: %v", err)
	}
	clientPort := clientLn.Addr().(*net.TCPAddr).Port
	_ = clientLn.Close()

	peerLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen peer: %v", err)
	}
	peerPort := peerLn.Addr().(*net.TCPAddr).Port
	_ = peerLn.Close()

	clientURL := fmt.Sprintf("http://127.0.0.1:%d", clientPort)
	peerURL := fmt.Sprintf("http://127.0.0.1:%d", peerPort)

	clientURLs, err := types.NewURLs([]string{clientURL})
	if err != nil {
		t.Fatalf("client urls: %v", err)
	}
	peerURLs, err := types.NewURLs([]string{peerURL})
	if err != nil {
		t.Fatalf("peer urls: %v", err)
	}

	cfg := embed.NewConfig()
	cfg.Name = "test"
	cfg.Dir = dir
	cfg.LogLevel = "error"
	cfg.UnsafeNoFsync = true
	cfg.ListenClientUrls = clientURLs
	cfg.AdvertiseClientUrls = clientURLs
	cfg.ListenPeerUrls = peerURLs
	cfg.AdvertisePeerUrls = peerURLs
	cfg.InitialCluster = fmt.Sprintf("test=%s", peerURL)

	e, err := embed.StartEtcd(cfg)
	if err != nil {
		t.Fatalf("start etcd: %v", err)
	}
	t.Cleanup(func() { e.Close() })

	select {
	case <-e.Server.ReadyNotify():
	case <-time.After(5 * time.Second):
		t.Fatal("etcd not ready")
	}
	return clientURL
}

func testStore(t *testing.T) *Store {
	t.Helper()
	endpoint := startTestEtcd(t)
	store, err := NewStore(Config{
		Endpoints:     []string{endpoint},
		ClusterDomain: "cluster.local",
		TTL:           1,
		DialTimeout:   2 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestNewStoreDefaults(t *testing.T) {
	endpoint := startTestEtcd(t)
	store, err := NewStore(Config{Endpoints: []string{endpoint}})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer func() { _ = store.Close() }()

	if store.cfg.PathPrefix != defaultPathPrefix {
		t.Fatalf("path prefix: got %q", store.cfg.PathPrefix)
	}
	if store.cfg.ClusterDomain != "cluster.local" {
		t.Fatalf("cluster domain: got %q", store.cfg.ClusterDomain)
	}
	if store.cfg.TTL != 1 {
		t.Fatalf("ttl: got %d", store.cfg.TTL)
	}
}

func TestNewStoreInvalidEndpoint(t *testing.T) {
	store, err := NewStore(Config{
		Endpoints:   []string{"http://127.0.0.1:1"},
		DialTimeout: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer func() { _ = store.Close() }()

	if err := store.Ping(context.Background()); err == nil {
		t.Fatal("expected ping failure")
	}
}

func TestStoreRegisterListDeregister(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	if err := store.Register(ctx, RegisterOptions{
		Capability: "db-reader",
		URL:        "https://mcp.example.com/v1/agents/db-reader",
		Caps:       []string{"sql", "crypto"},
		Proto:      "mcp",
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	entries, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries: got %d", len(entries))
	}
	if entries[0].URL != "https://mcp.example.com/v1/agents/db-reader" {
		t.Fatalf("url: got %q", entries[0].URL)
	}

	if err := store.Deregister(ctx, "db-reader"); err != nil {
		t.Fatalf("Deregister: %v", err)
	}
	entries, err = store.List(ctx)
	if err != nil {
		t.Fatalf("List after deregister: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestStoreRegisterValidation(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	if err := store.Register(ctx, RegisterOptions{URL: "https://example.com"}); err == nil {
		t.Fatal("expected empty capability error")
	}
	if err := store.Register(ctx, RegisterOptions{Capability: "x"}); err == nil {
		t.Fatal("expected empty url error")
	}
	if err := store.Register(ctx, RegisterOptions{
		Capability: "x",
		URL:        "://bad",
	}); err == nil {
		t.Fatal("expected url parse error")
	}
}

func TestStoreRegisterCustomSRV(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	if err := store.Register(ctx, RegisterOptions{
		Capability: "custom",
		URL:        "https://ignored.example.com/path",
		SRVHost:    "target.example.com",
		SRVPort:    8443,
		Proto:      "mcp",
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	entries, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 || entries[0].SRVHost != "target.example.com" || entries[0].SRVPort != 8443 {
		t.Fatalf("srv: %+v", entries[0])
	}
}

func TestStorePing(t *testing.T) {
	store := testStore(t)
	if err := store.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestStoreDeregisterValidation(t *testing.T) {
	store := testStore(t)
	if err := store.Deregister(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty capability")
	}
}
