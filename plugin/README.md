# caddy-admin Plugin for Claude Code

Scaffold and manage dynamic services for the [caddy-admin](https://github.com/jssfy/caddy-admin) reverse proxy platform.

## Install

```bash
# Add the marketplace (once)
claude plugin marketplace add jssfy/caddy-admin

# Install the plugin
claude plugin install caddy-admin@jssfy-caddy-admin
```

Or test locally:

```bash
claude --plugin-dir ./plugin
```

## Skills

### `/caddy-admin:scaffold-service`

Generate a complete project directory that auto-registers with caddy-admin on startup.

```bash
# Go backend (default)
/caddy-admin:scaffold-service my-project

# Node.js backend
/caddy-admin:scaffold-service my-project --lang node

# Static site only (no backend)
/caddy-admin:scaffold-service my-project --lang static

# Custom backend port
/caddy-admin:scaffold-service my-project --port 9090
```

Generated files:

```
my-project/
├── .env                  # Service registration config
├── register.sh           # Auto-registration sidecar script
├── docker-compose.yml    # caddy-net + register sidecar
├── Makefile              # up / down / logs
├── backend/
│   ├── main.go           # (or index.js for node)
│   └── Dockerfile
└── frontend/
    ├── index.html
    ├── nginx.conf
    └── Dockerfile
```

After generation:

```bash
cd my-project
docker compose up -d --build
docker compose logs my-project-register  # confirm "registered successfully"
open https://my-project.yeanhua.asia
```

## Prerequisites

- caddy-admin infrastructure running (`docker compose up -d` in caddy-admin repo)
- Docker network `caddy-net` exists (created by caddy-admin)
- DNS: `*.yeanhua.asia` resolves to your server (or add to `/etc/hosts` for local dev)

## Demo

See [project-c](https://github.com/jssfy/project-c) for a working example of a dynamically registered service.
