# dns-agent-discovery

K8s-native DNS agent discovery — a lightweight pointer layer for agents and MCP-compatible tool servers at runtime.

DNS records answer *where* an agent lives and *what* it can do. MCP schema exchange happens afterward at the application layer via `tools/list`. See [ADR-001](ADR-001-dns-agent-discovery.md) for the full architecture decision.

**Status:** v0.1.0 prototype — deployed and smoke-tested on Colima/Talos.

---

## Problem

Agentic systems need to find each other without hardcoded endpoints, static service meshes, or bespoke registries. Those approaches couple orchestration to deployment topology and break under ephemeral workloads.

DNS is already everywhere. Standard `TXT` and `SRV` records are enough to encode endpoint URIs, capability tags, and protocol hints — without stuffing tool schemas into the name resolution plane.

---

## Core System

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           Kubernetes Cluster                            │
│                                                                         │
│  ┌─────────────────────┐         register / deregister                  │
│  │  Agent or Operator  │──────────────────────────────────────┐         │
│  │  (dad CLI / Go lib) │                                      │         │
│  └──────────┬──────────┘                                      ▼         │
│             │ lookup (DNS)                         ┌──────────────────┐ │
│             │                                      │       etcd       │ │
│             │                                      │  /skydns/...     │ │
│             │                                      │  SkyDNS JSON     │ │
│             │                                      └────────▲─────────┘ │
│             │                                               │           │
│             ▼                                               │ read      │
│  ┌─────────────────────┐                                    │           │
│  │    agent-coredns    │────────────────────────────────────┘           │
│  │ agents.cluster.local│  etcd plugin, TTL=1s, no zone cache            │
│  │   (Helm deploy)     │                                                │
│  └──────────┬──────────┘                                                │
│             │                                                           │
│  ┌──────────▼──────────┐                                                │
│  │  Cluster CoreDNS      │  NS delegation (future):                     │
│  │  (kube-dns)           │  agents.* → agent-coredns                     │
│  └─────────────────────┘  not required for NodePort lab access          │
│                                                                         │
│  NodePort (lab):  DNS 30053/udp   etcd 32379/tcp                        │
└─────────────────────────────────────────────────────────────────────────┘
```

### DNS Zone Layout

```
agents.<cluster-domain>.                         NS    agent-coredns
<capability>.agents.<cluster-domain>.            TXT   "url=..." "proto=mcp" "caps=..."
<capability>.agents.<cluster-domain>.            SRV   0 5 <port> <hostname>.
```

Records are stored in etcd using the [CoreDNS SkyDNS format](https://coredns.io/plugins/etcd/) — one key per TXT string (`/txt0`, `/txt1`, …) plus a `/srv` key. TTL defaults to **1 second** for ephemeral agent lifecycles.

---

## Discovery Workflow

```
  Orchestrator / Agent                    agent-coredns              etcd
        │                                      │                      │
        │  1. needs capability "db-reader"     │                      │
        │                                      │                      │
        │  2. DNS TXT+SRV query                  │                      │
        │     db-reader.agents.cluster.local     │                      │
        │─────────────────────────────────────▶│                      │
        │                                      │  3. prefix lookup    │
        │                                      │─────────────────────▶│
        │                                      │◀─────────────────────│
        │  4. TXT: url, caps, proto              │                      │
        │     SRV: port, hostname                │                      │
        │◀─────────────────────────────────────│                      │
        │                                      │                      │
        │  5. connect to returned URL            │                      │
        │     (HTTP/SSE/stdio)                   │                      │
        │                                      │                      │
        │  6. MCP tools/list — schema exchange   │                      │
        │     DNS is done. MCP takes over.       │                      │
        ▼                                      │                      │
