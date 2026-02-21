# caddy-admin

Caddy 多项目管理平台：只读仪表盘 + **动态服务注册中心**。

- **仪表盘**：查询 Caddy Admin API，展示当前纳管的所有站点、路由配置和 TLS 证书状态
- **服务注册**：新项目启动时通过 `POST /api/services` 自注册路由到 Caddy，持久化到 `services.json`，Caddy 重启后自动恢复

同时作为 **"Go API + React 前端 + Caddy 单进程管理多项目"** 部署方案的可验证 Demo。`project-c` 是动态注册的端到端验证项目。

---

## 快速上手

### ECS 生产部署

```bash
# 1. 阿里云控制台添加 DNS A 记录（或泛域名 *.yeanhua.asia → ECS IP）
#    也可用 CLI：make add-dns RR=caddy-admin

# 2. 构建前端
make build-frontend

# 3. 签发 TLS 证书（首次，仅需一次）
#    使用 acme.sh DNS-01 challenge，详见下方"步骤 2"

# 4. 启动
make up
```

访问：`https://caddy-admin.yeanhua.asia`

---

### 本地开发测试

```bash
# 1. 构建前端
make build-frontend

# 2. 签发 TLS 证书（首次，仅需一次）
#    使用 acme.sh DNS-01 challenge，详见下方"步骤 2"

# 3. 启动所有容器
make up

# 4. 添加本地 DNS 映射（需要 sudo）
make hosts-add
# 写入 /etc/hosts：caddy-admin / site-a / site-b / project-c / project-d-stripe → 127.0.0.1
# 撤销：make hosts-remove
```

访问：`https://caddy-admin.yeanhua.asia`

---

## 推送到github

- /Users/yeanhua/workspace/playground/claude/github-assistant/README.md

``` log
gh repo create caddy-admin \
  --source=. \
  --public \
  --description "使用caddy管理多项目" \
  --push

```

## 新项目添加动态注册功能

- 使用 skill: plugin/README.md

## 容器关系

### 基础设施（caddy-admin docker-compose，4 个容器）

```
┌─────────────────────────────────────────────────────────────────┐
│ Docker 命名网络 caddy-net（外部项目可通过 external: true 加入）    │
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │ caddy（唯一入口）                                         │   │
│  │  端口：443 / 8180→80 / 2019(Admin API)                   │   │
│  │  职责：按域名路由、TLS 终结、服务静态文件                   │   │
│  │  *.yeanhua.asia 通配符 catch-all（动态路由 prepend 在其前） │   │
│  │                                                          │   │
│  │  caddy-admin.yeanhua.asia/api/*  ────────────────────►   │──►  caddy-admin-api:8090
│  │  caddy-admin.yeanhua.asia/*  → ./frontend/dist           │   │  （Go 后端 + 服务注册中心）
│  │  site-a.yeanhua.asia/api/*   ────────────────────────►   │──►  site-a-api:8081
│  │  site-b.yeanhua.asia/api/*   ────────────────────────►   │──►  site-b-api:8082
│  │  project-c.yeanhua.asia/*    ────────────────────────►   │──►  project-c-frontend:80（动态注册）
│  └──────────────────────────────────────────────────────────┘   │
│                                                                 │
│  caddy-admin-api                                                │
│    职责：① 查询 caddy:2019 → 解析站点和证书信息（只读）          │
│         ② 服务注册 API → 动态写入 Caddy 路由 + 持久化            │
│    数据：services_data volume → /app/data/services.json          │
│                                                                 │
│  site-a-api / site-b-api                                        │
│    职责：Caddyfile 静态路由的模拟后端                             │
└─────────────────────────────────────────────────────────────────┘
```

### 外部项目（独立 docker-compose，通过注册 API 接入）

```
┌─────────────────────────────────────────────────────────────────┐
│ project-c（独立 docker-compose，加入 caddy-net）                 │
│                                                                 │
│  project-c-backend  (:8080)    ← Go API                         │
│  project-c-frontend (:80)     ← nginx: HTML + 反代 /api/*       │
│  project-c-register (sidecar)  ← curl POST → caddy-admin-api    │
│                                  注册完成后退出                   │
└─────────────────────────────────────────────────────────────────┘
```

**请求流：**
```
Browser → Caddy(:443, TLS) → project-c-frontend(nginx:80) → /api/* → project-c-backend(:8080)
                                                           → /*    → static HTML
```

### 两种路由方式对比

| | Caddyfile 静态路由 | 动态注册路由 |
|---|---|---|
| 适用场景 | 基础设施自身的站点（caddy-admin, site-a, site-b） | 外部项目（project-c, 以及未来新增的项目） |
| 配置方式 | 修改 Caddyfile → reload Caddy | `POST /api/services` → 即时生效 |
| 重启恢复 | Caddy 自动从 Caddyfile 加载 | caddy-admin-api 启动时从 services.json sync |
| 新项目是否需要改基础设施 | 需要改 Caddyfile + docker-compose | **不需要**——新项目自注册 |

### 容器说明

