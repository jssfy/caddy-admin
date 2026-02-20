# 方案 B+ 详细设计：拆分仓库 + Caddy Admin API 动态注册

> 日期：2026-02-20
> 状态：设计稿

## 核心结论

- caddy-admin 从「只读仪表盘」升级为「服务注册中心」，新项目启动时调用 caddy-admin API 自注册路由
- 持久化用 JSON 文件，Caddy 重启后由 caddy-admin 重放路由
- 前端改为各项目自带 nginx 容器，Caddy 纯反代——彻底消除 volume 挂载的耦合
- 每个独立项目通过 Docker external network 加入 `caddy-net`

---

## 整体架构

```
                                    ┌──────────────────────┐
                                    │  services.json       │
                                    │  (持久化存储)          │
                                    └──────────┬───────────┘
                                               │ read/write
                                               │
┌──────────────┐  POST /api/services  ┌────────┴──────────┐  POST /config/...  ┌─────────┐
│  project-c   ├─────────────────────>│   caddy-admin     ├───────────────────>│  Caddy  │
│  (register   │     注册请求          │   (注册中心)       │   Admin API        │         │
│   sidecar)   │                      └───────────────────┘                    └─────────┘
└──────────────┘                               │
                                      Caddy 重启时:
                                      读 services.json → 重放所有路由
```

### 关键设计决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 前端静态文件服务 | 各项目自带 nginx 容器 | 消除 Caddy volume 挂载耦合，实现真正动态 |
| 持久化方案 | JSON 文件 (`services.json`) | 单节点场景足够，无外部依赖 |
| 注册方式 | 项目侧 sidecar 容器调用 caddy-admin API | 不直接调 Caddy Admin API，统一由 caddy-admin 管控 |
| 健康检查 | Caddy 内置 passive health check | 无需自建，upstream 不可达时 Caddy 自动返回 502 |
| TLS | 继续用通配符证书 `*.yeanhua.asia` | 新子域名无需额外证书操作 |

---

## 仓库结构

### caddy-infra（当前仓库演进）

```
caddy-infra/
├── caddy/
│   └── Caddyfile                  ← 精简：仅 caddy-admin 自身的路由 + 全局设置
├── caddy-admin/
│   ├── backend/
│   │   ├── caddy/
│   │   │   ├── client.go          ← 扩展：添加 AddRoute / RemoveRoute / SyncRoutes
│   │   │   ├── route_builder.go   ← 新增：ServiceConfig → Caddy JSON route 的转换器
│   │   │   ├── types.go
│   │   │   └── parser.go
│   │   ├── handlers/
│   │   │   ├── sites.go           ← 现有：只读查询
│   │   │   ├── services.go        ← 新增：注册/注销 API
│   │   │   ├── certs.go           ← 现有
│   │   │   └── helpers.go
│   │   ├── store/
│   │   │   └── file_store.go      ← 新增：services.json 读写
│   │   └── main.go                ← 扩展：启动时 sync
│   └── frontend/
│       └── src/
│           ├── pages/
│           │   ├── SiteList.tsx    ← 现有
│           │   ├── CertList.tsx    ← 现有
│           │   └── ServiceList.tsx ← 新增：注册服务管理页
│           └── ...
├── data/
│   └── services.json              ← 运行时持久化文件（volume 挂载）
├── docker-compose.yml             ← 仅 caddy + caddy-admin
├── Makefile
└── templates/
    └── project/                   ← 新项目脚手架模板
        ├── frontend/
        │   ├── Dockerfile         ← nginx 静态服务
        │   └── nginx.conf
        ├── backend/
        │   └── Dockerfile
        ├── register/
        │   ├── Dockerfile
        │   └── register.sh        ← 注册 sidecar 脚本
        ├── docker-compose.yml
        ├── Makefile
        └── .env.example
```

### 独立项目仓库（以 project-c 为例）

