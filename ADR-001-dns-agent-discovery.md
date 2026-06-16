# ADR-001: DNS Agent Discovery

| Field       | Value                                      |
|-------------|---------------------------------------------|
| ID          | ADR-001                                     |
| Title       | DNS Agent Discovery                         |
| Status      | ACCEPTED                                    |
| Date        | 2026-05-27                                  |
| Author      | Jim Ray                                     |
| Domain      | Agentic Infrastructure / Service Discovery  |

---

## Context

Agentic systems require a lightweight, protocol-agnostic mechanism for agents to discover and connect to other agents and MCP-compatible tool servers at runtime. Current approaches rely on hardcoded environment variables, static service meshes, or application-layer registries — all of which couple the orchestration layer to deployment topology and break under ephemeral workload patterns.

DNS is a proven, universal pointer layer. Standard `SRV` and `TXT` records are sufficient to encode agent endpoint URIs, capability hints, and protocol metadata without violating DNS packet size constraints or introducing schema complexity into the name resolution plane. This ADR adopts DNS as the agent discovery plane for the project, deferring MCP tool schema exchange entirely to the MCP application layer handshake once a connection is established.

The broader IETF DNS-AID standardization effort is ongoing. This ADR delivers a working K8s-native implementation now, ahead of that spec, using patterns compatible with its likely direction.

---

## Decision

Implement a K8s-native DNS agent discovery system composed of:

1. **CoreDNS** deployed as a delegated sub-zone authority for `agents.<cluster-domain>`, backed by an etcd key-value store for dynamic record management.
2. **Go client library** (`dns-agent-discovery`) — a thin, importable library wrapping `miekg/dns` for programmatic agent lookup from any Go-based orchestrator or agent runtime.
3. **Go CLI binary** (`dad`) — a runnable diagnostic and registration tool for operator use and integration testing.
4. **Helm chart** — single-chart K8s distribution consistent with existing project packaging conventions.

DNS records are the **pointer layer only**. They return endpoint URI, protocol hint, and capability tags. MCP tool schemas and full tool listings are exchanged at the application layer via `tools/list` after the agent connects.

---

## Architecture

### Zone Design

```
agents.<cluster-domain>.          NS    agent-coredns.<cluster-domain>.
<capability>.agents.<cluster-domain>.  TXT   "url=<endpoint>" "caps=<tags>" "proto=mcp"
<capability>.agents.<cluster-domain>.  SRV   0 5 <port> <hostname>.<cluster-domain>.
```

TTL: `1s` globally on the `agents.*` sub-zone to support ephemeral agent lifecycles.

### Component Layout

```
┌────────────────────────────────────────────────────────────────┐
│                        K8s Cluster                             │
│                                                                │
│  ┌──────────────────┐        ┌──────────────────────────────┐  │
│  │   Agent / Orch   │──DNS──▶│         CoreDNS              │  │
│  │  (Go client lib) │        │  agents.<cluster-domain>     │  │
│  └──────────────────┘        │  TTL=1s / etcd backend       │  │
│                              └───────────────┬──────────────┘  │
│                                              │                 │
│                              ┌───────────────▼──────────────┐  │
│                              │          etcd                │  │
│                              │  (dynamic record store)      │  │
│                              └──────────────────────────────┘  │
│                                                                │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │              Corporate / Cluster Core DNS                │  │
│  │   NS delegation: agents.* → agent-coredns (static NS)   │  │
│  │   Zero zone transfers. Core DNS untouched.              │  │
│  └──────────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────────┘
```

### Discovery Sequence

```
1. Agent needs capability: "db-reader"
2. Lookup:  dig TXT db-reader.agents.<cluster-domain>
3. Response: "url=https://mcp.internal/v1/agents/db-reader" "caps=sql,crypto" "proto=mcp"
4. Agent opens HTTP/SSE or stdio connection to returned URL
5. Agent calls MCP tools/list — full schema exchange at application layer
6. DNS is done. MCP takes over.
```

---

## Go Packages

### Library: `dns-agent-discovery`

```
pkg/
  discovery/
    client.go        // Lookup(capability string) → AgentRecord, error
    record.go        // AgentRecord struct: URL, Caps []string, Proto string
    resolver.go      // miekg/dns wrapper, configurable server + timeout
  registration/
    register.go      // Write TXT+SRV records to etcd backing store
    deregister.go    // Remove records on agent shutdown
```

### CLI: `dad` (DNS Agent Discovery)

```
cmd/dad/
  main.go

Subcommands:
  dad lookup <capability>          // resolve and print AgentRecord
  dad register <capability> <url>  // write record to etcd
  dad deregister <capability>      // remove record
  dad list                         // dump all agents.* records
  dad health                       // check CoreDNS + etcd reachability
```

---

## Helm Chart

```
charts/dns-agent-discovery/
  Chart.yaml
  values.yaml
  templates/
    coredns-configmap.yaml       // Corefile with etcd plugin config
    coredns-deployment.yaml
    coredns-service.yaml         // ClusterIP + NodePort for lab access
    etcd-deployment.yaml
    etcd-service.yaml
    rbac.yaml
    namespace.yaml
```

