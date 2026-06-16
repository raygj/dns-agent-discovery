#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

if docker compose version >/dev/null 2>&1; then
	COMPOSE=(docker compose)
elif command -v docker-compose >/dev/null 2>&1; then
	COMPOSE=(docker-compose)
else
	echo "error: docker compose or docker-compose is required" >&2
	exit 1
fi

echo "Starting etcd + agent-coredns..."
"${COMPOSE[@]}" up -d etcd coredns

echo "Running bootstrap (register + lookup)..."
"${COMPOSE[@]}" run --rm --build bootstrap

echo ""
echo "Stack is still running. Stop with: ${COMPOSE[*]} down"