| 容器 | 角色 | 对应生产场景 |
|------|------|------------|
| `caddy` | 统一入口，管理所有域名和 TLS | ECS 上的 Caddy 系统服务 |
| `caddy-admin-api` | 仪表盘后端 + 服务注册中心 | `/opt/caddy-admin/` 下的 docker compose |
| `site-a-api` | 模拟项目 A 的后端（静态路由） | Caddyfile 直接配置的项目 |
| `site-b-api` | 模拟项目 B 的后端（静态路由） | Caddyfile 直接配置的项目 |

**关键设计：** Caddy 是唯一监听外部端口（443/80）的容器。其余容器只在 `caddy-net` 内部网络监听，外部无法直接访问——所有流量必须经过 Caddy。

---

## 各容器接口 & 流量路径

### caddy（唯一对外暴露端口的容器）

对外端口：`443` / `80` / `2019`（Admin API，调试用）

Caddy 本身不只是反代，**也直接提供前端页面**——静态文件服务是 Caddy 的内建功能，不需要额外的 Nginx 或 Node 容器。

| 请求 | Caddy 处理方式 | 流量终点 |
|------|--------------|---------|
| `https://caddy-admin.yeanhua.asia/` | 读 bind mount `./frontend/dist/index.html` | **Caddy 本身**（无转发） |
| `https://caddy-admin.yeanhua.asia/sites` | 读 `./frontend/dist/index.html`（SPA fallback）| **Caddy 本身** |
| `https://caddy-admin.yeanhua.asia/api/*` | reverse_proxy | `caddy-admin-api:8090` |
| `https://site-a.yeanhua.asia/` | 读 bind mount `./mock-sites/static/site-a/` | **Caddy 本身** |
| `https://api.site-a.yeanhua.asia/*` | reverse_proxy | `site-a-api:8081` |
| `https://site-b.yeanhua.asia/` | 读 bind mount `./mock-sites/static/site-b/` | **Caddy 本身** |
| `https://api.site-b.yeanhua.asia/*` | reverse_proxy | `site-b-api:8082` |
| `http://localhost:2019/config/` | Admin API（Caddy 内建）| **Caddy 本身** |

### caddy-admin-api（Go 后端 + 服务注册中心）

内网端口：`8090`（本地调试时额外暴露到 host）

**只读接口（仪表盘）：**

| 接口 | 说明 | 数据来源 |
|------|------|---------|
| `GET /api/status` | Caddy 是否在线 | 请求 `caddy:2019/config/` |
| `GET /api/sites` | 所有站点列表（域名/类型/upstream/CORS）| 解析 `caddy:2019/config/apps/http` |
| `GET /api/sites/{domain}` | 单站点详情 | 同上，过滤 |
| `GET /api/certs` | TLS 证书列表（颁发者/有效期）| 读 `caddy_data` volume 中的 `.crt` 文件 |

**写入接口（服务注册）：**

| 接口 | 说明 | 操作 |
|------|------|------|
| `POST /api/services` | 注册/更新服务 | Caddy upsert 路由 + 持久化到 services.json |
| `DELETE /api/services/{name}` | 注销服务 | Caddy 删除路由 + 从 services.json 移除 |
| `GET /api/services` | 列出已注册服务 | 读 services.json |
| `POST /api/services/sync` | 手动触发同步 | 遍历 services.json → Caddy upsert |

#### caddy:2019 是什么？

`caddy:2019` 是 **Caddy 内建的 Admin API**——Caddy 进程自己暴露的 HTTP 管理接口，与业务端口（80/443）完全无关。`caddy` 是 Docker 服务名，由 Docker 内部 DNS 解析到对应容器 IP。

三处配置共同让它可用：

**① Caddyfile 将 Admin API 绑定到 `0.0.0.0`**（默认只监听 localhost，跨容器不可达）
```
# caddy/Caddyfile
{
    admin 0.0.0.0:2019
}
```

**② docker-compose.yml 通过环境变量传入地址**
```yaml
caddy-admin-api:
  environment:
    CADDY_ADMIN_ADDR: caddy:2019
```

**③ Go 代码用它查询 Caddy 当前配置**
```go
// backend/caddy/client.go
GET http://caddy:2019/config/   → 返回 Caddy 当前加载的完整 JSON 配置
```

调用链：
```
/api/sites 请求
  → caddy-admin-api:8090
  → GET http://caddy:2019/config/apps/http
  → Caddy 返回所有 HTTP server 路由的 JSON
  → parser.go 提取域名、类型、upstream
  → 返回给前端
```

本地可直接验证原始数据：`curl http://localhost:2019/config/`（host 上的 2019 端口映射到容器内同名端口）。

### site-a-api / site-b-api（mock 后端）

内网端口：`8081` / `8082`，**无 host 端口映射**

| 接口 | 说明 |
|------|------|
| `GET /health` | `{"status":"ok","service":"site-a-api"}` |
| `GET /api/hello` | `{"message":"Hello from Site A API","port":"8081"}` |

### 前端页面由谁提供？

**Caddy，不是任何应用容器。**

`make build-frontend` 将 React 编译成静态 HTML/JS/CSS 输出到 `./frontend/dist/`，这个目录以 bind mount 方式挂入 caddy 容器。Caddy 的 `file_server` 指令直接读取并返回这些文件——和 Nginx 托管静态文件是同一个原理，只是 Caddy 内建了这个功能，不需要再起一个 Nginx 容器。

