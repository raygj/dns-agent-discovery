# Demo — try it in 5 seconds

No Kubernetes required. Spins up **etcd + agent-coredns** locally via Docker Compose, registers a sample agent, and verifies DNS lookup.

## One command

```bash
cd demo
./try.sh
```

Or without the helper script (works with `docker compose` or `docker-compose`):

```bash
cd demo
docker compose up -d etcd coredns    # or: docker-compose up -d etcd coredns
docker compose run --rm --build bootstrap
```

Expected output ends with:

```
==> DNS TXT
"url=https://mcp.example.com/v1/agents/db-reader"
"proto=mcp"
"caps=sql,crypto"

==> DNS SRV
0 100 443 mcp.example.com.

Demo OK.
```

## Try from your host

While the stack is running:

```bash
# DNS lookup
dig @127.0.0.1 -p 5353 db-reader.agents.cluster.local TXT +short

# dad CLI (build from repo root first: make build)
../bin/dad --dns-server 127.0.0.1:5353 \
           --etcd-endpoints http://127.0.0.1:2379 \
           lookup db-reader
```

## Ports

| Service | Host port | Purpose |
|---------|-----------|---------|
| agent-coredns | `5353/udp` | DNS queries (5353 avoids macOS port-53 conflicts) |
| etcd | `2379/tcp` | Registration writes |

## Cleanup

```bash
docker compose down
```

## What's inside

```
demo/
  docker-compose.yml    etcd + coredns + bootstrap services
  Dockerfile.bootstrap  alpine + dig/curl, runs bootstrap.sh
  bootstrap.sh          writes SkyDNS keys, verifies TXT+SRV
  Corefile              agents.cluster.local zone (same as Helm chart)
  try.sh                one-liner entrypoint
```

The bootstrap container uses the same SkyDNS key layout as the Go registration library and Helm deployment — so behavior matches production.
