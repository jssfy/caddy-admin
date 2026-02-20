# 动态服务接入 Quick Start

## 核心结论

- 新项目接入 **不需要修改任何基础设施文件**（Caddyfile、caddy-admin 的 docker-compose 等）
- 只需 3 个接入文件：`.env` + `register.sh` + `docker-compose.yml`（加入 `caddy-net` 网络 + register sidecar）
- 启动即自注册，Caddy 路由即时生效，HTTPS 自动覆盖（泛域名证书 `*.yeanhua.asia`）
- Demo 项目：[project-c](https://github.com/jssfy/project-c) — 完整的端到端验证示例

---

## 工作原理

### 整体架构

```
                    ┌───────────────────────────────────────┐
                    │         Caddy（唯一外部入口）           │
                    │         443 (HTTPS) / 80 (HTTP)       │
                    │                                       │
  Browser ────────► │  *.yeanhua.asia 泛域名 TLS 终结       │
                    │                                       │
                    │  动态路由（API 注入，优先级高）：         │
                    │    project-c.yeanhua.asia → frontend:80│
                    │    my-project.yeanhua.asia → app:3000  │
                    │                                       │
                    │  静态路由（Caddyfile）：                 │
                    │    caddy-admin.yeanhua.asia → api:8090 │
                    │    site-a.yeanhua.asia → api:8081      │
                    │                                       │
                    │  *.yeanhua.asia → 404 catch-all       │
                    └───────────┬───────────────────────────┘
                                │
                         Docker caddy-net
                    （所有容器通过服务名互通）
```

### 注册流程（一次性）

```
docker compose up -d
      │
      ▼
┌─────────────────────────┐
│ register sidecar 启动    │
│ (curlimages/curl)       │
│                         │
│ 1. 轮询等待 caddy-admin │  GET /api/status → 200?
│    API 就绪（最多 60s） │
│                         │
│ 2. POST /api/services   │──────► caddy-admin-api:8090
│    {name, domain,       │            │
│     upstream}           │            ▼
│                         │     ┌──────────────────┐
│ 3. 收到 201 → 退出      │     │ ① UpsertRoute()  │
│    (restart: "no")      │     │   PUT caddy:2019  │ ← 注入 Caddy JSON 路由
└─────────────────────────┘     │                  │
                                │ ② fileStore.Upsert│ ← 持久化到 services.json
                                └──────────────────┘
```

### Caddy 动态路由注入细节

注册 API 收到请求后，调用 Caddy Admin API（`:2019`）将路由以 JSON 格式写入运行中的 Caddy 实例：

```json
{
  "@id": "svc-project-c",
  "match": [{"host": ["project-c.yeanhua.asia"]}],
  "handle": [{
    "handler": "subroute",
    "routes": [{
      "handle": [{
        "handler": "reverse_proxy",
        "upstreams": [{"dial": "project-c-frontend:80"}]
      }]
    }]
  }],
  "terminal": true
}
```

- **`@id: "svc-{name}"`** — 唯一标识，用于后续更新/删除
- **`match.host`** — 按域名匹配请求
- **`reverse_proxy.upstreams.dial`** — 转发到 Docker 内网容器（服务名:端口）
- 路由 **prepend** 到 Caddy 路由表头部，优先级高于 Caddyfile 静态路由和 `*.yeanhua.asia` catch-all

### 持久化与重启恢复

- 注册信息写入 `services.json`（Docker volume 持久化）
- caddy-admin-api 启动时自动执行 `syncToCaddy()`：读取 `services.json`，逐条重新注入 Caddy
- Caddy/基础设施重启后，动态路由自动恢复，无需重新注册

---

## 接入步骤

### 前提条件

| 条件 | 说明 |
|------|------|
| caddy-admin 基础设施已运行 | `cd demos/caddy-admin && docker compose up -d` |
| Docker 网络 `caddy-net` 已存在 | 基础设施启动时自动创建 |
| DNS 泛域名已配置 | `*.yeanhua.asia` → ECS IP（或本地 `/etc/hosts`） |

### Step 1：创建项目目录结构

```
my-project/
├── .env                    # 服务注册配置（4 个变量）
├── register.sh             # 注册脚本（可直接复制）
├── docker-compose.yml      # 加入 caddy-net + register sidecar
├── backend/                # 你的后端（任意语言/框架）
│   ├── main.go (或 app.py, index.js ...)
│   └── Dockerfile
└── frontend/               # 你的前端（可选）
    ├── index.html (或 React/Vue build)
    ├── nginx.conf           # 反代 /api/* 到后端
    └── Dockerfile
```

### Step 2：配置 `.env`

```bash
# 服务唯一标识（英文，用于 Caddy 路由 @id 和管理面板显示）
SERVICE_NAME=my-project

# 域名（必须是 *.yeanhua.asia 子域名，泛域名证书自动覆盖）
SERVICE_DOMAIN=my-project.yeanhua.asia

# Docker 内网上游地址（容器服务名:端口）
# Caddy 通过 Docker DNS 解析此地址，所以必须是 docker-compose 中定义的 service 名
SERVICE_UPSTREAM=my-project-frontend:80

# caddy-admin 注册 API 地址（固定值，不需要修改）
CADDY_ADMIN_URL=http://caddy-admin-api:8090
```

**`SERVICE_UPSTREAM` 指向哪个容器？**

取决于你的项目架构：

| 架构 | `SERVICE_UPSTREAM` | 说明 |
|------|-------------------|------|
| 前后端分离（nginx 反代） | `my-project-frontend:80` | nginx 统一入口，`/api/*` 反代到后端 |
| 纯后端 API | `my-project-backend:8080` | Caddy 直接代理到后端 |
| 纯静态站 | `my-project-web:80` | nginx/caddy 托管静态文件 |
| Node.js 全栈 | `my-project-app:3000` | Express/Next.js 直接监听 |

### Step 3：复制 `register.sh`

直接从 [project-c/register.sh](https://github.com/jssfy/project-c) 复制，**无需修改**——所有参数通过 `.env` 环境变量传入：

```bash
#!/bin/sh
set -e

CADDY_ADMIN_URL="${CADDY_ADMIN_URL:-http://caddy-admin-api:8090}"
SERVICE_NAME="${SERVICE_NAME:-my-project}"
SERVICE_DOMAIN="${SERVICE_DOMAIN:-my-project.yeanhua.asia}"
SERVICE_UPSTREAM="${SERVICE_UPSTREAM:-my-project-frontend:80}"

echo "Waiting for caddy-admin API at ${CADDY_ADMIN_URL}..."

RETRIES=30
for i in $(seq 1 $RETRIES); do
  if curl -sf "${CADDY_ADMIN_URL}/api/status" > /dev/null 2>&1; then
    echo "caddy-admin API is ready."
    break
  fi
  if [ "$i" -eq "$RETRIES" ]; then
    echo "ERROR: caddy-admin API not ready after ${RETRIES} attempts."
    exit 1
  fi
  echo "  attempt $i/${RETRIES}..."
  sleep 2
done

echo "Registering service: ${SERVICE_NAME} -> ${SERVICE_DOMAIN} -> ${SERVICE_UPSTREAM}"

RESPONSE=$(curl -sf -X POST "${CADDY_ADMIN_URL}/api/services" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"${SERVICE_NAME}\",\"domain\":\"${SERVICE_DOMAIN}\",\"upstream\":\"${SERVICE_UPSTREAM}\"}")

echo "Response: ${RESPONSE}"
echo "${SERVICE_NAME} registered successfully."
```

### Step 4：编写 `docker-compose.yml`

关键点用注释标注：

```yaml
services:

  my-project-backend:
    build:
      context: ./backend
    networks: [caddy-net]        # ← 必须加入 caddy-net

  my-project-frontend:
    build:
      context: ./frontend
    depends_on:
      - my-project-backend
    networks: [caddy-net]        # ← 必须加入 caddy-net

  # ── register sidecar（一次性容器）──
  my-project-register:
    image: curlimages/curl:latest
    depends_on:
      - my-project-frontend      # ← 确保业务容器先启动
    volumes:
      - ./register.sh:/register.sh:ro
    entrypoint: ["/bin/sh", "/register.sh"]
    env_file: .env
    networks: [caddy-net]        # ← 必须加入 caddy-net
    restart: "no"                # ← 注册完成后不重启

networks:
  caddy-net:
    external: true               # ← 使用基础设施创建的外部网络
```

### Step 5：配置 DNS（如果是新子域名）

**本地开发**：在 `/etc/hosts` 添加：
```
127.0.0.1  my-project.yeanhua.asia
```

**生产环境（ECS）**：如果已配置 `*.yeanhua.asia` 泛域名 A 记录，则**无需额外操作**。

### Step 6：启动

```bash
cd my-project
docker compose up -d --build
```

查看注册日志确认成功：
```bash
docker compose logs my-project-register
# → "caddy-admin API is ready."
# → "Registering service: my-project -> my-project.yeanhua.asia -> my-project-frontend:80"
# → "Response: {"registered":true,...}"
# → "my-project registered successfully."
```

访问验证：
```bash
# 浏览器
open https://my-project.yeanhua.asia

# 或终端（macOS 用 openssl 避免 TLS 兼容性问题）
echo -e "GET / HTTP/1.1\r\nHost: my-project.yeanhua.asia\r\nConnection: close\r\n\r\n" \
  | openssl s_client -connect localhost:443 -servername my-project.yeanhua.asia -quiet 2>/dev/null
```

---

## 注意事项

### 命名规范

| 字段 | 规则 | 示例 |
|------|------|------|
| `SERVICE_NAME` | 英文小写 + 连字符，全局唯一 | `my-project`、`user-service` |
| `SERVICE_DOMAIN` | 必须是 `*.yeanhua.asia` 子域名 | `my-project.yeanhua.asia` |
| `SERVICE_UPSTREAM` | Docker compose 中定义的 service 名 + 端口 | `my-project-frontend:80` |
| 容器服务名 | 建议以项目名为前缀，避免跨项目冲突 | `my-project-backend`、`my-project-frontend` |

### 网络

- 所有需要被 Caddy 代理的容器**必须**加入 `caddy-net` 网络
- `caddy-net` 是 `external: true`，由 caddy-admin 基础设施创建，你的项目只是加入
- 容器间通过 Docker DNS 用服务名通信（如 `my-project-backend:8080`），不要用 IP

### 幂等性

- `POST /api/services` 是**幂等的**（upsert 语义）：相同 `name` 重复注册会更新而非报错
- 项目重启时 register sidecar 会重新注册，不会产生重复路由
- `DELETE /api/services/{name}` 也是幂等的：已不存在的服务删除不会报错

### 端口

- 你的容器**不需要**暴露端口到 host（`ports` 映射）
- 所有外部流量通过 Caddy 443 端口进入，Caddy 在 `caddy-net` 内部代理到你的容器
- 只有调试需要时才映射端口到 host

### 前端 nginx 反代模式（推荐）

如果项目是前后端分离，推荐用 nginx 作为项目内部统一入口：

```nginx
# frontend/nginx.conf
server {
    listen 80;
    server_name _;

    root /usr/share/nginx/html;
    index index.html;

    # API 请求反代到后端容器
    location /api/ {
        proxy_pass http://my-project-backend:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # 前端 SPA fallback
    location / {
        try_files $uri $uri/ /index.html;
    }
}
```

请求链路：
```
Browser → Caddy(:443) → nginx(:80) → /api/* → backend(:8080)
                                    → /*    → static HTML/JS
```

### 常见问题

| 问题 | 原因 | 解决 |
|------|------|------|
| register 容器报 "caddy-admin API not ready" | 基础设施未启动 | 先 `cd demos/caddy-admin && docker compose up -d` |
| 域名访问 404 | DNS 未配置 | 检查 `/etc/hosts` 或云 DNS 解析 |
| 域名访问 502 Bad Gateway | upstream 容器未运行或服务名错误 | `docker ps` 检查容器状态；确认 `SERVICE_UPSTREAM` 与 docker-compose service 名一致 |
| 重复注册后旧路由残留 | 不会发生 | UpsertRoute 先删后加，原子替换 |
| Caddy/基础设施重启后路由丢失 | 不会发生 | caddy-admin-api 启动时自动从 services.json 恢复 |

---

## 管理操作

### 在管理面板查看/删除

访问 `https://caddy-admin.yeanhua.asia/services`（Services tab），可查看所有已注册的动态服务并通过 Delete 按钮注销。

### 命令行操作

```bash
# 查看所有已注册服务
curl -s https://caddy-admin.yeanhua.asia/api/services

# 手动注册（不用 sidecar）
curl -X POST https://caddy-admin.yeanhua.asia/api/services \
  -H "Content-Type: application/json" \
  -d '{"name":"my-svc","domain":"my-svc.yeanhua.asia","upstream":"my-svc-app:3000"}'

# 注销服务
curl -X DELETE https://caddy-admin.yeanhua.asia/api/services/my-svc

# 手动触发同步（services.json → Caddy）
curl -X POST https://caddy-admin.yeanhua.asia/api/services/sync
```

### 完整清理

```bash
# 1. 注销服务路由
curl -X DELETE https://caddy-admin.yeanhua.asia/api/services/my-project

# 2. 停止项目容器
cd my-project && docker compose down
```

---

## Demo 项目

完整的端到端示例：**[project-c](https://github.com/jssfy/project-c)**

```
project-c/
├── .env                  # SERVICE_NAME=project-c, DOMAIN=project-c.yeanhua.asia
├── register.sh           # 等待 caddy-admin → POST /api/services
├── docker-compose.yml    # 3 services: backend + frontend(nginx) + register(curl sidecar)
├── Makefile              # up / down / logs
├── backend/
│   ├── main.go           # Go API: /health + /api/hello
│   └── Dockerfile        # 多阶段构建，最终 alpine 镜像
└── frontend/
    ├── index.html        # 简单页面 + fetch /api/hello 按钮
    ├── nginx.conf        # /api/* 反代到 backend:8080
    └── Dockerfile        # nginx:alpine
```

启动验证：
```bash
cd demos/project-c && make up
docker compose logs project-c-register   # 确认 "registered successfully"
open https://project-c.yeanhua.asia      # 浏览器访问
```