```
浏览器请求 https://caddy-admin.yeanhua.asia/
        ↓
  Caddy file_server 读 /var/www/caddy-admin/dist/index.html
  （容器内路径，对应 host 的 ./frontend/dist/index.html）
        ↓
  返回 HTML，浏览器加载 JS
        ↓
  JS 发起 fetch("/api/sites")
        ↓
  Caddy 匹配 @api 规则，转发到 caddy-admin-api:8090
        ↓
  caddy-admin-api 查询 caddy:2019，返回 JSON
```

没有 Node.js 运行时，没有 SSR，纯静态文件 + API 分离。

---

## 新增接口是否需要改配置？

### 情况一：caddy-admin-api 新增 `/api/*` 接口

**什么都不用改，直接生效。**

Caddyfile 的 matcher 是路径前缀，不是精确匹配：

```
@api path /api/*
handle @api {
    reverse_proxy caddy-admin-api:8090
}
```

新增 `GET /api/newroute` 后，Caddy 自动将该路径转发到后端，无需 reload、无需重启任何容器。

### 情况二：caddy-admin-api 新增非 `/api/` 路径（如 `/metrics`）

命中 `@static` 规则，被当作静态文件处理，返回 404。需要扩展 Caddyfile 的 matcher：

```
# 方式 A：枚举额外路径
@api path /api/* /metrics /healthz

# 方式 B：用 not 反转（更通用）
@api not path /assets/* /favicon.ico
```

改完执行 `docker exec caddy-admin-caddy-1 caddy reload --config /etc/caddy/Caddyfile`，**不需要重启任何应用容器**。

### 情况三：新增一个完全独立的后端项目

**推荐方式：动态注册（不需要改任何基础设施文件）**

新项目只需在 `docker-compose.yml` 中加一个 register sidecar，启动时自动注册到 Caddy：

```bash
# 1. 新项目目录下创建 .env
SERVICE_NAME=my-project
SERVICE_DOMAIN=my-project.yeanhua.asia
SERVICE_UPSTREAM=my-project-frontend:80
CADDY_ADMIN_URL=http://caddy-admin-api:8090

# 2. docker-compose.yml 加入 caddy-net 并包含 register sidecar
networks:
  caddy-net:
    external: true

# 3. 启动——register sidecar 会自动 POST /api/services 完成注册
docker compose up -d
```

完整示例参考 `demos/project-c/`（包含 backend、frontend、register.sh、docker-compose.yml）。

**备选方式：Caddyfile 静态路由（适合基础设施自身的站点）**

```bash
# 直接修改 Caddyfile 追加站点块，reload Caddy
docker exec caddy caddy reload --config /etc/caddy/Caddyfile
```

### 汇总

| 场景 | Caddyfile | 应用容器 | 需要改基础设施？ |
|------|-----------|---------|---------------|
| 新增 `/api/*` 接口 | 不用改 | 不用重启 | 否 |
| 新增非 `/api/` 接口 | 改 matcher → reload | 不用重启 | 是 |
| 新增独立项目（动态注册） | 不用改 | 新项目 `docker compose up` | **否** |
| 新增独立项目（静态路由） | 追加站点块 → reload | 新项目 `docker compose up` | 是 |

## 架构（生产）

```
Browser → *.yeanhua.asia (HTTPS 443, 通配符证书)
             ↓
         Caddy (单进程，TLS 终结)
           ├── caddy-admin.yeanhua.asia/api/* → caddy-admin-api:8090    [Caddyfile 静态]
           ├── caddy-admin.yeanhua.asia/*     → /var/www/caddy-admin/dist
           ├── site-a.yeanhua.asia/*          → site-a-api:8081         [Caddyfile 静态]
           ├── project-c.yeanhua.asia/*       → project-c-frontend:80   [动态注册]
           ├── new-project.yeanhua.asia/*     → ...                     [动态注册]
           └── *.yeanhua.asia                 → 404 catch-all
             ↓
         caddy-admin-api (Go)
           ├── 只读：查询 Caddy Admin API + 读证书文件
           └── 写入：服务注册 API → Caddy 动态路由 + services.json 持久化
```

新项目接入不再需要改 Caddyfile——启动时自动注册，基础设施零改动。

---

## 本地运行（Docker 模拟）

### 前提