```
project-c/
├── frontend/
│   ├── src/
│   ├── dist/                      ← build 产物
│   ├── Dockerfile                 ← 多阶段构建：node build → nginx serve
│   └── nginx.conf                 ← 仅本地 :80，不处理 TLS
├── backend/
│   ├── main.go
│   └── Dockerfile
├── docker-compose.yml             ← 加入 external network
├── .env
└── Makefile
```

---

## 核心 API 设计

### 服务注册 API（caddy-admin 新增）

```
POST   /api/services              ← 注册新服务（幂等）
GET    /api/services              ← 列出所有已注册服务
GET    /api/services/{name}       ← 获取单个服务详情
PUT    /api/services/{name}       ← 更新服务配置
DELETE /api/services/{name}       ← 注销服务
POST   /api/services/sync         ← 手动触发：重放所有路由到 Caddy
```

### 注册请求体

```json
POST /api/services
{
  "name": "project-c",
  "domain": "project-c.yeanhua.asia",
  "upstreams": [
    {
      "match": "/api/*",
      "dial": "project-c-api:8080",
      "strip_prefix": ""
    },
    {
      "match": "/*",
      "dial": "project-c-frontend:80",
      "spa": true
    }
  ],
  "options": {
    "encode": "gzip"
  }
}
```

设计要点：
- `name` 是唯一标识，重复注册为**幂等更新**
- `upstreams` 按 match 精度排序（`/api/*` 在 `/*` 前）
- `spa: true` 表示启用 `try_files {path} /index.html` 等效行为（通过 rewrite 实现）
- `dial` 使用 Docker service name + port，依赖 `caddy-net` 内部 DNS

### 注册响应

```json
{
  "name": "project-c",
  "domain": "project-c.yeanhua.asia",
  "status": "active",
  "registered_at": "2026-02-20T15:30:00Z",
  "caddy_route_id": "svc-project-c"
}
```

---

## Caddy Client 扩展

### 现有能力（只读）

```go
// caddy/client.go — 现有
func (c *Client) GetConfig() (*CaddyConfig, error)
func (c *Client) IsRunning() bool
```

### 新增能力（写入）

```go
// caddy/client.go — 新增方法

// AddRoute 向 Caddy 添加一条命名路由
// 使用 @id 标识，便于后续更新/删除
// POST /config/apps/http/servers/srv0/routes
func (c *Client) AddRoute(routeID string, route json.RawMessage) error

// RemoveRoute 从 Caddy 删除指定 @id 的路由
// DELETE /id/{routeID}
func (c *Client) RemoveRoute(routeID string) error

// UpsertRoute = 先尝试删除旧路由（忽略 404），再添加新路由
func (c *Client) UpsertRoute(routeID string, route json.RawMessage) error

// SyncRoutes 批量同步：清除所有动态路由，重新添加
// 用于 Caddy 重启后的恢复
func (c *Client) SyncRoutes(routes map[string]json.RawMessage) error
```

### Caddy @id 机制

Caddy Admin API 支持在 JSON 对象中使用 `@id` 字段标记，后续可通过 `/id/{id}` 直接操作：

```json
{
  "@id": "svc-project-c",
  "match": [{"host": ["project-c.yeanhua.asia"]}],
  "handle": [...],
  "terminal": true
}
```

```bash
# 通过 @id 直接定位并删除
DELETE http://caddy:2019/id/svc-project-c

# 通过 @id 直接替换
PUT http://caddy:2019/id/svc-project-c
```

这避免了追踪数组索引的复杂性。

---

## Route Builder：ServiceConfig → Caddy JSON

