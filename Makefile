.PHONY: all build test lint helm-lint helm-template deploy smoke clean

MODULE := github.com/raygj/dns-agent-discovery
BIN := bin/dad
CHART := charts/dns-agent-discovery
NODE_IP ?= 192.168.1.98
DNS_PORT ?= 30053
ETCD_PORT ?= 32379

all: build test

build:
	go build -o $(BIN) ./cmd/dad

test:
	go test ./... -count=1 -cover

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not installed, skipping"; exit 0; }
	golangci-lint run ./...

helm-lint:
	helm lint $(CHART)

helm-template:
	helm template dns-agent-discovery $(CHART) --dry-run

deploy:
	helm upgrade --install dns-agent-discovery $(CHART) \
		--namespace dns-agent-discovery \
		--create-namespace \
		--wait --timeout 3m
	@echo "Waiting for pods..."
	kubectl rollout status deployment/etcd -n dns-agent-discovery --timeout=120s
	kubectl rollout status deployment/agent-coredns -n dns-agent-discovery --timeout=120s

smoke: build deploy
	@echo "=== dad health ==="
	DAD_DNS_SERVER=$(NODE_IP):$(DNS_PORT) \
	DAD_ETCD_ENDPOINTS=http://$(NODE_IP):$(ETCD_PORT) \
	$(BIN) health
	@echo "=== register db-reader ==="
	DAD_ETCD_ENDPOINTS=http://$(NODE_IP):$(ETCD_PORT) \
	$(BIN) register db-reader https://mcp.internal/v1/agents/db-reader --caps sql,crypto
	@sleep 2
	@echo "=== dad list ==="
	DAD_ETCD_ENDPOINTS=http://$(NODE_IP):$(ETCD_PORT) \
	$(BIN) list
	@echo "=== dad lookup ==="
	DAD_DNS_SERVER=$(NODE_IP):$(DNS_PORT) \
	DAD_ETCD_ENDPOINTS=http://$(NODE_IP):$(ETCD_PORT) \
	$(BIN) lookup db-reader
	@echo "=== deregister ==="
	DAD_ETCD_ENDPOINTS=http://$(NODE_IP):$(ETCD_PORT) \
	$(BIN) deregister db-reader
	@sleep 2
	@echo "=== lookup after deregister (expect failure) ==="
	DAD_DNS_SERVER=$(NODE_IP):$(DNS_PORT) \
	$(BIN) lookup db-reader && exit 1 || echo "NXDOMAIN confirmed"

clean:
	rm -rf bin/
