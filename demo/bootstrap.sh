#!/bin/sh
set -eu

ETCD_URL="${ETCD_URL:-http://etcd:2379}"
DNS_HOST="${DNS_HOST:-coredns}"
DNS_PORT="${DNS_PORT:-53}"
CAPABILITY="${CAPABILITY:-db-reader}"
AGENT_URL="${AGENT_URL:-https://mcp.example.com/v1/agents/db-reader}"
BASE="/skydns/local/cluster/agents/${CAPABILITY}"

b64() {
	printf '%s' "$1" | base64 | tr -d '\n'
}

etcd_put() {
	key="$1"
	value="$2"
	curl -sf -X POST "${ETCD_URL}/v3/kv/put" \
		-H "Content-Type: application/json" \
		-d "{\"key\":\"$(b64 "$key")\",\"value\":\"$(b64 "$value")\"}" \
		>/dev/null
}

echo "==> waiting for etcd..."
until curl -sf "${ETCD_URL}/health" >/dev/null; do sleep 0.3; done

echo "==> waiting for agent-coredns..."
until curl -sf http://coredns:8080/health | grep -q OK; do sleep 0.3; done

echo "==> registering capability: ${CAPABILITY}"
etcd_put "${BASE}/txt0" "{\"ttl\":1,\"text\":\"url=${AGENT_URL}\"}"
etcd_put "${BASE}/txt1" "{\"ttl\":1,\"text\":\"proto=mcp\"}"
etcd_put "${BASE}/txt2" "{\"ttl\":1,\"text\":\"caps=sql,crypto\"}"
etcd_put "${BASE}/srv"  '{"host":"mcp.example.com.","port":443,"priority":0,"weight":5,"ttl":1}'

sleep 1

FQDN="${CAPABILITY}.agents.cluster.local"
echo ""
echo "==> DNS TXT"
dig @"${DNS_HOST}" -p "${DNS_PORT}" "${FQDN}" TXT +short

echo ""
echo "==> DNS SRV"
dig @"${DNS_HOST}" -p "${DNS_PORT}" "${FQDN}" SRV +short

echo ""
echo "Demo OK."
echo ""
echo "From your host (while compose is running):"
echo "  dig @127.0.0.1 -p 5353 ${FQDN} TXT +short"
echo ""
echo "With the dad CLI (optional — run 'make build' from repo root first):"
echo "  dad --dns-server 127.0.0.1:5353 --etcd-endpoints http://127.0.0.1:2379 lookup ${CAPABILITY}"
