# dns-agent-discovery

K8s-native DNS agent discovery — CoreDNS + etcd sub-zone authority for `agents.<cluster-domain>`.

See [ADR-001](ADR-001-dns-agent-discovery.md) for architecture decisions.

## Quick Start (Colima / local K8s)

```bash
# Build CLI
make build

# Deploy CoreDNS + etcd to cluster
make deploy

# Run integration smoke test
make smoke NODE_IP=192.168.1.98
```

## CLI: `dad`

```bash
# Health check
dad --dns-server 192.168.1.98:30053 --etcd-endpoints http://192.168.1.98:32379 health

# Register an agent capability
dad register db-reader https://mcp.internal/v1/agents/db-reader --caps sql,crypto

# Lookup via DNS
dad --dns-server 192.168.1.98:30053 lookup db-reader

# List all registered agents (etcd)
dad list

# Deregister
dad deregister db-reader
```

Environment variables: `DAD_CLUSTER_DOMAIN`, `DAD_DNS_SERVER`, `DAD_ETCD_ENDPOINTS`, `DAD_ETCD_PATH`.

## Go Library

```go
import "github.com/raygj/dns-agent-discovery/pkg/discovery"

client := discovery.NewClient("cluster.local", "192.168.1.98:30053", 2*time.Second)
rec, err := client.Lookup("db-reader")
// rec.URL, rec.Caps, rec.Proto
```

```go
import "github.com/raygj/dns-agent-discovery/pkg/registration"

store, _ := registration.NewStore(registration.Config{
    Endpoints:     []string{"http://127.0.0.1:32379"},
    ClusterDomain: "cluster.local",
})
store.Register(ctx, registration.RegisterOptions{
    Capability: "db-reader",
    URL:        "https://mcp.internal/v1/agents/db-reader",
    Caps:       []string{"sql", "crypto"},
    Proto:      "mcp",
})
```

## Helm

```bash
helm upgrade --install dns-agent-discovery charts/dns-agent-discovery \
  --namespace dns-agent-discovery --create-namespace
```

NodePort defaults: DNS `30053/udp`, etcd `32379`.

## Discovery Flow

1. Agent registers capability → etcd (TXT + SRV in SkyDNS format)
2. CoreDNS serves `agents.cluster.local` from etcd (TTL=1s)
3. Orchestrator looks up `db-reader.agents.cluster.local` TXT+SRV
4. Agent connects to returned URL, MCP `tools/list` handles schema exchange