```go
// caddy/route_builder.go

// ServiceConfig 是 caddy-admin 内部的服务描述格式
type ServiceConfig struct {
    Name      string           `json:"name"`
    Domain    string           `json:"domain"`
    Upstreams []UpstreamConfig `json:"upstreams"`
    Options   RouteOptions     `json:"options"`
}

type UpstreamConfig struct {
    Match       string `json:"match"`        // "/api/*" 或 "/*"
    Dial        string `json:"dial"`         // "project-c-api:8080"
    StripPrefix string `json:"strip_prefix"` // 可选：去除路径前缀
    SPA         bool   `json:"spa"`          // 是否启用 SPA fallback
}

type RouteOptions struct {
    Encode string `json:"encode"` // "gzip" | ""
}

// BuildCaddyRoute 将 ServiceConfig 转换为 Caddy JSON route
func BuildCaddyRoute(svc ServiceConfig) json.RawMessage {
    // 生成结构：
    // {
    //   "@id": "svc-{name}",
    //   "match": [{"host": ["{domain}"]}],
    //   "handle": [{
    //     "handler": "subroute",
    //     "routes": [
    //       { match /api/* → reverse_proxy },
    //       { match /* → reverse_proxy (+ rewrite for SPA) }
    //     ]
    //   }],
    //   "terminal": true
    // }
}
```

### SPA 处理方式的变化

当前架构中，Caddy 直接 serve 前端静态文件并用 `try_files` 处理 SPA。

B+ 架构中，前端由项目自带的 nginx 容器 serve，Caddy 只做反代。SPA fallback 由 **nginx 自己处理**：

```nginx
# project-c/frontend/nginx.conf
server {
    listen 80;
    root /usr/share/nginx/html;

    location / {
        try_files $uri $uri/ /index.html;
    }
}
```

Caddy 侧简化为纯反代，不需要关心 SPA 逻辑：

```json
{
  "handler": "reverse_proxy",
  "upstreams": [{"dial": "project-c-frontend:80"}]
}
```

---

## 持久化层

```go
// store/file_store.go

type FileStore struct {
    path string             // "data/services.json"
    mu   sync.RWMutex
    data map[string]ServiceConfig
}

func NewFileStore(path string) *FileStore
func (s *FileStore) Load() error                          // 启动时从文件加载
func (s *FileStore) Save() error                          // 每次变更后写文件
func (s *FileStore) Get(name string) (ServiceConfig, bool)
func (s *FileStore) List() []ServiceConfig
func (s *FileStore) Put(svc ServiceConfig) error          // 写内存 + 写文件
func (s *FileStore) Delete(name string) error             // 删内存 + 写文件
```

### services.json 格式

```json
{
  "version": 1,
  "services": {
    "project-c": {
      "name": "project-c",
      "domain": "project-c.yeanhua.asia",
      "upstreams": [
        { "match": "/api/*", "dial": "project-c-api:8080" },
        { "match": "/*", "dial": "project-c-frontend:80", "spa": true }
      ],
      "options": { "encode": "gzip" },
      "registered_at": "2026-02-20T15:30:00Z"
    },
    "project-d": {
      ...
    }
  }
}
```

### Volume 挂载

```yaml
# caddy-infra/docker-compose.yml
services:
  caddy-admin-api:
    volumes:
      - ./data:/app/data    # services.json 持久化到宿主机
```

---

## 生命周期流程

### 1. 基础设施启动

```
make up
  │
  ├─ Caddy 启动
  │   └─ 加载 Caddyfile（仅包含 caddy-admin 自身路由）
  │
  └─ caddy-admin 启动
      ├─ 加载 data/services.json
      ├─ 等待 Caddy Admin API 可达
      └─ SyncRoutes: 逐个调用 Caddy Admin API 注册所有已知服务
          ├─ POST /config/.../routes  (svc-project-c)
          ├─ POST /config/.../routes  (svc-project-d)
          └─ ...
```

### 2. 新项目注册

```
cd project-c && make up
  │
  ├─ project-c-frontend 启动 (nginx:80)
  ├─ project-c-api 启动 (:8080)
  │
  └─ project-c-register 启动 (sidecar)
      ├─ 等待 caddy-admin API 可达
      ├─ POST http://caddy-admin-api:8090/api/services
      │   Body: { name, domain, upstreams }
      │
      └─ caddy-admin 收到请求:
          ├─ 1. 验证请求
          ├─ 2. BuildCaddyRoute(config) → Caddy JSON
          ├─ 3. UpsertRoute("svc-project-c", route) → Caddy Admin API
          ├─ 4. store.Put(config) → 写 services.json
          └─ 5. 返回 201 Created
```

