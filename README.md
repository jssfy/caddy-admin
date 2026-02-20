# caddy-admin

一个只读信息面板，查询 Caddy Admin API，展示当前纳管的所有站点、路由配置和 TLS 证书状态。

同时作为 **"Go API + React 前端 + Caddy 单进程管理多项目"** 部署方案的可验证 Demo。

---

## 推送到github

``` log
gh repo create caddy-admin \
  --source=. \
  --public \
  --description "使用caddy管理多项目" \
  --push

```

## 容器关系（本地 4 个容器）

```
┌─────────────────────────────────────────────────────────────────┐
│ Docker 内部网络 caddy-net                                        │
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │ caddy（唯一入口）                                         │   │
│  │  端口：8443→443 / 8180→80 / 2019(Admin API)              │   │
│  │  职责：按域名路由、自动签 SSL 证书、服务静态文件            │   │
│  │                                                          │   │
│  │  caddy-admin.localhost/api/* ──────────────────────────► │──►  caddy-admin-api:8090
│  │  caddy-admin.localhost/*    → ./frontend/dist（bind mount）│   │  （本项目的 Go 后端）
│  │  site-a.localhost/*         → ./mock-sites/static/site-a │   │
│  │  api.site-a.localhost/*  ──────────────────────────────► │──►  site-a-api:8081
│  │  site-b.localhost/*         → ./mock-sites/static/site-b │   │  （模拟项目 A 的 API）
│  │  api.site-b.localhost/*  ──────────────────────────────► │──►  site-b-api:8082
│  └──────────────────────────────────────────────────────────┘   │  （模拟项目 B 的 API）
│                                                                 │
│  caddy-admin-api                                                │
│    职责：查询 caddy:2019/config/ → 解析站点和证书信息            │
│    访问证书文件：caddy_data volume（Caddy 写入的证书目录）        │
│                                                                 │
│  site-a-api / site-b-api                                        │
│    职责：模拟两个真实项目的后端 API（验证 Caddy 多项目反代）      │
└─────────────────────────────────────────────────────────────────┘
```

**为什么需要 4 个容器：**

| 容器 | 角色 | 对应生产场景 |
|------|------|------------|
| `caddy` | 统一入口，管理所有域名和 SSL | ECS 上的 Caddy 系统服务 |
| `caddy-admin-api` | 本项目的 Go 后端 | `/opt/caddy-admin/` 下的 docker compose |
| `site-a-api` | 模拟项目 A 的后端 | 另一个项目的 docker compose |
| `site-b-api` | 模拟项目 B 的后端 | 另一个项目的 docker compose |

**关键设计：** Caddy 是唯一监听外部端口（443/80）的容器。其余三个只在内部网络监听，外部无法直接访问——所有流量必须经过 Caddy。这正是生产环境的样子：每个项目的 API 只暴露内网端口（8090/8081/8082），Caddy 按域名反代。

---

## 各容器接口 & 流量路径

### caddy（唯一对外暴露端口的容器）

对外端口：`8443`→443 / `8180`→80 / `2019`（Admin API，调试用）

Caddy 本身不只是反代，**也直接提供前端页面**——静态文件服务是 Caddy 的内建功能，不需要额外的 Nginx 或 Node 容器。

| 请求 | Caddy 处理方式 | 流量终点 |
|------|--------------|---------|
| `https://caddy-admin.localhost:8443/` | 读 bind mount `./frontend/dist/index.html` | **Caddy 本身**（无转发） |
| `https://caddy-admin.localhost:8443/sites` | 读 `./frontend/dist/index.html`（SPA fallback）| **Caddy 本身** |
| `https://caddy-admin.localhost:8443/api/*` | reverse_proxy | `caddy-admin-api:8090` |
| `https://site-a.localhost:8443/` | 读 bind mount `./mock-sites/static/site-a/` | **Caddy 本身** |
| `https://api.site-a.localhost:8443/*` | reverse_proxy | `site-a-api:8081` |
| `https://site-b.localhost:8443/` | 读 bind mount `./mock-sites/static/site-b/` | **Caddy 本身** |
| `https://api.site-b.localhost:8443/*` | reverse_proxy | `site-b-api:8082` |
| `http://localhost:2019/config/` | Admin API（Caddy 内建）| **Caddy 本身** |

### caddy-admin-api（Go 后端）

内网端口：`8090`（本地调试时额外暴露到 host）

| 接口 | 说明 | 数据来源 |
|------|------|---------|
| `GET /api/status` | Caddy 是否在线 | 请求 `caddy:2019/config/` |
| `GET /api/sites` | 所有站点列表（域名/类型/upstream/CORS）| 解析 `caddy:2019/config/apps/http` |
| `GET /api/sites/{domain}` | 单站点详情 | 同上，过滤 |
| `GET /api/certs` | TLS 证书列表（颁发者/有效期）| 读 `caddy_data` volume 中的 `.crt` 文件 |

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
浏览器请求 https://caddy-admin.localhost:8443/
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

