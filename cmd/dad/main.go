package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/raygj/dns-agent-discovery/pkg/discovery"
	"github.com/raygj/dns-agent-discovery/pkg/registration"
)

var (
	clusterDomain = flag.String("cluster-domain", envOr("DAD_CLUSTER_DOMAIN", "cluster.local"), "cluster domain")
	dnsServer     = flag.String("dns-server", envOr("DAD_DNS_SERVER", "127.0.0.1:53"), "DNS server for lookups")
	etcdEndpoints = flag.String("etcd-endpoints", envOr("DAD_ETCD_ENDPOINTS", "http://127.0.0.1:2379"), "comma-separated etcd endpoints")
	etcdPath      = flag.String("etcd-path", envOr("DAD_ETCD_PATH", "/skydns"), "etcd path prefix for CoreDNS")
	ttl           = flag.Int("ttl", 1, "DNS record TTL in seconds")
	timeout       = flag.Duration("timeout", 5*time.Second, "operation timeout")
)

func main() {
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	switch args[0] {
	case "lookup":
		cmdLookup(ctx, args[1:])
	case "register":
		cmdRegister(ctx, args[1:])
	case "deregister":
		cmdDeregister(ctx, args[1:])
	case "list":
		cmdList(ctx, args[1:])
	case "health":
		cmdHealth(ctx, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", args[0])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `dad — DNS Agent Discovery CLI

Usage:
  dad [flags] <subcommand> [args]

Subcommands:
  lookup <capability>                    Resolve agent record via DNS
  register <capability> <url> [caps...]  Write TXT+SRV records to etcd
  deregister <capability>                Remove records from etcd
  list                                   Dump all registered agents
  health                                 Check CoreDNS + etcd reachability

Flags:
`)
	flag.PrintDefaults()
}

func cmdLookup(_ context.Context, args []string) {
	if len(args) < 1 {
		fatal("usage: dad lookup <capability>")
	}
	client := discovery.NewClient(*clusterDomain, *dnsServer, *timeout)
	rec, err := client.Lookup(args[0])
	if err != nil {
		fatal(err.Error())
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rec); err != nil {
		fatal(err.Error())
	}
}

func cmdRegister(ctx context.Context, args []string) {
	if len(args) < 2 {
		fatal("usage: dad register <capability> <url> [--caps tag1,tag2]")
	}
	caps := []string{}
	for i := 2; i < len(args); i++ {
		if args[i] == "--caps" && i+1 < len(args) {
			caps = strings.Split(args[i+1], ",")
			break
		}
	}

	store, err := newStore()
	if err != nil {
		fatal(err.Error())
	}
	defer store.Close()

	if err := store.Register(ctx, registration.RegisterOptions{
		Capability: args[0],
		URL:        args[1],
		Caps:       caps,
		Proto:      "mcp",
	}); err != nil {
		fatal(err.Error())
	}
	fmt.Printf("registered %s -> %s\n", args[0], args[1])
}

func cmdDeregister(ctx context.Context, args []string) {
	if len(args) < 1 {
		fatal("usage: dad deregister <capability>")
	}
	store, err := newStore()
	if err != nil {
		fatal(err.Error())
	}
	defer store.Close()

	if err := store.Deregister(ctx, args[0]); err != nil {
		fatal(err.Error())
	}
	fmt.Printf("deregistered %s\n", args[0])
}

func cmdList(ctx context.Context, args []string) {
	if len(args) != 0 {
		fatal("usage: dad list")
	}
	store, err := newStore()
	if err != nil {
		fatal(err.Error())
	}
	defer store.Close()

	entries, err := store.List(ctx)
	if err != nil {
		fatal(err.Error())
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(entries); err != nil {
		fatal(err.Error())
	}
}

func cmdHealth(ctx context.Context, args []string) {
	if len(args) != 0 {
		fatal("usage: dad health")
	}
	var failed bool

	resolver := discovery.NewResolver(*dnsServer, *timeout)
	if err := resolver.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "DNS: FAIL — %v\n", err)
		failed = true
	} else {
		fmt.Printf("DNS: OK (%s)\n", *dnsServer)
	}

	store, err := newStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "etcd: FAIL — %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	if err := store.Ping(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "etcd: FAIL — %v\n", err)
		failed = true
	} else {
		fmt.Printf("etcd: OK (%s)\n", *etcdEndpoints)
	}

	if failed {
		os.Exit(1)
	}
}

func newStore() (*registration.Store, error) {
	endpoints := strings.Split(*etcdEndpoints, ",")
	for i := range endpoints {
		endpoints[i] = strings.TrimSpace(endpoints[i])
	}
	return registration.NewStore(registration.Config{
		Endpoints:     endpoints,
		PathPrefix:    *etcdPath,
		ClusterDomain: *clusterDomain,
		TTL:           *ttl,
		DialTimeout:   *timeout,
	})
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func fatal(msg string) {
	fmt.Fprintf(os.Stderr, "error: %s\n", msg)
	os.Exit(1)
}
