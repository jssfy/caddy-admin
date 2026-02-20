.PHONY: build-frontend up down logs clean add-dns

# Build frontend static files (required before first `make up`)
build-frontend:
	cd caddy-admin/frontend && npm install && npm run build

# Start all services (run `make build-frontend` first)
up:
	docker compose up -d --build
	@echo ""
	@echo "Services started:"
	@echo "  caddy-admin:  https://caddy-admin.yeanhua.asia"
	@echo "  site-a:       https://site-a.yeanhua.asia"
	@echo "  site-b:       https://site-b.yeanhua.asia"
	@echo "  caddy admin:  http://localhost:2019/config/"
	@echo ""
	@echo "Note: Ensure DNS A records point to this server:"
	@echo "  caddy-admin.yeanhua.asia  site-a.yeanhua.asia  site-b.yeanhua.asia"

# Start with logs in foreground
up-logs:
	docker compose up --build

down:
	docker compose down

logs:
	docker compose logs -f

# Test: query Caddy Admin API directly
test-caddy-api:
	@echo "=== Caddy config (HTTP servers) ==="
	curl -s http://localhost:2019/config/apps/http/servers | python3 -m json.tool | head -60
	@echo ""
	@echo "=== caddy-admin sites API ==="
	curl -s http://localhost:8090/api/sites | python3 -m json.tool
	@echo ""
	@echo "=== caddy-admin certs API ==="
	curl -s http://localhost:8090/api/certs | python3 -m json.tool

# Add DNS A record for a new project: make add-dns RR=my-project
ECS_IP ?= 121.41.107.93
add-dns:
ifndef RR
	$(error Usage: make add-dns RR=<subdomain>  Example: make add-dns RR=project-c)
endif
	aliyun alidns AddDomainRecord \
		--DomainName yeanhua.asia \
		--RR "$(RR)" \
		--Type A \
		--Value "$(ECS_IP)"
	@echo ""
	@echo "DNS record added: $(RR).yeanhua.asia -> $(ECS_IP)"
	@echo "Verifying..."
	@sleep 3
	@dig +short $(RR).yeanhua.asia || echo "(may take a few minutes to propagate)"

clean:
	docker compose down -v
	rm -rf caddy-admin/frontend/dist