需要两步，互相独立：

```bash
# 1. 新项目自己启动（与 caddy-admin 无关）
cd /opt/new-project && docker compose up -d

# 2. Caddyfile 追加站点块，reload（不重启现有服务）
cat >> /etc/caddy/Caddyfile << 'EOF'

new-project.yeanhua.asia {
    @api path /api/*
    @static not path /api/*
    handle @api { reverse_proxy localhost:8091 }
    handle @static {
        root * /var/www/new-project/dist
        file_server
        try_files {path} /index.html
    }
}
EOF
systemctl reload caddy   # 自动为新域名申请 SSL，已有站点不受影响
```

### 汇总

| 场景 | Caddyfile | 应用容器 |
|------|-----------|---------|
| 新增 `/api/*` 接口 | 不用改 | 不用重启 |
| 新增非 `/api/` 接口 | 改 matcher → reload | 不用重启 |
| 新增独立项目 | 追加站点块 → reload | 新项目 `docker compose up` |

## 架构（生产）

```
Browser → caddy-admin.yeanhua.asia (HTTPS 443)
             ↓
         Caddy (单进程，自动 SSL)
           ├── caddy-admin.yeanhua.asia/api/* → localhost:8090 (caddy-admin Go API)
           ├── caddy-admin.yeanhua.asia/*     → /var/www/caddy-admin/dist (React 静态)
           ├── project-b.yeanhua.asia/api/*   → localhost:8091 (project-b API)
           └── ...（每新增项目追加两段 Caddyfile，reload 即可）
             ↓
         caddy-admin-api (Go)
           └── 查询 Caddy Admin API (localhost:2019) + 读证书文件
```

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

> Caddy 运行在非标准端口 8443，Let's Encrypt HTTP-01 challenge 无法回调，
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
   开始监听 8180（→容器内 80）和 8443（→容器内 443），
   按 Host 头将请求路由到对应的静态目录或上游容器

完成后容器网络拓扑：
```
Host 8443 → caddy容器:443
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
ECS_IP=121.41.107.93
for rr in caddy-admin site-a site-b; do
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
open https://caddy-admin.yeanhua.asia:8443   # 本地（非标端口）
open https://caddy-admin.yeanhua.asia        # ECS 生产（标准 443）
# 或直接测试 API：
curl -s https://caddy-admin.yeanhua.asia:8443/api/sites
```

### 验证接口（不走浏览器，直接打 API）

```bash
make test-caddy-api
# 等价于：
curl -s http://localhost:2019/config/apps/http/servers  # Caddy 原始 config
curl -s http://localhost:8090/api/sites                 # caddy-admin 解析结果
curl -s http://localhost:8090/api/certs                 # 证书列表
```
`localhost:2019` 是 Caddy Admin API，直接暴露到 host 方便调试；`localhost:8090` 是 caddy-admin 后端，同样直接暴露。在生产环境两者都不对外暴露——Caddy 只暴露 80/443，Admin API 仅 `localhost:2019` 监听。

---

## 部署到 ECS（yeanhua.asia 真实域名）

### 前提
- ECS 已完成初始化（Docker + Caddy + 安全组 22/80/443）
- 阿里云 DNS 已有 *.yeanhua.asia 泛域名，或手动添加 A 记录

### 部署

```bash
# 添加 DNS A 记录（阿里云控制台 → DNS）：
#   caddy-admin.yeanhua.asia → ECS IP
#   site-a.yeanhua.asia      → ECS IP
#   site-b.yeanhua.asia      → ECS IP

# 一键部署
ECS_IP=<your-ecs-ip> ./deploy/deploy.sh

# 验证
curl -sf https://caddy-admin.yeanhua.asia/api/sites
curl -sf https://caddy-admin.yeanhua.asia/api/certs
```

---

## API

| 端点 | 说明 |
|------|------|
| `GET /api/status` | Caddy 是否在线 |
| `GET /api/sites` | 所有站点列表 |
| `GET /api/sites/{domain}` | 单站点详情 |
| `GET /api/certs` | TLS 证书列表 |

---

## 目录结构

```
caddy-admin/
├── backend/          # Go API（查询 Caddy Admin API）
├── frontend/         # React + Vite + TypeScript
├── caddy/            # 本地模拟 Caddyfile
├── mock-sites/       # 本地模拟用的静态页面和 mock API
├── deploy/           # 生产部署模板（Caddyfile + docker-compose + deploy.sh）
├── docker-compose.yml
└── Makefile
```

---

## 可复用模板

`deploy/Caddyfile` 和 `deploy/docker-compose.yml` 可作为新项目的起点：

```bash
# 新项目接入 Caddy 只需三步：
# 1. 在 /etc/caddy/Caddyfile 追加两个站点块（前端 + API）
# 2. 在 /opt/{project}/ 放 docker-compose.yml，docker compose up -d
# 3. systemctl reload caddy → SSL 自动签发
```
