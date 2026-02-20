---
name: scaffold-service
description: >
  Scaffold a new dynamic service project for caddy-admin reverse proxy platform.
  Use when user says "create new service", "scaffold service", "new project for caddy",
  "接入新项目", "创建新服务", or wants to generate boilerplate for caddy-admin registration.
  Generates .env, register.sh, docker-compose.yml, Makefile, and backend/frontend code.
user-invocable: true
disable-model-invocation: true
argument-hint: <project-name> [--lang go|node|static] [--port 8080]
allowed-tools:
  - Write
  - Bash
  - Read
  - Glob
---

# scaffold-service

Generate a complete, runnable project directory that auto-registers with caddy-admin on startup.

## Step 1: Parse Arguments

Extract from `$ARGUMENTS`:

- **project-name** (REQUIRED): first positional argument, e.g. `my-project`. Must be lowercase with hyphens only.
- **--lang** (optional): `go` (default), `node`, or `static` (no backend)
- **--port** (optional): backend listen port, default `8080`

If project-name is missing, ask the user with AskUserQuestion.

Derive all variables from the project name:

| Variable | Value |
|----------|-------|
| `PROJECT_NAME` | the project name as-is |
| `SERVICE_DOMAIN` | `{PROJECT_NAME}.yeanhua.asia` |
| `SERVICE_UPSTREAM` | `{PROJECT_NAME}-frontend:80` |
| `CADDY_ADMIN_URL` | `http://caddy-admin-api:8090` (fixed) |
| `BACKEND_SERVICE` | `{PROJECT_NAME}-backend` |
| `FRONTEND_SERVICE` | `{PROJECT_NAME}-frontend` |
| `REGISTER_SERVICE` | `{PROJECT_NAME}-register` |
| `BACKEND_PORT` | from `--port` or `8080` |

## Step 2: Check Target Directory

Check if `./{PROJECT_NAME}/` already exists in the current working directory. If it does, ask the user whether to overwrite.

## Step 3: Create Directory Structure

```bash
mkdir -p {PROJECT_NAME}/backend {PROJECT_NAME}/frontend
```

## Step 4: Generate Files

Read each template from `${CLAUDE_PLUGIN_ROOT}/skills/scaffold-service/templates/` and write to the target directory, replacing ALL placeholders:

- `__PROJECT_NAME__` → actual project name
- `__BACKEND_PORT__` → actual backend port

### 4.1 `.env`

Write directly (no template file needed):

```
SERVICE_NAME={PROJECT_NAME}
SERVICE_DOMAIN={PROJECT_NAME}.yeanhua.asia
SERVICE_UPSTREAM={PROJECT_NAME}-frontend:80
CADDY_ADMIN_URL=http://caddy-admin-api:8090
```

### 4.2 `register.sh`

Read from `${CLAUDE_PLUGIN_ROOT}/skills/scaffold-service/templates/register.sh`, replace `__PROJECT_NAME__`, write to `{PROJECT_NAME}/register.sh`.

### 4.3 `Makefile`

Copy from `${CLAUDE_PLUGIN_ROOT}/skills/scaffold-service/templates/Makefile` (no placeholders to replace).

### 4.4 `docker-compose.yml`

Generate based on language choice:

**For `go` or `node` (with backend):**

```yaml
services:

  {PROJECT_NAME}-backend:
    build:
      context: ./backend
    networks: [caddy-net]

  {PROJECT_NAME}-frontend:
    build:
      context: ./frontend
    depends_on:
      - {PROJECT_NAME}-backend
    networks: [caddy-net]

  {PROJECT_NAME}-register:
    image: curlimages/curl:latest
    depends_on:
      - {PROJECT_NAME}-frontend
    volumes:
      - ./register.sh:/register.sh:ro
    entrypoint: ["/bin/sh", "/register.sh"]
    env_file: .env
    networks: [caddy-net]
    restart: "no"

networks:
  caddy-net:
    external: true
```

**For `static` (no backend):**

```yaml
services:

  {PROJECT_NAME}-frontend:
    build:
      context: ./frontend
    networks: [caddy-net]

  {PROJECT_NAME}-register:
    image: curlimages/curl:latest
    depends_on:
      - {PROJECT_NAME}-frontend
    volumes:
      - ./register.sh:/register.sh:ro
    entrypoint: ["/bin/sh", "/register.sh"]
    env_file: .env
    networks: [caddy-net]
    restart: "no"

networks:
  caddy-net:
    external: true
```

### 4.5 Frontend Files

Read templates from `${CLAUDE_PLUGIN_ROOT}/skills/scaffold-service/templates/frontend/`:
- `index.html` → replace `__PROJECT_NAME__`
- `nginx.conf` → replace `__PROJECT_NAME__` and `__BACKEND_PORT__`
- `Dockerfile` → copy as-is

For `static` lang: modify `nginx.conf` to remove the `/api/` proxy_pass block (only serve static files).

### 4.6 Backend Files (skip if `--lang static`)

Read templates from `${CLAUDE_PLUGIN_ROOT}/skills/scaffold-service/templates/{LANG}/`:

**Go** (`go/`): `main.go`, `Dockerfile` → replace `__PROJECT_NAME__` and `__BACKEND_PORT__`

**Node** (`node/`): `index.js`, `package.json`, `Dockerfile` → replace `__PROJECT_NAME__` and `__BACKEND_PORT__`

## Step 5: Post-generation

1. Run `chmod +x {PROJECT_NAME}/register.sh`
2. Print a summary of all created files
3. Print next steps:

```
Next steps:
  1. cd {PROJECT_NAME}
  2. (Optional) Add {PROJECT_NAME}.yeanhua.asia to /etc/hosts for local dev
  3. docker compose up -d --build
  4. docker compose logs {PROJECT_NAME}-register   # confirm registration
  5. open https://{PROJECT_NAME}.yeanhua.asia
```

## Architecture Reference

How caddy-admin dynamic registration works:

1. `docker compose up` starts your service containers + a one-shot `register` sidecar
2. The sidecar waits for caddy-admin API readiness, then POSTs to `/api/services`
3. caddy-admin injects a reverse_proxy route into Caddy via its Admin API (`:2019`)
4. Caddy immediately starts routing `{domain}` → your `{upstream}` container
5. Registration is persisted to `services.json`; survives Caddy restarts

All containers communicate via Docker network `caddy-net`. Your containers do NOT expose ports to the host — Caddy is the single entry point (443/80).

### API Contract

**Register**: `POST http://caddy-admin-api:8090/api/services`
```json
{"name": "...", "domain": "....yeanhua.asia", "upstream": "container:port"}
```

**Deregister**: `DELETE http://caddy-admin-api:8090/api/services/{name}`

**List**: `GET http://caddy-admin-api:8090/api/services`