### 3. 项目更新（重新部署）

```
cd project-c && make deploy
  │
  ├─ 重新构建 frontend / api 容器
  └─ register sidecar 重新执行 POST /api/services
      └─ caddy-admin: UpsertRoute（幂等：先删旧路由，再加新路由）
```

### 4. 项目下线

```
cd project-c && make down
  │
  ├─ register sidecar 的 shutdown hook:
  │   DELETE http://caddy-admin-api:8090/api/services/project-c
  │   └─ caddy-admin:
  │       ├─ RemoveRoute("svc-project-c") → Caddy Admin API
  │       └─ store.Delete("project-c") → 更新 services.json
  │
  └─ 所有容器停止
```

如果 sidecar 未能优雅关闭（kill -9），路由仍留在 Caddy 中，但 upstream 不可达，Caddy 返回 502。管理员可通过 caddy-admin 面板手动清理。

### 5. Caddy 意外重启

```
Caddy 重启
  │
  └─ 只加载 Caddyfile（caddy-admin 路由）
      动态路由全部丢失
      │
      caddy-admin 检测到 Caddy 重启（定期 health check）
      └─ SyncRoutes: 从 services.json 重放所有路由
```

---

## 精简后的 Caddyfile

```caddyfile
{
    admin 0.0.0.0:2019
}

(tls_yeanhua) {
    tls /etc/caddy/certs/fullchain.pem /etc/caddy/certs/key.pem
}

# 唯一静态配置：caddy-admin 自身（bootstrap 路由）
# 所有其他项目的路由由 caddy-admin 通过 Admin API 动态注入
caddy-admin.yeanhua.asia {
    import tls_yeanhua
    encode gzip

    @api path /api/*
    handle @api {
        reverse_proxy caddy-admin-api:8090
    }

    handle {
        reverse_proxy caddy-admin-frontend:80
    }
}
```

注意：caddy-admin 的前端也改为 nginx 容器自行 serve，保持一致性。

---

## 精简后的 docker-compose.yml

```yaml
# caddy-infra/docker-compose.yml
services:

  caddy:
    image: caddy:2-alpine
    ports:
      - "80:80"
      - "443:443"
      - "2019:2019"
    volumes:
      - ./caddy/Caddyfile:/etc/caddy/Caddyfile:ro
      - ~/certs/yeanhua.asia:/etc/caddy/certs:ro
      - caddy_data:/data
      - caddy_config:/config
    networks: [caddy-net]
    restart: unless-stopped

  caddy-admin-api:
    build: ./caddy-admin/backend
    environment:
      CADDY_ADMIN_ADDR: caddy:2019
      CADDY_CERT_STORE: /data/caddy/caddy
      EXTERNAL_CERT_DIR: /external-certs
      LISTEN_ADDR: ":8090"
      STORE_PATH: /app/data/services.json
    volumes:
      - caddy_data:/data/caddy:ro
      - ~/certs/yeanhua.asia:/external-certs:ro
      - ./data:/app/data                         # services.json 持久化
    networks: [caddy-net]
    depends_on: [caddy]
    restart: unless-stopped

  caddy-admin-frontend:
    build: ./caddy-admin/frontend
    networks: [caddy-net]
    restart: unless-stopped

networks:
  caddy-net:
    name: caddy-net      # 命名网络，允许外部项目加入

volumes:
  caddy_data:
  caddy_config:
```

---

## 独立项目的 docker-compose.yml 模板