```

Example lookup result:

```json
{
  "capability": "db-reader",
  "url": "https://mcp.internal/v1/agents/db-reader",
  "caps": ["sql", "crypto"],
  "proto": "mcp",
  "srv_host": "mcp.internal",
  "srv_port": 443
}
```

---

## Release Roadmap

| Capability | v0.1.0 (current) | Planned |
|---|---|---|
| **Helm chart** — CoreDNS + etcd single-chart deploy | ✅ | |
| **`agents.*` sub-zone** — delegated authority with TTL=1s | ✅ | |
| **Go discovery library** — `Lookup()` via TXT+SRV | ✅ | |
| **Go registration library** — `Register()`, `Deregister()`, `List()` | ✅ | |
| **`dad` CLI** — lookup, register, deregister, list, health | ✅ | |
| **NodePort lab access** — direct DNS/etcd from host | ✅ | |
| **SkyDNS/etcd record format** — CoreDNS-compatible keys | ✅ | |
| **Smoke test** — register → lookup → deregister → NXDOMAIN | ✅ | |
| **Docker demo** — `demo/try.sh` local stack, no Kubernetes | ✅ | |
| **CI pipeline** — lint, test, coverage ≥60%, govulncheck, helm | ✅ | cluster smoke in CI |
| **Cluster DNS delegation** — NS record from kube-dns to agent-coredns | | 🔜 |
| **Forge commune integration tests** — 8-scenario validation plan (ADR) | | 🔜 |
| **Registration auth** — etcd ACL + NetworkPolicy, mTLS on write path | | 🔜 |
| **DNS-AID alignment** — schema migration when IETF draft ratifies | | 🔜 |
| **Cross-cluster discovery** — federated agent lookup | | future ADR |
| **Health-aware registration** — readiness beyond TTL expiry | | future ADR |
| **Red team gate** — pre-release security review checklist | | 🔜 |
| **Published releases** — tagged Go module + Helm chart OCI | | 🔜 |

---

## CI

All gates run on push/PR to `main` via [GitHub Actions](.github/workflows/ci.yml). Run locally:

```bash
make ci
```

| Gate | Tool | Pass criteria | Local command |
|------|------|---------------|---------------|
| **Build** | `go build` | `dad` compiles | `make build` |
| **Unit tests** | `go test` | 100% pass | `make test` |
| **Coverage** | `go tool cover` | ≥ 60% statements | `go test ./... -coverprofile=coverage.out && go tool cover -func=coverage.out` |
| **Lint** | `golangci-lint` v2 | 0 issues | `make lint` |
| **Vuln scan** | `govulncheck` | 0 vulns in your code | `govulncheck ./...` |
| **Helm lint** | `helm lint` | clean | `make helm-lint` |
| **Helm template** | `helm template` | renders | `make helm-template` |
| **Demo smoke** | Docker Compose | register → lookup OK | `make demo` |
| **Cluster smoke** | Helm + `dad` | full ADR scenario | `make smoke NODE_IP=<node>` |

Current local baseline: **~67%** statement coverage (`pkg/discovery` ~94%, `pkg/registration` ~93%).

---

## Forward-Looking: Alignment with DNS-AID

This project predates awareness of the IETF work, but converges on the same thesis. [DNS for AI Discovery (DNS-AID)](https://datatracker.ietf.org/doc/draft-mozleywilliams-dnsop-dnsaid/) (`draft-mozleywilliams-dnsop-dnsaid-02`, May 2026) standardizes DNS as a pointer layer for AI agent discovery — same day ADR-001 was written.

**What we share:** DNS answers *where* to connect and *what* protocol/capability hints apply. Full schemas and tool listings stay at the application layer (MCP `tools/list`, agent cards, capability descriptors). No bespoke registry API. Direct connect after lookup.

**What differs today:** DNS-AID is org-scoped and SVCB-first; this repo is cluster-scoped and TXT+SRV-first for CoreDNS/etcd compatibility.

### Discovery use cases

DNS-AID defines three requestor scenarios:

| Use case | Requestor knows | DNS-AID approach | This project |
|----------|-----------------|------------------|--------------|
| **1. Known agent** | Org + agent name | SVCB query at `agent-name.example.com` | Not primary — we name by capability, not agent |
| **2. Known org** | Org, not agent | `_index._agents.example.com` → org index → select agent | Not yet — single cluster trust domain |
| **3. Known capability** | Capability, not org/agent | **Out of draft scope** — needs external search | **Primary** — `db-reader.agents.cluster.local` |

We effectively implement a **cluster-local variant of (3)**: capability-first lookup inside one trust boundary (`cluster.local`), without cross-org federation.

### Record format comparison

| Concern | DNS-AID (normative) | This project (v0.1.0) |
|---------|---------------------|------------------------|
| **Primary RR** | [SVCB](https://datatracker.ietf.org/doc/html/rfc9460) — target, port, ALPN, structured SvcParams | TXT + SRV (SkyDNS/etcd via CoreDNS) |
| **TXT role** | Fallback when SVCB unavailable (Section 4) | Primary metadata carrier |
| **Protocol** | `alpn` / `bap` (e.g. `mcp`, `a2a`) | `proto=mcp` in TXT |
| **Endpoint** | SVCB TargetName + `port` + hints | `url=https://...` in TXT + SRV host/port |
| **Capabilities** | `cap` = descriptor URI; optional `cap-sha256` | `caps=sql,crypto` inline tags |
| **Agent card** | `well-known=/.well-known/agent-card.json` | Deferred to MCP handshake |
| **Trust** | DNSSEC + TLSA recommended | Open query plane; registration auth planned |
| **TTL** | Cacheable (e.g. 3600s) — "learnable as a skill" | 1s — ephemeral K8s agent lifecycles |

