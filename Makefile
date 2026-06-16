.PHONY: all build test lint helm-lint helm-template deploy smoke demo ci clean

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
	helm template dns-agent-discovery $(CHART) --dry-run=client > /dev/null

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

demo:
	cd demo && ./try.sh

ci: build test lint helm-lint
	helm template dns-agent-discovery $(CHART) --dry-run=client > /dev/null
	@command -v govulncheck >/dev/null 2>&1 || go install golang.org/x/vuln/cmd/govulncheck@latest
	@PATH="$$(go env GOPATH)/bin:$$PATH" govulncheck ./...
	@go test ./... -count=1 -coverprofile=coverage.out
	@go tool cover -func=coverage.out | tail -1
	@total=$$(go tool cover -func=coverage.out | awk '/total:/ {gsub(/%/,"",$$3); print $$3}'); \
	 echo "Coverage: $$total%"; \
	 awk -v c="$$total" 'BEGIN { exit (c+0 >= 60) ? 0 : 1 }'

clean:
	rm -rf bin/