- Docker + Docker Compose
- Node.js 20+（构建前端）
- 浏览器安装 [Caddy Local Certificates](https://github.com/nicowillis/caddy-certificate-installer) 或手动信任本地 CA

### 步骤

**步骤 1：构建前端**
```bash
make build-frontend
```
在本机用 Node.js 执行 `vite build`，将 React 源码编译成静态文件输出到 `frontend/dist/`。
这一步在本机运行而非 Docker 内，是因为 `dist/` 目录随后会通过绑定挂载直接进入 Caddy 容器。

**步骤 2：签发 TLS 证书（首次，仅需一次）**

> 本地开发使用真实域名（`*.yeanhua.asia`）+ 443 端口，Let's Encrypt HTTP-01 challenge 依赖外部回调，
> 必须用 acme.sh DNS-01 challenge 提前签好证书再挂入容器。

这一步分 **签发** 和 **安装** 两个阶段：

| | `--issue`（签发） | `--install-cert`（安装） |
|---|---|---|
| 做什么 | 与 Let's Encrypt 交互，DNS 验证，获取证书 | 从 acme.sh 工作目录复制证书到你指定的路径 |
| 产物位置 | `~/.acme.sh/` 内部（混着配置和元数据，不适合直接引用） | `~/certs/yeanhua.asia/`（干净的 pem 文件，供 Caddy 挂载） |
| 需要网络 | 需要（阿里云 DNS API + CA） | 不需要 |
| 重复执行 | 幂等——证书已存在且未到期时自动跳过，不会重新签发（加 `--force` 才会） | 幂等——重复执行只是覆盖同名文件 |

分离的好处：acme.sh 内部目录结构可以随版本变化，而 Caddy 始终从固定路径 `~/certs/yeanhua.asia/` 读证书，互不耦合。`--cron` 续签后也会自动重新 install 到同一位置。

```bash
# 1. 签发：向 Let's Encrypt 申请通配符证书（DNS challenge，不需要开放任何端口）
#    产物存入 ~/.acme.sh/ 内部目录
docker run --rm -it \
  -v "$HOME/.acme.sh:/acme.sh" \
  -e Ali_Key="${Ali_Key}" \
  -e Ali_Secret="${Ali_Secret}" \
  neilpang/acme.sh \
  --issue -d "*.yeanhua.asia" -d yeanhua.asia \
  --dns dns_ali --server letsencrypt

# 2. 安装：将证书复制到统一目录（按域名区分），供 Caddy 挂载
mkdir -p ~/certs/yeanhua.asia
docker run --rm -it \
  -v "$HOME/.acme.sh:/acme.sh" \
  -v "$HOME/certs/yeanhua.asia:/certs" \
  neilpang/acme.sh \
  --install-cert -d "*.yeanhua.asia" \
  --fullchain-file /certs/fullchain.pem \
  --key-file       /certs/key.pem
```

证书从签发到被 Caddy 使用的完整链路：
```
acme.sh --issue
  → ~/.acme.sh/          Let's Encrypt 签发，存入 acme.sh 内部工作目录

acme.sh --install-cert
  → ~/certs/yeanhua.asia/fullchain.pem + key.pem    复制到统一证书目录

docker-compose volumes（~/certs/yeanhua.asia:/etc/caddy/certs:ro）
  → 容器内 /etc/caddy/certs/fullchain.pem + key.pem

Caddyfile（tls /etc/caddy/certs/fullchain.pem /etc/caddy/certs/key.pem）
  → Caddy 加载证书，服务 HTTPS
```

证书续签：Let's Encrypt 有效期 90 天。Caddy 启动后从 `caddy_data` volume 缓存读证书，**不会自动感知 pem 文件变化**，续签后必须 restart：

```bash
# 1. 续签（acme.sh 自动判断是否到期，未到期则跳过）
docker run --rm -it -v "$HOME/.acme.sh:/acme.sh" neilpang/acme.sh --cron

# 2. 重新 install-cert（覆盖 ~/certs/yeanhua.asia/ 下的 pem 文件）
docker run --rm -it \
  -v "$HOME/.acme.sh:/acme.sh" \
  -v "$HOME/certs/yeanhua.asia:/certs" \
  neilpang/acme.sh \
  --install-cert -d "*.yeanhua.asia" \
  --fullchain-file /certs/fullchain.pem \
  --key-file       /certs/key.pem

# 3. 只重启 Caddy，让它重新读取新证书（其他容器不受影响）
docker compose restart caddy
```

**步骤 3：启动所有服务**
```bash
make up
```
执行 `docker compose up -d --build`，依次发生：

1. **构建镜像**：`backend/` 用 Go 交叉编译出静态二进制；`mock-sites/site-a-api/` 和 `site-b-api/` 各自编译出轻量 Go HTTP server
2. **启动 site-a-api / site-b-api**：两个 mock 后端，分别监听容器内 8081 / 8082 端口，模拟真实项目的 API 服务
3. **启动 caddy-admin-api**：Go 后端，监听 8090，启动后等待被 Caddy 反代——此时还不能从外部直接访问（没有域名解析）
4. **启动 Caddy**：读取 `caddy/Caddyfile`，读取挂载的 TLS 证书（步骤 2 签发），
   开始监听 443 和 80，
   按 Host 头将请求路由到对应的静态目录或上游容器

完成后容器网络拓扑：
```
Host 443 → caddy容器:443
               ├── caddy-admin.yeanhua.asia  → /api/* → caddy-admin-api:8090 | 其余 → static dist/
               ├── site-a.yeanhua.asia       → /api/* → site-a-api:8081      | 其余 → static site-a/
               └── site-b.yeanhua.asia       → /api/* → site-b-api:8082      | 其余 → static site-b/
```
所有容器通过 Docker 内部网络 `caddy-net` 互通，Caddy 用服务名（`caddy-admin-api`、`site-a-api`）做 DNS 解析，无需 IP 地址。

**步骤 4：本地 DNS 解析（仅本地开发需要）**

```bash
sudo tee -a /etc/hosts <<EOF
127.0.0.1  caddy-admin.yeanhua.asia
127.0.0.1  site-a.yeanhua.asia
127.0.0.1  site-b.yeanhua.asia
127.0.0.1  project-c.yeanhua.asia
EOF
```

ECS 生产环境不需要这步——通过阿里云 DNS API 添加 A 记录：

```bash
# 前提：安装 aliyun CLI 并配置 AccessKey
# brew install aliyun-cli
# aliyun configure --mode AK
#   → Access Key Id:     输入 Ali_Key
#   → Access Key Secret: 输入 Ali_Secret
#   → Default Region Id: cn-hangzhou（或你的 ECS 所在区域）
#   → Default Language:  zh
# 配置保存在 ~/.aliyun/config.json，只需执行一次

ECS_IP=<your-ecs-ip>
# caddy-admin site-a site-b project-c
ECS_IP=121.41.107.93
for rr in project-d-stripe; do
  aliyun alidns AddDomainRecord \
    --DomainName yeanhua.asia \
    --RR "$rr" \
    --Type A \
    --Value "$ECS_IP"
done

# 验证解析是否生效
for rr in caddy-admin site-a site-b; do
  dig +short "$rr.yeanhua.asia"
done
```

<details>
<summary>curl 版本（无需安装 aliyun CLI）</summary>

阿里云 RPC API 需要签名，以下脚本封装了签名逻辑：

```bash
#!/usr/bin/env bash
# 用法: Ali_Key=xxx Ali_Secret=xxx bash add-dns.sh <ECS_IP>
set -euo pipefail

ECS_IP="${1:?用法: $0 <ECS_IP>}"
AK="${Ali_Key:?请设置 Ali_Key}"
SK="${Ali_Secret:?请设置 Ali_Secret}"
DOMAIN="yeanhua.asia"

# URL 编码（RFC 3986）
urlencode() {
  python3 -c "import urllib.parse; print(urllib.parse.quote('$1', safe=''))"
}

# 阿里云 RPC API 签名（v1 HMAC-SHA1）
signed_request() {
  local rr="$1"
  local nonce="$RANDOM$RANDOM"
  local timestamp
  timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

  # 公共参数 + 业务参数（按 key 字母序排列）
  local params=(
    "AccessKeyId=${AK}"
    "Action=AddDomainRecord"
    "DomainName=${DOMAIN}"
    "Format=JSON"
    "RR=${rr}"
    "SignatureMethod=HMAC-SHA1"
    "SignatureNonce=${nonce}"
    "SignatureVersion=1.0"
    "Timestamp=${timestamp}"
    "Type=A"
    "Value=${ECS_IP}"
    "Version=2015-01-09"
  )

  # 拼接并排序
  local sorted
  sorted=$(printf '%s\n' "${params[@]}" | sort)

  # 构造 canonicalized query string
  local query=""
  while IFS= read -r p; do
    local key="${p%%=*}" val="${p#*=}"
    query+="&$(urlencode "$key")=$(urlencode "$val")"
  done <<< "$sorted"
  query="${query:1}"  # 去掉开头的 &

  # StringToSign = GET&%2F&<url-encoded-query>
  local string_to_sign="GET&%2F&$(urlencode "$query")"

  # HMAC-SHA1 签名
  local signature
  signature=$(printf '%s' "$string_to_sign" \
    | openssl dgst -sha1 -hmac "${SK}&" -binary \
    | base64)

  # 发起请求
  curl -s "https://alidns.aliyuncs.com/?${query}&Signature=$(urlencode "$signature")"
}

for rr in caddy-admin site-a site-b; do
  echo "添加 ${rr}.${DOMAIN} → ${ECS_IP}"
  result=$(signed_request "$rr")
  echo "$result" | python3 -m json.tool 2>/dev/null || echo "$result"
done
```

</details>

**步骤 5：访问面板**
```bash
open https://caddy-admin.yeanhua.asia
# 或直接测试 API：
curl -s https://caddy-admin.yeanhua.asia/api/sites
```

### 验证接口（不走浏览器，直接打 API）

```bash
make test-caddy-api
# 等价于：
curl -s http://localhost:2019/config/apps/http/servers  # Caddy 原始 config
curl -s http://localhost:8090/api/sites                 # caddy-admin 解析结果
curl -s http://localhost:8090/api/certs                 # 证书列表
curl -s http://localhost:8090/api/services              # 已注册的动态服务
```
`localhost:2019` 是 Caddy Admin API，直接暴露到 host 方便调试；`localhost:8090` 是 caddy-admin 后端，同样直接暴露。在生产环境两者都不对外暴露——Caddy 只暴露 80/443，Admin API 仅 `localhost:2019` 监听。

---

## 部署到 ECS（yeanhua.asia 真实域名）

### 前提
- ECS 已完成初始化（Docker + Caddy + 安全组 22/80/443）
- 阿里云 DNS 已有 *.yeanhua.asia 泛域名，或手动添加 A 记录

### 部署

```bash
# 添加 DNS A 记录（推荐泛域名 *.yeanhua.asia → ECS IP）
# 这样动态注册的新子域名无需额外添加 DNS 记录

# 一键部署
ECS_IP=<your-ecs-ip> ./deploy/deploy.sh

# 验证
curl -sf https://caddy-admin.yeanhua.asia/api/sites
curl -sf https://caddy-admin.yeanhua.asia/api/services
```

---

## API

### 只读（仪表盘）

| 端点 | 说明 |
|------|------|
| `GET /api/status` | Caddy 是否在线 |
| `GET /api/sites` | 所有站点列表 |
| `GET /api/sites/{domain}` | 单站点详情 |
| `GET /api/certs` | TLS 证书列表 |

### 读写（服务注册）

| 端点 | 说明 | 请求体/参数 |
|------|------|------------|
| `GET /api/services` | 列出已注册服务 | - |
| `POST /api/services` | 注册/更新服务 | `{"name":"xxx","domain":"xxx.yeanhua.asia","upstream":"container:port"}` |
| `DELETE /api/services/{name}` | 注销服务 | URL 路径参数 `name` |
| `POST /api/services/sync` | 手动触发同步 | - |

---

## 目录结构

```
caddy-admin/                        # 基础设施
├── caddy-admin/
│   ├── backend/
│   │   ├── caddy/
│   │   │   ├── client.go           # Caddy Admin API 客户端（读 + 写）
│   │   │   ├── route_builder.go    # 动态路由 JSON 构建
│   │   │   ├── parser.go           # 配置解析
│   │   │   └── types.go            # Caddy 配置类型定义
│   │   ├── handlers/
│   │   │   ├── sites.go            # 站点查询（只读）
│   │   │   ├── certs.go            # 证书查询（只读）
│   │   │   ├── services.go         # 服务注册/注销/列表/同步
│   │   │   └── helpers.go          # JSON 响应工具函数
│   │   ├── store/
│   │   │   └── file_store.go       # services.json 持久化层
│   │   └── main.go                 # 入口 + syncToCaddy + CORS
│   └── frontend/                   # React + Vite + TypeScript
├── caddy/
│   └── Caddyfile                   # 静态路由 + *.yeanhua.asia 通配符 catch-all
├── site-a/ site-b/                 # 模拟项目（静态路由）
├── deploy/                         # 生产部署模板
├── docker-compose.yml
└── Makefile

.claude-plugin/                     # Claude Code Plugin Marketplace 声明
└── marketplace.json

plugin/                             # Claude Code Plugin（可分发）
├── .claude-plugin/
│   └── plugin.json                 # plugin 元信息
├── skills/
│   └── scaffold-service/           # /caddy-admin:scaffold-service
│       ├── SKILL.md                # skill 定义 + 执行指令
│       └── templates/              # 项目模板（go/node/frontend）
└── README.md

docs/                               # 文档
├── dynamic-service-quick-start-*.md  # 新项目接入 Quick Start
└── scaffold-service-skill-analysis-*.md  # Plugin 设计分析

demos/project-c/                    # 动态注册验证项目（独立 docker-compose）
├── backend/
│   ├── main.go                     # Go HTTP :8080
│   └── Dockerfile
├── frontend/
│   ├── index.html                  # 简单 HTML，fetch /api/hello
│   ├── nginx.conf                  # 反代 /api/* → backend
│   └── Dockerfile
├── register.sh                     # 等待 caddy-admin → curl POST /api/services
├── docker-compose.yml              # 3 services + external caddy-net
├── .env                            # SERVICE_NAME, DOMAIN, UPSTREAM, CADDY_ADMIN_URL
└── Makefile
```

---

## 可复用模板

### 方式一：Claude Code Plugin 一键脚手架（推荐）

安装 plugin 后，一条命令生成完整的可运行项目：

```bash
# 安装（一次性）
claude plugin marketplace add jssfy/caddy-admin
claude plugin install caddy-admin@jssfy-caddy-admin

# 生成项目（在任意目录下）
/caddy-admin:scaffold-service my-project              # Go 后端（默认）
/caddy-admin:scaffold-service my-project --lang node   # Node.js 后端
/caddy-admin:scaffold-service my-project --lang static  # 纯静态站

# 启动即自动注册
cd my-project && docker compose up -d --build
```

自动生成：`.env` / `register.sh` / `docker-compose.yml` / `Makefile` / `backend/` / `frontend/`，所有变量从项目名自动派生。详见 [`plugin/README.md`](plugin/README.md)。

### 方式二：手动复制模板

以 `demos/project-c/`（[GitHub](https://github.com/jssfy/project-c)）为模板，新项目只需 4 个文件即可接入：

```
my-project/
├── docker-compose.yml    # 加入 external caddy-net + register sidecar
├── register.sh           # 等待 caddy-admin → POST /api/services
├── .env                  # SERVICE_NAME / DOMAIN / UPSTREAM / CADDY_ADMIN_URL
└── ...（你的 backend/frontend）
```

```bash
# 启动即自动注册，无需改基础设施任何文件
cd my-project && docker compose up -d
```

详细步骤参考 [`docs/dynamic-service-quick-start-2026-02-21.md`](docs/dynamic-service-quick-start-2026-02-21.md)。

### 方式三：Caddyfile 静态路由

```bash
# 1. 在 Caddyfile 追加站点块
# 2. docker compose up -d（新项目）
# 3. docker exec caddy caddy reload --config /etc/caddy/Caddyfile
```

---

## 动态服务注册：操作说明

### 注册服务

```bash
curl -X POST http://localhost:8090/api/services \
  -H "Content-Type: application/json" \
  -d '{"name":"my-svc","domain":"my-svc.yeanhua.asia","upstream":"my-svc-frontend:80"}'
# → {"registered":true,"name":"my-svc","domain":"my-svc.yeanhua.asia","upstream":"my-svc-frontend:80"}
```

字段说明：

| 字段 | 必填 | 说明 | 示例 |
|------|------|------|------|
| `name` | 是 | 服务唯一标识（用于路由 @id 和注销） | `project-c` |
| `domain` | 是 | 域名（必须是 `*.yeanhua.asia` 子域名，通配符证书覆盖） | `project-c.yeanhua.asia` |
| `upstream` | 是 | Docker 内网地址（容器名:端口） | `project-c-frontend:80` |

### 注销服务

```bash
curl -X DELETE http://localhost:8090/api/services/my-svc
# → {"deleted":true,"name":"my-svc"}
```

### 查看已注册服务

```bash
curl http://localhost:8090/api/services
# → {"services":[...],"total":1}
```

### 手动触发同步（services.json → Caddy）

```bash
curl -X POST http://localhost:8090/api/services/sync
# → {"synced":1,"total":1,"errors":null}
```

### 验证路由已注入 Caddy

```bash
curl -s http://localhost:2019/config/apps/http/servers/srv0/routes \
  | python3 -c "import sys,json; routes=json.load(sys.stdin); [print(r.get('@id','')) for r in routes if r.get('@id','')]"
# → svc-project-c
```

### 启动时自动恢复

caddy-admin-api 启动时执行 `syncToCaddy()`：
1. 等待 Caddy Admin API 就绪（重试 15 次，每次 2 秒）
2. 读取 `services.json` 中所有持久化的服务
3. 逐个调用 `UpsertRoute()` 写入 Caddy

日志示例：
```
sync: waiting for caddy... (1/15)
sync: restored 1/1 services to caddy
```

---

## 测试流程

### 前提

- 基础设施已启动：`cd demos/caddy-admin && docker compose up -d --build`
- 本地 DNS 已配置：`/etc/hosts` 包含 `*.yeanhua.asia` 解析到 `127.0.0.1`

### 1. 基础设施健康检查

```bash
# Caddy 在线
curl -s http://localhost:8090/api/status                        # 直连后端
curl -s https://caddy-admin.yeanhua.asia/api/status             # 走域名
# → {"caddy":true}

# Caddy server 名确认为 srv0（仅 Admin API 端口，无域名路由）
curl -s http://localhost:2019/config/apps/http/servers | python3 -m json.tool | head -3
# → { "srv0": { ...

# 已注册服务列表（初始应为空）
curl -s http://localhost:8090/api/services                      # 直连后端
curl -s https://caddy-admin.yeanhua.asia/api/services           # 走域名
# → {"services":[],"total":0}

# caddy-admin-api 启动日志（确认 sync 成功）
docker logs caddy-admin-caddy-admin-api-1 --tail 5
```

### 2. 手动注册/注销测试

```bash
# 注册测试服务
curl -s -X POST http://localhost:8090/api/services \
  -H "Content-Type: application/json" \
  -d '{"name":"test","domain":"test.yeanhua.asia","upstream":"localhost:9999"}'
# 走域名：
curl -s -X POST https://caddy-admin.yeanhua.asia/api/services \
  -H "Content-Type: application/json" \
  -d '{"name":"test","domain":"test.yeanhua.asia","upstream":"localhost:9999"}'
# → {"registered":true,...}

# 验证路由已注入 Caddy（应出现 svc-test）
# ⚠ localhost:2019 是 Caddy Admin API 调试端口，无域名路由，只能 localhost
curl -s http://localhost:2019/config/apps/http/servers/srv0/routes \
  | python3 -c "import sys,json; [print(r['@id']) for r in json.load(sys.stdin) if '@id' in r]"
# → svc-test

# 验证持久化
curl -s http://localhost:8090/api/services                      # 直连后端
curl -s https://caddy-admin.yeanhua.asia/api/services           # 走域名
# → {"services":[{"name":"test",...}],"total":1}

# 注销
curl -s -X DELETE http://localhost:8090/api/services/test       # 直连后端
curl -s -X DELETE https://caddy-admin.yeanhua.asia/api/services/test  # 走域名
# → {"deleted":true,"name":"test"}

# 验证已从 Caddy 删除（无输出）
curl -s http://localhost:2019/config/apps/http/servers/srv0/routes \
  | python3 -c "import sys,json; [print(r['@id']) for r in json.load(sys.stdin) if '@id' in r]"
```

### 3. project-c 端到端测试

```bash
# 启动 project-c
cd demos/project-c && docker compose up -d --build

# 查看 register sidecar 日志（应显示注册成功）
docker compose logs project-c-register
# → "project-c registered successfully."

# 验证路由已注入 Caddy
curl -s http://localhost:2019/config/apps/http/servers/srv0/routes \
  | python3 -c "import sys,json; [print(r['@id']) for r in json.load(sys.stdin) if '@id' in r]"
# → svc-project-c

# 通过域名访问 project-c
# 方式 A：浏览器（推荐）
#   https://project-c.yeanhua.asia/
#   点击按钮 → 调用 /api/hello → 显示 JSON 结果

# 方式 B：openssl（终端验证，macOS curl 有 TLS 兼容性问题）
echo -e "GET / HTTP/1.1\r\nHost: project-c.yeanhua.asia\r\nConnection: close\r\n\r\n" \
  | openssl s_client -connect localhost:443 -servername project-c.yeanhua.asia -quiet 2>/dev/null
# → 200 OK，返回 HTML

echo -e "GET /api/hello HTTP/1.1\r\nHost: project-c.yeanhua.asia\r\nConnection: close\r\n\r\n" \
  | openssl s_client -connect localhost:443 -servername project-c.yeanhua.asia -quiet 2>/dev/null
# → {"message":"Hello from Project C API","port":"8080"}
```

> **macOS curl TLS 注意事项：** macOS 自带的 `curl`（LibreSSL 3.3.6）与 Caddy 的 ECDSA 证书可能存在 TLS 握手兼容性问题（exit code 35）。这不影响浏览器和 `openssl` 访问。如需 curl 测试，可用 `brew install curl` 安装 OpenSSL 版本。

### 4. 持久化恢复测试（Caddy 重启）

```bash
# 重启 Caddy + caddy-admin-api
cd demos/caddy-admin
docker compose restart caddy caddy-admin-api

# 等待 sync 完成（约 10 秒）
sleep 10

# 检查日志确认恢复
docker logs caddy-admin-caddy-admin-api-1 --tail 3
# → "sync: restored 1/1 services to caddy"

# 验证路由仍在
curl -s http://localhost:2019/config/apps/http/servers/srv0/routes \
  | python3 -c "import sys,json; [print(r['@id']) for r in json.load(sys.stdin) if '@id' in r]"
# → svc-project-c

# 验证 project-c 仍可访问
# 浏览器：https://project-c.yeanhua.asia/api/hello
# 终端：
echo -e "GET /api/hello HTTP/1.1\r\nHost: project-c.yeanhua.asia\r\nConnection: close\r\n\r\n" \
  | openssl s_client -connect localhost:443 -servername project-c.yeanhua.asia -quiet 2>/dev/null
# → {"message":"Hello from Project C API","port":"8080"}

# 验证 services API 仍可用
curl -s https://caddy-admin.yeanhua.asia/api/services           # 走域名
# → {"services":[{"name":"project-c",...}],"total":1}
```

### 5. 清理

```bash
# 注销 project-c
curl -X DELETE http://localhost:8090/api/services/project-c             # 直连后端
# 或走域名：
curl -X DELETE https://caddy-admin.yeanhua.asia/api/services/project-c  # 走域名

# 停止 project-c 容器
cd demos/project-c && docker compose down

# （可选）停止基础设施
cd demos/caddy-admin && docker compose down
```

### 域名 vs localhost 速查

| 接口 | localhost（直连后端） | 域名（走 Caddy 反代） |
|------|---------------------|---------------------|
| caddy-admin API | `http://localhost:8090/api/*` | `https://caddy-admin.yeanhua.asia/api/*` |
| Caddy Admin API | `http://localhost:2019/*` | **无域名**（调试端口，仅 localhost） |
| project-c 页面 | — | `https://project-c.yeanhua.asia/` |
| project-c API | — | `https://project-c.yeanhua.asia/api/hello` |

### 测试检查清单

| # | 测试项 | 预期结果 | 直连命令 | 域名命令 |
|---|--------|---------|---------|---------|
| 1 | caddy-admin-api 启动 | sync 日志正常 | `docker logs ... --tail 5` | — |
| 2 | POST 注册 | `{"registered":true}` | `curl -X POST localhost:8090/...` | `curl -X POST https://caddy-admin.yeanhua.asia/...` |
| 3 | Caddy 路由注入 | 出现 `svc-{name}` | `curl localhost:2019/...` | **仅 localhost** |
| 4 | 持久化写入 | services 列表非空 | `curl localhost:8090/api/services` | `curl https://caddy-admin.yeanhua.asia/api/services` |
| 5 | DELETE 注销 | `{"deleted":true}` | `curl -X DELETE localhost:8090/...` | `curl -X DELETE https://caddy-admin.yeanhua.asia/...` |
| 6 | sidecar 注册 | "registered successfully" | `docker compose logs ...` | — |
| 7 | 端到端 HTML | 200 OK | `openssl s_client ...` | 浏览器 `https://project-c.yeanhua.asia/` |
| 8 | 端到端 API | `{"message":"Hello..."}` | `openssl s_client ...` | 浏览器 `https://project-c.yeanhua.asia/api/hello` |
| 9 | 重启后恢复 | "sync: restored N/N" | `docker logs ...` | — |
| 10 | 重启后路由在 | `svc-project-c` 仍在 | `curl localhost:2019/...` | **仅 localhost** |