### Side-by-side flow

```
DNS-AID (org / internet scale)              This project (cluster scale)
────────────────────────────────            ────────────────────────────
agent-name.example.com                      db-reader.agents.cluster.local
    │ SVCB (+ TLSA)                             │ TXT + SRV
    ├─ alpn, bap, port, hints                   ├─ url=, proto=, caps=
    ├─ well-known path                          └─ (no well-known in DNS)
    └─ cap URI + cap-sha256
         │                                          │
         ▼                                          ▼
   fetch agent card / descriptor                 MCP tools/list
   validate TLS via TLSA                         connect to url
```

### Positioning

We did not implement DNS-AID. We built a **K8s-native lab prototype** that validates the same pointer-layer idea with tooling that ships today (CoreDNS + etcd + SkyDNS keys). Migration cost when the draft stabilizes is bounded to **record format and naming** — not architecture.

### Planned alignment (when draft matures)

| Gap | Direction |
|-----|-----------|
| SVCB records | Publish SVCB alongside or instead of TXT+SRV; evaluate CoreDNS SVCB + etcd path |
| Org index | Add `_index._agents.<domain>` for use case (2) |
| Capability descriptors | Replace inline `caps=` tags with `cap` URI + optional `cap-sha256` |
| DNSSEC / TLSA | Adopt for production trust boundary (see ADR red-team checklist) |
| Cross-org capability search | Future ADR — federated index derived from (2), as draft suggests for (3) |