```yaml
# project-c/docker-compose.yml
services:

  project-c-api:
    build: ./backend
    environment:
      PORT: "8080"
    networks: [caddy-net]
    restart: unless-stopped

  project-c-frontend:
    build: ./frontend
    networks: [caddy-net]
    restart: unless-stopped

  # 注册 sidecar：启动时注册，停止时注销
  project-c-register:
    image: curlimages/curl:latest
    depends_on:
      - project-c-api
      - project-c-frontend
    environment:
      CADDY_ADMIN_URL: http://caddy-admin-api:8090
      SERVICE_NAME: project-c
      SERVICE_DOMAIN: project-c.yeanhua.asia
      API_UPSTREAM: project-c-api:8080
      FRONTEND_UPSTREAM: project-c-frontend:80
    entrypoint: ["/bin/sh", "/register/register.sh"]
    volumes:
      - ./register:/register:ro
    networks: [caddy-net]
    restart: "no"

networks:
  caddy-net:
    external: true
```

---

## 注册 Sidecar 脚本

```bash
#!/bin/sh
# register/register.sh

set -e

CADDY_ADMIN_URL="${CADDY_ADMIN_URL:-http://caddy-admin-api:8090}"

echo "Waiting for caddy-admin to be ready..."
until curl -sf "$CADDY_ADMIN_URL/api/status" > /dev/null 2>&1; do
  sleep 2
done

echo "Registering ${SERVICE_NAME} → ${SERVICE_DOMAIN}"
HTTP_CODE=$(curl -sf -o /dev/null -w "%{http_code}" \
  -X POST "$CADDY_ADMIN_URL/api/services" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"${SERVICE_NAME}\",
    \"domain\": \"${SERVICE_DOMAIN}\",
    \"upstreams\": [
      { \"match\": \"/api/*\", \"dial\": \"${API_UPSTREAM}\" },
      { \"match\": \"/*\", \"dial\": \"${FRONTEND_UPSTREAM}\" }
    ],
    \"options\": { \"encode\": \"gzip\" }
  }")

if [ "$HTTP_CODE" -ge 200 ] && [ "$HTTP_CODE" -lt 300 ]; then
  echo "Registered successfully (HTTP $HTTP_CODE)"
else
  echo "Registration failed (HTTP $HTTP_CODE)" >&2
  exit 1
fi

# 注册完毕后，sidecar 退出（restart: "no"）
# 注销由 make down 或手动 DELETE 处理
```

---

## 安全考量

### 注册 API 的访问控制

当前设计中，任何能访问 `caddy-net` 的容器都可以注册路由。生产环境需要增加：

1. **共享密钥（简单方案）**
   ```yaml
   environment:
     REGISTER_TOKEN: "secret-token-here"
   ```
   ```bash
   curl -H "Authorization: Bearer $REGISTER_TOKEN" ...
   ```

2. **Caddy Admin API 本身的保护**
   已有的 `admin 0.0.0.0:2019` 仅暴露在 `caddy-net` 内部。
   对外暴露的 `:2019` 端口在生产环境应移除。

### 域名校验

caddy-admin 应验证注册的 domain 符合 `*.yeanhua.asia` 模式，防止恶意注册。

---

## 实施路径（分阶段）

### Phase 1：caddy-admin 支持写入
- [ ] `caddy/client.go` 添加 AddRoute / RemoveRoute / UpsertRoute
- [ ] `caddy/route_builder.go` 实现 ServiceConfig → Caddy JSON
- [ ] `store/file_store.go` 实现 JSON 文件持久化
- [ ] `handlers/services.go` 实现注册 CRUD API
- [ ] `main.go` 启动时执行 SyncRoutes

### Phase 2：前端管理界面
- [ ] `ServiceList.tsx` 注册服务列表/状态面板
- [ ] 手动注册/注销功能
- [ ] 服务健康状态展示

### Phase 3：项目模板 + 自动注册
- [ ] 创建 `templates/project/` 脚手架
- [ ] 实现 register sidecar 脚本
- [ ] 精简 Caddyfile 和 docker-compose.yml
- [ ] 迁移 site-a / site-b 为独立项目验证

### Phase 4：健壮性
- [ ] caddy-admin 定期检测 Caddy 重启，自动 sync
- [ ] 注册 API 添加 token 认证
- [ ] 域名白名单校验
- [ ] 前端显示服务 upstream 健康状态
