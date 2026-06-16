package main

import (
	"context"
	"testing"
	"time"
)

func TestEnvOr(t *testing.T) {
	const key = "DAD_TEST_ENV_OR"
	t.Setenv(key, "from-env")
	if got := envOr(key, "fallback"); got != "from-env" {
		t.Fatalf("got %q", got)
	}
	t.Setenv(key, "")
	if got := envOr(key, "fallback"); got != "fallback" {
		t.Fatalf("got %q", got)
	}
}

func TestNewStoreEndpointParsing(t *testing.T) {
	orig := *etcdEndpoints
	origTimeout := *timeout
	t.Cleanup(func() {
		*etcdEndpoints = orig
		*timeout = origTimeout
	})

	*etcdEndpoints = " http://127.0.0.1:1 , http://127.0.0.1:2 "
	*timeout = 200 * time.Millisecond
	store, err := newStore()
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}
	defer func() { _ = store.Close() }()

	if err := store.Ping(context.Background()); err == nil {
		t.Fatal("expected ping failure for invalid etcd endpoints")
	}
}

func TestUsageDoesNotPanic(t *testing.T) {
	usage()
}