Track draft revisions at [datatracker.ietf.org/doc/draft-mozleywilliams-dnsop-dnsaid/](https://datatracker.ietf.org/doc/draft-mozleywilliams-dnsop-dnsaid/).

---

## Quick Start

### Try it in 5 seconds (no Kubernetes)

Requires Docker only.

```bash
cd demo
./try.sh
```

Docker Compose spins up etcd + agent-coredns, registers a sample agent, and verifies DNS lookup. See [demo/README.md](demo/README.md).

### Full cluster deploy

Requires Go 1.23+ (toolchain **1.26.4** recommended), Helm, and a Kubernetes cluster (tested on Colima/Talos).

```bash
make build

# Deploy CoreDNS + etcd
make deploy

# Full integration smoke test (set NODE_IP to your cluster node)
make smoke NODE_IP=192.168.1.98
```

---

## CLI: `dad`

```bash
# Health check
dad --dns-server 192.168.1.98:30053 \
    --etcd-endpoints http://192.168.1.98:32379 \
    health

# Register an agent capability
dad register db-reader https://mcp.internal/v1/agents/db-reader --caps sql,crypto

# Lookup via DNS
dad --dns-server 192.168.1.98:30053 lookup db-reader

# List all registered agents (reads etcd directly)
dad list

# Deregister
dad deregister db-reader
```

| Flag / Env | Default | Description |
|---|---|---|
| `--cluster-domain` / `DAD_CLUSTER_DOMAIN` | `cluster.local` | Cluster domain suffix |
| `--dns-server` / `DAD_DNS_SERVER` | `127.0.0.1:53` | DNS server for lookups |
| `--etcd-endpoints` / `DAD_ETCD_ENDPOINTS` | `http://127.0.0.1:2379` | Comma-separated etcd URLs |
| `--etcd-path` / `DAD_ETCD_PATH` | `/skydns` | CoreDNS etcd path prefix |
| `--ttl` | `1` | Record TTL in seconds |

---

## Go Library

### Discovery

```go
import (
    "time"
    "github.com/raygj/dns-agent-discovery/pkg/discovery"
)

client := discovery.NewClient("cluster.local", "192.168.1.98:30053", 2*time.Second)
rec, err := client.Lookup("db-reader")
// rec.URL, rec.Caps, rec.Proto, rec.SRVHost, rec.SRVPort
```

### Registration

```go
import (
    "context"
    "github.com/raygj/dns-agent-discovery/pkg/registration"
)

store, err := registration.NewStore(registration.Config{
    Endpoints:     []string{"http://192.168.1.98:32379"},
    ClusterDomain: "cluster.local",
    TTL:           1,
})
defer store.Close()

store.Register(ctx, registration.RegisterOptions{
    Capability: "db-reader",
    URL:        "https://mcp.internal/v1/agents/db-reader",
    Caps:       []string{"sql", "crypto"},
    Proto:      "mcp",
})
```

---

## Helm

```bash
helm upgrade --install dns-agent-discovery charts/dns-agent-discovery \
  --namespace dns-agent-discovery \
  --create-namespace
```

Key `values.yaml` settings:

| Value | Default | Description |
|---|---|---|
| `clusterDomain` | `cluster.local` | Zone suffix for `agents.*` |
| `coredns.service.nodePort` | `30053` | External DNS access (UDP) |
| `etcd.service.nodePort` | `32379` | External etcd access |
| `coredns.ttl` | `1` | Record TTL written by registration |

---

## Project Layout

```
cmd/dad/                  CLI binary
pkg/discovery/            DNS lookup client (miekg/dns)
pkg/registration/         etcd register/deregister/list
charts/dns-agent-discovery/   Helm: agent-coredns + etcd
ADR-001-dns-agent-discovery.md
```

---

## What This Does Not Solve

By design — see ADR-001 for rationale:

- **MCP tool schema registry** — handled at application layer after connect
- **Cross-cluster / federated discovery** — future ADR
- **Agent health / readiness signaling** — TTL is a blunt instrument for now
- **Auth on the DNS query plane** — queries are open; registration writes should be protected (future)

---

## References

- [ADR-001: DNS Agent Discovery](ADR-001-dns-agent-discovery.md)
- [CoreDNS etcd plugin](https://coredns.io/plugins/etcd/)
- [miekg/dns](https://github.com/miekg/dns)
- [MCP Specification](https://spec.modelcontextprotocol.io)
- [IETF DNS-AID draft — DNS for AI Discovery](https://datatracker.ietf.org/doc/draft-mozleywilliams-dnsop-dnsaid/)