`values.yaml` exposes: cluster domain, etcd connection, CoreDNS replica count, TTL override, resource limits.

---

## Build Phase

### Phase 1 — Single Phase, Full Delivery

| Stage | Tasks |
|-------|-------|
| **Scaffold** | Repo init, Go module, Helm chart skeleton, CI skeleton |
| **CoreDNS + etcd** | CoreDNS `etcd` plugin config, zone delegation, TTL=1s |
| **Library** | `Lookup()`, `Register()`, `Deregister()` — unit tested |
| **CLI** | `dad` binary — all subcommands wired |
| **Helm** | Chart linted, values documented, dry-run clean |
| **CI** | Full gate pipeline (see below) |
| **Forge Integration** | Deploy to Forge commune Talos lab, run integration test plan |
| **Red Team** | Security review gate before final green light |
| **Release** | Tag, publish chart, publish Go module |

---

## CI Pipeline

All gates must pass before merge to `main`.

| Gate | Tool | Pass Criteria |
|------|------|---------------|
| **Lint** | `golangci-lint` | Zero warnings, enforced ruleset |
| **Unit Tests** | `go test ./...` | 100% pass, coverage ≥ 80% |
| **CVE Scan** | `govulncheck` + `trivy` (image + chart) | Zero HIGH/CRITICAL |
| **Helm Lint** | `helm lint` | Clean |
| **Helm Template** | `helm template --dry-run` | Renders without error |
| **Integration Tests** | Forge commune Talos lab (see below) | All scenarios pass |
| **Red Team Gate** | Manual security review | Sign-off required before release tag |

---

## Forge Commune Integration Test Plan (Task: TBD post-build)

> **Status:** Task to be created on completion of Phase 1 build. Assigned to Forge validation commune.

### Scenarios to Cover (Minimum)

| # | Scenario | Pass Criteria |
|---|----------|---------------|
| 1 | Register agent, lookup returns correct record | Exact match on URL, caps, proto |
| 2 | Deregister agent, subsequent lookup returns NXDOMAIN | No stale record |
| 3 | Update agent endpoint (re-register), client resolves new URL within TTL window | ≤ 2s propagation |
| 4 | CoreDNS pod restart — lookup recovers from etcd without data loss | Zero record loss |
| 5 | etcd pod restart — CoreDNS degrades gracefully, recovers on etcd return | No crash, correct recovery |
| 6 | 100 concurrent agent lookups — latency and error rate | p99 ≤ 50ms, 0% error |
| 7 | `dad` CLI — all subcommands against live cluster | Expected output, exit 0 |
| 8 | Corporate DNS delegation — lookup from outside `agents.*` sub-zone resolves correctly | Correct NS delegation traversal |

---

## Red Team Checklist (Pre-Release Gate)

- [ ] DNS amplification / reflection risk assessment
- [ ] etcd ACL and network policy — no unauthenticated write path
- [ ] TXT record injection — can an unauthorized agent register a spoofed capability?
- [ ] TTL abuse — can a malicious record persist beyond intended window?
- [ ] `dad` CLI — any privilege escalation surface?
- [ ] Helm chart RBAC — least privilege confirmed
- [ ] CVE scan clean at release commit (not just at CI time)

---

## Consequences

**Accepted tradeoffs:**
- CoreDNS + etcd adds two infrastructure components. Mitigated by Helm single-chart delivery and small resource footprint. etcd is first-party supported by the CoreDNS core team, eliminating the plugin maintenance risk of the Redis alternative.
- This implementation predates DNS-AID spec ratification. Record schema (`url`, `caps`, `proto`) is intentionally minimal and forward-compatible. Migration cost when spec lands is bounded to record format only.
- No auth on the DNS query plane. Agent registration protected by etcd ACL + network policy. Acceptable for initial release; mTLS on the registration path is a future ADR candidate.

**What this does not solve:**
- MCP tool schema registry — out of scope by design, handled at MCP application layer.
- Cross-cluster or federated discovery — future ADR.
- Agent health / readiness signaling — TTL is a blunt instrument. Health-aware registration is a follow-on.

---

## Alternatives Considered

| Option | Rejected Because |
|--------|-----------------|
| Hardcoded env vars | Breaks under ephemeral workloads, couples config to topology |
| Kubernetes Service + Endpoints API | K8s-internal only; not portable to non-K8s agent runtimes |
| Consul service catalog | Adds full service mesh dependency; heavyweight for this scope |
| Application-layer registry (e.g., custom API) | Another moving part; DNS is already present everywhere |

---

## References

- CoreDNS etcd plugin: https://coredns.io/plugins/etcd/
- miekg/dns Go library: https://github.com/miekg/dns
- MCP Specification: https://spec.modelcontextprotocol.io
- IETF DNS-AID (draft): in progress — monitor for schema alignment
- Gemini session: DNS as Agent Plane architectural analysis (2026-05-27)
