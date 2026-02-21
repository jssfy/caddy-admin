.PHONY: build-frontend up down logs clean add-dns hosts-add hosts-remove cert-issue cert-renew

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

# Add local /etc/hosts entries for development (requires sudo)
HOSTS_BLOCK = \# BEGIN caddy-admin\n\
127.0.0.1 caddy-admin.yeanhua.asia\n\
127.0.0.1 site-a.yeanhua.asia\n\
127.0.0.1 site-b.yeanhua.asia\n\
127.0.0.1 project-c.yeanhua.asia\n\
127.0.0.1 project-d-stripe.yeanhua.asia\n\
\# END caddy-admin\n

hosts-add:
	@if grep -q "# BEGIN caddy-admin" /etc/hosts; then \
		echo "Hosts entries already exist, skipping."; \
	else \
		printf '$(HOSTS_BLOCK)' | sudo tee -a /etc/hosts > /dev/null; \
		echo "Hosts entries added."; \
	fi

# Remove local /etc/hosts entries added by hosts-add (requires sudo)
hosts-remove:
	@if grep -q "# BEGIN caddy-admin" /etc/hosts; then \
		sudo sed -i '' '/# BEGIN caddy-admin/,/# END caddy-admin/d' /etc/hosts; \
		echo "Hosts entries removed."; \
	else \
		echo "No hosts entries found, nothing to remove."; \
	fi

# Issue wildcard TLS cert via acme.sh DNS-01 (requires Ali_Key / Ali_Secret env vars)
# Run once; re-running is safe (skips if cert is still valid)
cert-issue:
	mkdir -p $(HOME)/certs/yeanhua.asia
	docker run --rm -it \
		-v "$(HOME)/.acme.sh:/acme.sh" \
		-e Ali_Key="$(Ali_Key)" \
		-e Ali_Secret="$(Ali_Secret)" \
		neilpang/acme.sh \
		--issue -d "*.yeanhua.asia" -d yeanhua.asia \
		--dns dns_ali --server letsencrypt
	docker run --rm -it \
		-v "$(HOME)/.acme.sh:/acme.sh" \
		-v "$(HOME)/certs/yeanhua.asia:/certs" \
		neilpang/acme.sh \
		--install-cert -d "*.yeanhua.asia" \
		--fullchain-file /certs/fullchain.pem \
		--key-file       /certs/key.pem
	@echo ""
	@echo "Cert installed to ~/certs/yeanhua.asia/{fullchain,key}.pem"

# Renew cert if expiring soon, reinstall, then restart Caddy to pick up new cert
cert-renew:
	docker run --rm -it \
		-v "$(HOME)/.acme.sh:/acme.sh" \
		neilpang/acme.sh --cron
	docker run --rm -it \
		-v "$(HOME)/.acme.sh:/acme.sh" \
		-v "$(HOME)/certs/yeanhua.asia:/certs" \
		neilpang/acme.sh \
		--install-cert -d "*.yeanhua.asia" \
		--fullchain-file /certs/fullchain.pem \
		--key-file       /certs/key.pem
	docker compose restart caddy
	@echo ""
	@echo "Cert renewed and Caddy restarted."

clean:
	docker compose down -v
	rm -rf caddy-admin/frontend/dist
